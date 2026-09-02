// internal/scanner/trackcontinuity.go
package scanner

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"
	"strconv"

	"github.com/andresbott/aether/internal/store"
)

// relinkPass is one way of proving a move: the key both ends must share, plus
// how to fetch the rows that could carry it. The passes run in order, and a file
// one pass claims is withdrawn before the next runs.
//
// There are two because they fail on opposite inputs. size+title costs nothing —
// the walk and the tag read already produced both — but a tag edit changes both,
// so it is blind to a move that also retagged the file. The audio hash is blind
// to tags by construction, but it exists only for the formats libs/audiohash
// covers and only for rows some earlier scan already hashed. Neither subsumes
// the other, so both run.
type relinkPass struct {
	// name goes in the re-link log line, so which proof carried it is visible.
	name string
	// keyOf buckets a candidate file. "" means the file cannot take part in
	// this pass — an unhashable format, say — and is skipped rather than
	// bucketed under the empty key.
	keyOf func(tagResult) string
	// rowKeyOf buckets a database row the same way, with the same "" rule.
	rowKeyOf func(store.TrackRow) string
	// rowsFor fetches the rows that could match these files. Each pass queries
	// a different column, which is why this is a callback and not a key list.
	rowsFor func([]tagResult) ([]store.TrackRow, error)
}

// planTrackContinuity re-points track rows at the path their file moved to, so a
// move or a rename does not delete the row — and with it the track's playlist
// memberships, play history, stars and play-queue position, every one of which
// store.DeleteOrphanedAggregates cascades away once the id is gone.
//
// It runs before the per-track loop and before planAlbumContinuity, and rewrites
// nothing but file_path, filename and library_id: once the row carries the new
// path, reconcileTrack's `WHERE file_path = ?` finds it and the whole per-track
// path is unchanged. Anything this cannot prove is left alone and becomes an
// insert plus a delete, exactly as before.
//
// Every proof shares three requirements: the old path is gone from disk (this is
// what separates a move from a copy — a file still there is a live row that must
// not be re-pointed), the durations agree within a second, and the shared key
// identifies exactly one vanished row and one new file. What differs is the key:
//
//   - size+title, the cheap and exact one, catches a plain move.
//   - the audio hash catches a move whose file was also retagged. A tag edit
//     rewrites the file, so its size changes and its title may too, and the
//     first proof loses both of its anchors — yet libs/audiohash reads only the
//     audio payload, so its value is identical before and after. This is the
//     common real-world case, because the tools that rename files from tags
//     (Picard, beets) retag and re-file in one operation.
//
// Deliberately conservative throughout: a false match merges two tracks'
// listening history, which is worse than losing one's.
//
// Known conservative misses: a retagged move of a format libs/audiohash does not
// cover — it handles FLAC, MP3, MP4, WAV, AIFF, Ogg Vorbis and Opus, leaving
// WMA, APE, WavPack, TTA, DSF, Matroska/WebM and raw AAC of walk.go's sixteen
// extensions, and it declines an Ogg file carrying a mapping other than Vorbis or
// Opus, an Ogg file that is chained, truncated or carries a trailer (its stream
// then has no page ending at end of file, so the digest has no length component),
// and a file with no locatable audio at all, such as a WAV declaring a "data"
// size of 0 — or of a track whose row has not been hashed yet; a full scan
// hashes every file and arms it. Also: two files swapping paths (neither old
// path is gone), and a move straddling two scan runs (Cleanup already deleted
// the row).
func (s *Scanner) planTrackContinuity(results []tagResult) error {
	if len(results) == 0 {
		logTrackContinuity(0, 0, 0)
		return nil
	}
	paths := make([]string, 0, len(results))
	for _, tr := range results {
		paths = append(paths, tr.walk.FilePath)
	}
	known, err := s.store.KnownTrackPaths(paths)
	if err != nil {
		return err
	}

	// A path this batch walked is on disk by definition — half of what tells a
	// move from a copy. The stat in fileIsGone is the other half.
	inBatch := make(map[string]bool, len(paths))
	for _, p := range paths {
		inBatch[p] = true
	}

	// Only a path the DB has never seen can be the far end of a move.
	candidates := make([]tagResult, 0, len(results))
	for _, tr := range results {
		if !known[tr.walk.FilePath] {
			candidates = append(candidates, tr)
		}
	}
	if len(candidates) == 0 {
		logTrackContinuity(0, 0, 0) // nothing new in this batch, so nothing moved into it
		return nil
	}
	newPaths := len(candidates)

	relinked := 0
	vanishedIDs := map[uint]bool{} // by id, so a row both passes consider is counted once
	for _, pass := range s.relinkPasses() {
		got, vanished, consumed, err := s.runRelinkPass(pass, candidates, inBatch)
		relinked += got
		for id := range vanished {
			vanishedIDs[id] = true
		}
		if err != nil {
			logTrackContinuity(newPaths, len(vanishedIDs), relinked)
			return err
		}
		if len(consumed) == 0 {
			continue
		}
		// A file this pass claimed now has a row pointing at it; a later pass
		// must not hand it a second one.
		remaining := make([]tagResult, 0, len(candidates)-len(consumed))
		for _, tr := range candidates {
			if !consumed[tr.walk.FilePath] {
				remaining = append(remaining, tr)
			}
		}
		candidates = remaining
		if len(candidates) == 0 {
			break
		}
	}
	logTrackContinuity(newPaths, len(vanishedIDs), relinked)
	return nil
}

