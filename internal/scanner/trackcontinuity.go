// internal/scanner/trackcontinuity.go
package scanner

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sort"

	"github.com/andresbott/aether/internal/store"
)

// trackFingerprint is the exact part of the move proof — the part that can put
// both ends of a move in the same bucket. Duration is compared with a tolerance
// instead (durationsAgree) and file_mod_time is only a tiebreak, so neither
// belongs in the key.
type trackFingerprint struct {
	FileSize int64
	Title    string
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
// The proof, per pair: equal file sizes, equal titles, durations within a
// second, the old path gone from disk, and a fingerprint that identifies exactly
// one vanished row and one new file. The stat is what separates a move from a
// copy — a file still on disk is a live row that must not be re-pointed.
// Deliberately conservative: a false match merges two tracks' listening history,
// which is worse than losing one's.
//
// Known conservative misses: a move that also rewrote the tags (the bytes, and
// so the size, change), two files swapping paths (neither old path is gone), and
// a move straddling two scan runs (Cleanup already deleted the row).
func (s *Scanner) planTrackContinuity(results []tagResult) error {
	if len(results) == 0 {
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

	// Only a path the DB has never seen can be the far end of a move.
	incoming := map[trackFingerprint][]tagResult{}
	sizeSet := map[int64]bool{}
	unclaimed := 0
	for _, tr := range results {
		if known[tr.walk.FilePath] {
			continue
		}
		unclaimed++
		key := fingerprintOf(tr)
		incoming[key] = append(incoming[key], tr)
		sizeSet[key.FileSize] = true
	}
	if unclaimed == 0 {
		return nil // nothing new in this batch, so nothing moved into it
	}

	rows, err := s.store.TracksByFileSizes(sortedSizes(sizeSet))
	if err != nil {
		return err
	}

	// A row is doomed only when its file is really gone. A path this batch walked
	// is on disk by definition; for the rest the stat is the authority, and it is
	// what keeps a copy from stealing the original's identity.
	inBatch := make(map[string]bool, len(paths))
	for _, p := range paths {
		inBatch[p] = true
	}
	vanished := map[trackFingerprint][]store.TrackRow{}
	vanishedRows := 0
	for _, row := range rows {
		if inBatch[row.FilePath] {
			continue
		}
		if _, err := os.Stat(row.FilePath); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			continue // on EACCES, EIO, ELOOP etc, leave the row alone
		}
		key := trackFingerprint{FileSize: row.FileSize, Title: row.Title}
		vanished[key] = append(vanished[key], row)
		vanishedRows++
	}
	if vanishedRows == 0 {
		return nil // nothing of this shape left the library: every new path is a new file
	}

	// Sorted iteration: which pair wins must not depend on which tag reader
	// finished first — the same discipline planAlbumContinuity follows.
	relinked := 0
	for _, key := range sortedFingerprints(incoming) {
		row, tr, ok := matchOne(incoming[key], vanished[key])
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
			return fmt.Errorf("relink track %d: %w", row.ID, err)
		}
		if !done {
			continue // the row moved underneath us
		}
		relinked++
		slog.Info("track relinked after a move",
			"track_id", row.ID, "from", row.FilePath, "to", tr.walk.FilePath)
	}
	// Makes the conservative misses visible instead of silent: new paths and
	// vanished rows that did not add up to a proof are the difference here.
	slog.Info("track continuity", "new_paths", unclaimed, "vanished_rows", vanishedRows, "relinked", relinked)
	return nil
}

func fingerprintOf(tr tagResult) trackFingerprint {
	return trackFingerprint{FileSize: tr.walk.FileSize, Title: tr.meta.Title}
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

// matchOne returns the single pair a fingerprint group proves. One vanished row
// and one new file is the whole common case. When either side has several
// entries the group is ambiguous — real libraries hold byte-identical duplicates
// of the same track — and only an exact mod time can still single one out,
// because a move on one filesystem preserves it. Anything else is skipped:
// merging two tracks' history is worse than losing one's. Both sides having
// several entries is never resolved; matching them up pairwise would be
// guesswork.
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

// sortedFingerprints orders the groups by (FileSize, Title).
func sortedFingerprints(m map[trackFingerprint][]tagResult) []trackFingerprint {
	out := make([]trackFingerprint, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FileSize != out[j].FileSize {
			return out[i].FileSize < out[j].FileSize
		}
		return out[i].Title < out[j].Title
	})
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
