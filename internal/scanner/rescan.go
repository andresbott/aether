// internal/scanner/rescan.go
package scanner

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// RescanPaths re-reads the tags of absPaths and reconciles them into the store,
// so files the metadata editor just wrote are reflected in the index
// without a full scan. It applies the scan preflight's availability guard first,
// so a folder the scan refuses is refused here too rather than being indexed one
// edit at a time. Paths that are not inside the scan folder, not audio
// files, excluded by the folder's patterns, or unreadable are silently skipped
// and counted in ScanStats.TracksSkipped; only tag-read failures appear in
// ScanStats.Errors. A run indexed everything it was supposed to when
// TracksProcessed == len(absPaths)-TracksSkipped and Errors is empty — callers
// must not compare TracksProcessed to len(absPaths) directly, because the
// editor's file listing ignores the folder's exclude patterns, so it can hand
// this method a path the scanner deliberately skips as excluded.
//
// It deliberately does NOT run the scan cleanup: store.Cleanup deletes every
// track whose last_seen_at predates the run, which on a targeted rescan is the
// entire catalog. Nor does it run the exhaustive DeleteOrphanedAggregates sweep,
// whose cost scales with the whole catalog. Instead it snapshots the aggregate
// ids the touched tracks belonged to before reconcile and prunes only those with
// store.PruneOrphanedAggregates — an edit can only empty an album/artist/genre it
// moved a track away from. Anything that snapshot misses (a moved-and-retagged
// row, say) is swept by the scheduled scan's Cleanup, which still runs the
// exhaustive sweep.
func (s *Scanner) RescanPaths(ctx context.Context, scanFolder string, absPaths []string) (ScanStats, error) {
	stats := ScanStats{}
	if len(absPaths) == 0 {
		return stats, nil
	}

	folder, ok := s.cfg.Folders.ByName(scanFolder)
	if !ok {
		return stats, fmt.Errorf("rescan: scan folder %q is not configured", scanFolder)
	}
	// The same guard the scan preflight applies: a root that cannot be scanned —
	// unmounted, not a directory, itself a symlink — is not re-indexed either.
	// Without it an edit under a symlinked root indexes the file under a spelling
	// the scanner refuses, one row at a time, and those rows are swept — with
	// their stars, playlist entries and play history — the day the root is
	// pointed at the real directory.
	if err := folder.Available(); err != nil {
		return stats, fmt.Errorf("rescan: scan folder %q: %w", scanFolder, err)
	}
	excludes, err := folder.Excludes()
	if err != nil {
		return stats, fmt.Errorf("rescan: %w", err)
	}

	results := make([]tagResult, 0, len(absPaths))
	for _, abs := range absPaths {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}
		wr, ok := WalkWouldEmit(folder, excludes, abs)
		if !ok {
			stats.TracksSkipped++
			continue
		}
		meta, rerr := s.tagReader.Read(ctx, abs)
		if rerr != nil {
			stats.Errors = append(stats.Errors, fmt.Errorf("read tags %q: %w", abs, rerr))
			continue
		}
		// The editor's own writes come through here, which is what keeps the
		// stored hash current: a tag edit bumps the file's mod time, this pass
		// re-reads it, and the hash comes back *unchanged* because a tag edit
		// does not touch the audio. That is precisely what lets a later move of
		// the same file still be proved.
		results = append(results, tagResult{walk: wr, meta: meta, audioHash: audioHashOf(abs)})
	}

	// Snapshot the aggregates the touched tracks belong to *before* reconcile
	// re-points them: a retag can only orphan an album/artist/genre it moves a
	// track away from, so these ids are the only ones the prune must check.
	admitted := make([]string, 0, len(results))
	for _, tr := range results {
		admitted = append(admitted, tr.walk.FilePath)
	}
	touched, err := s.store.TouchedAggregatesForPaths(admitted)
	if err != nil {
		return stats, fmt.Errorf("rescan: snapshot aggregates: %w", err)
	}

	rec, err := s.reconcile(ctx, folder.Path, results, time.Now(), nil, noopProgress{})
	stats.TracksProcessed += rec.Processed
	stats.TracksNew += rec.New
	stats.TracksUpdated += rec.Updated
	stats.TracksFailed += rec.Failed
	if err != nil {
		return stats, err
	}

	// Prune only the aggregates this edit could have emptied. The exhaustive
	// whole-DB sweep is left to the scheduled scan's Cleanup.
	if err := s.store.PruneOrphanedAggregates(ctx, touched); err != nil {
		return stats, fmt.Errorf("rescan: prune orphans: %w", err)
	}
	return stats, nil
}

// excludedByAnySegment reports whether rel — a path relative to the scan
// folder's root — is excluded, testing every ancestor directory as well as the
// file itself.
//
// Walk prunes a matching *directory* with SkipDir, so an anchored pattern like
// "^Live$" removes everything under "Artist/Live/" even though neither the full
// relative path nor the filename of "Artist/Live/01.mp3" matches it. Checking
// only the leaf would admit tracks the next scan then deletes, so admission
// walks the same segments Walk would have visited: for each ancestor, its own
// relative path and its bare name — exactly matchExcludes' per-entry test.
func excludedByAnySegment(rel string, excludes []*regexp.Regexp) bool {
	if len(excludes) == 0 {
		return false
	}
	segments := strings.Split(rel, string(filepath.Separator))
	prefix := ""
	for _, seg := range segments {
		if prefix == "" {
			prefix = seg
		} else {
			prefix = prefix + string(filepath.Separator) + seg
		}
		if matchesExclude(excludes, prefix, seg) {
			return true
		}
	}
	return false
}