// relinkPasses returns the proofs in the order they are attempted: the exact,
// free one first, so the audio hash is only consulted for what it could not
// settle.
func (s *Scanner) relinkPasses() []relinkPass {
	return []relinkPass{
		{
			name:     "size+title",
			keyOf:    func(tr tagResult) string { return sizeTitleKey(tr.walk.FileSize, tr.meta.Title) },
			rowKeyOf: func(r store.TrackRow) string { return sizeTitleKey(r.FileSize, r.Title) },
			rowsFor: func(files []tagResult) ([]store.TrackRow, error) {
				sizes := map[int64]bool{}
				for _, tr := range files {
					sizes[tr.walk.FileSize] = true
				}
				return s.store.TracksByFileSizes(sortedSizes(sizes))
			},
		},
		{
			name:     "audiohash",
			keyOf:    func(tr tagResult) string { return tr.audioHash },
			rowKeyOf: func(r store.TrackRow) string { return r.AudioHash },
			rowsFor: func(files []tagResult) ([]store.TrackRow, error) {
				hashes := map[string]bool{}
				for _, tr := range files {
					if tr.audioHash != "" {
						hashes[tr.audioHash] = true
					}
				}
				if len(hashes) == 0 {
					return nil, nil // nothing in this batch is hashable
				}
				return s.store.TracksByAudioHashes(sortedStrings(hashes))
			},
		},
	}
}

// runRelinkPass buckets the candidate files by the pass's key, asks for the rows
// that could match them, keeps only the rows whose file is really gone, and
// re-links every bucket that resolves to exactly one pair. It reports the rows
// it considered doomed and the paths it claimed, so the caller can keep an
// honest count and withdraw claimed files from later passes.
func (s *Scanner) runRelinkPass(p relinkPass, files []tagResult, inBatch map[string]bool) (int, map[uint]bool, map[string]bool, error) {
	consumed := map[string]bool{}
	vanishedIDs := map[uint]bool{}

	byKey := map[string][]tagResult{}
	for _, tr := range files {
		if key := p.keyOf(tr); key != "" {
			byKey[key] = append(byKey[key], tr)
		}
	}
	if len(byKey) == 0 {
		return 0, vanishedIDs, consumed, nil
	}

	rows, err := p.rowsFor(files)
	if err != nil {
		return 0, vanishedIDs, consumed, err
	}

	vanished := map[string][]store.TrackRow{}
	for _, row := range rows {
		key := p.rowKeyOf(row)
		if key == "" || len(byKey[key]) == 0 {
			continue // no candidate shares this key, so there is nothing to pair
		}
		if !fileIsGone(row.FilePath, inBatch) {
			continue
		}
		vanished[key] = append(vanished[key], row)
		vanishedIDs[row.ID] = true
	}
	if len(vanishedIDs) == 0 {
		return 0, vanishedIDs, consumed, nil
	}

	// Sorted iteration: which pair wins must not depend on which tag reader
	// finished first — the same discipline planAlbumContinuity follows.
	relinked := 0
	for _, key := range sortedKeys(byKey) {
		row, tr, ok := matchOne(byKey[key], vanished[key])
		if !ok {
			continue
		}
		if !durationsAgree(row.Duration, int(tr.meta.Duration.Seconds())) {
			continue
		}
		done, err := s.store.RelinkTrack(row.ID, row.FilePath, tr.walk.FilePath, tr.walk.LibraryID)
		if err != nil {
			if store.IsUniqueViolation(err) {
				continue // a concurrent pass got there first
			}
			return relinked, vanishedIDs, consumed, fmt.Errorf("relink track %d: %w", row.ID, err)
		}
		if !done {
			continue // the row moved underneath us
		}
		relinked++
		consumed[tr.walk.FilePath] = true
		slog.Info("track relinked after a move",
			"track_id", row.ID, "from", row.FilePath, "to", tr.walk.FilePath, "proof", p.name)
	}
	return relinked, vanishedIDs, consumed, nil
}

// fileIsGone reports whether a row's file is really absent, which is what
// separates a move from a copy: a file still on disk belongs to a live row that
// must not be re-pointed at anything.
func fileIsGone(path string, inBatch map[string]bool) bool {
	if inBatch[path] {
		return false // this batch walked it, so it exists
	}
	_, err := os.Stat(path)
	if err == nil {
		return false
	}
	// Only a definite "not there" counts. On EACCES, EIO, ELOOP and friends the
	// file may well still exist, so leave the row alone.
	return errors.Is(err, fs.ErrNotExist)
}

// logTrackContinuity makes the conservative misses visible instead of silent:
// new paths and vanished rows that did not add up to a proof are the difference
// between the counters. It is emitted on every path that got far enough to know
// them, the all-zero "nothing moved" case included — a scan that logs nothing is
// indistinguishable from a pre-pass that never ran, which is exactly the shape of
// bug the counters exist to expose.
func logTrackContinuity(newPaths, vanishedRows, relinked int) {
	slog.Info("track continuity", "new_paths", newPaths, "vanished_rows", vanishedRows, "relinked", relinked)
}

// sizeTitleKey is the first pass's bucket key. The NUL separator keeps a size's
// last digit from running into a title that starts with one.
func sizeTitleKey(size int64, title string) string {
	return strconv.FormatInt(size, 10) + "\x00" + title
}

// durationsAgree allows a second of slack: production reads tags through
// tags.NewFallbackReader(taglib, ffprobe), so a row indexed by one reader and
// re-read by the other can round the same file differently.
func durationsAgree(a, b int) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= 1
}

// matchOne returns the single pair a key group proves. One vanished row and one
// new file is the whole common case. When either side has several entries the
// group is ambiguous — real libraries hold byte-identical duplicates of the same
// track, and those share an audio hash as well as a size — and only an exact mod
// time can still single one out, because a move on one filesystem preserves it.
// Anything else is skipped: merging two tracks' history is worse than losing
// one's. Both sides having several entries is never resolved; matching them up
// pairwise would be guesswork.
func matchOne(files []tagResult, rows []store.TrackRow) (store.TrackRow, tagResult, bool) {
	switch {
	case len(files) == 0 || len(rows) == 0:
		return store.TrackRow{}, tagResult{}, false
	case len(files) == 1 && len(rows) == 1:
		return rows[0], files[0], true
	case len(files) == 1:
		if row, ok := onlyRowWithModTime(rows, files[0].walk.ModTime.Unix()); ok {
			return row, files[0], true
		}
	case len(rows) == 1:
		if file, ok := onlyFileWithModTime(files, rows[0].FileModTime.Unix()); ok {
			return rows[0], file, true
		}
	}
	return store.TrackRow{}, tagResult{}, false
}

// sortedSizes returns the distinct sizes in ascending order, so the candidate
// query is independent of map iteration order.
func sortedSizes(set map[int64]bool) []int64 {
	out := make([]int64, 0, len(set))
	for size := range set {
		out = append(out, size)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// sortedStrings is sortedSizes for a set of string keys.
func sortedStrings(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// sortedKeys orders the candidate buckets, so which pair a pass tries first does
// not depend on map iteration order.
func sortedKeys(m map[string][]tagResult) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// onlyRowWithModTime returns the one row whose mod time is sec, and reports
// false when none or several are — whole seconds, because a nanosecond that has
// been through SQLite and back is not a value to bet a row's history on.
func onlyRowWithModTime(rows []store.TrackRow, sec int64) (store.TrackRow, bool) {
	var found store.TrackRow
	n := 0
	for _, row := range rows {
		if row.FileModTime.Unix() == sec {
			found, n = row, n+1
		}
	}
	return found, n == 1
}

// onlyFileWithModTime is onlyRowWithModTime for the other side of the match.
func onlyFileWithModTime(files []tagResult, sec int64) (tagResult, bool) {
	var found tagResult
	n := 0
	for _, f := range files {
		if f.walk.ModTime.Unix() == sec {
			found, n = f, n+1
		}
	}
	return found, n == 1
}
