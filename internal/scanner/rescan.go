// internal/scanner/rescan.go
package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// RescanPaths re-reads the tags of absPaths and reconciles them into the store,
// so files the metadata editor just wrote are reflected in the library index
// without a full scan. Paths that are not inside the library, not audio files,
// excluded by the library's patterns, or unreadable are silently skipped and
// counted in ScanStats.TracksSkipped; only tag-read failures appear in
// ScanStats.Errors. A run indexed everything it was supposed to when
// TracksProcessed == len(absPaths)-TracksSkipped and Errors is empty — callers
// must not compare TracksProcessed to len(absPaths) directly, because the
// editor's file listing is deliberately wider than the scanner's admission
// rules (it ignores the library's exclude patterns and accepts extensions the
// scanner does not index).
//
// It deliberately does NOT run the scan cleanup: store.Cleanup deletes every
// track whose last_seen_at predates the run, which on a targeted rescan is the
// entire library. Orphaned aggregates left behind by the edit (e.g. the artist
// a renamed track used to belong to) are pruned with DeleteOrphanedAggregates,
// which is keyed on "has no tracks" rather than on a timestamp and is therefore
// safe to run standalone.
func (s *Scanner) RescanPaths(ctx context.Context, libraryID uint, absPaths []string) (ScanStats, error) {
	stats := ScanStats{}
	if len(absPaths) == 0 {
		return stats, nil
	}

	lib, err := s.store.GetLibrary(libraryID)
	if err != nil {
		return stats, fmt.Errorf("rescan: library %d: %w", libraryID, err)
	}
	excludes, err := compileExcludes(lib.ExcludePatterns)
	if err != nil {
		return stats, fmt.Errorf("rescan: library %q: %w", lib.Name, err)
	}

	results := make([]tagResult, 0, len(absPaths))
	for _, abs := range absPaths {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}
		wr, ok := s.admitPath(lib.Path, lib.ID, abs, excludes)
		if !ok {
			stats.TracksSkipped++
			continue
		}
		meta, rerr := s.tagReader.Read(ctx, abs)
		if rerr != nil {
			stats.Errors = append(stats.Errors, fmt.Errorf("read tags %q: %w", abs, rerr))
			continue
		}
		results = append(results, tagResult{walk: wr, meta: meta})
	}

	rec, err := s.reconcile(ctx, lib.Path, results, time.Now())
	stats.TracksProcessed += rec.Processed
	stats.TracksNew += rec.New
	stats.TracksUpdated += rec.Updated
	if err != nil {
		return stats, err
	}

	// The edit may have emptied an album/artist/genre. This is the only prune
	// that is safe outside a full scan.
	if err := s.store.DeleteOrphanedAggregates(); err != nil {
		return stats, fmt.Errorf("rescan: prune orphans: %w", err)
	}
	return stats, nil
}

// admitPath decides whether abs may be reconciled into libRoot and, if so,
// builds its WalkResult. It mirrors the walk's admission rules (inside the
// root, audio extension, not excluded — including by an ancestor directory the
// walk would have pruned — and stat-able) so a rescan can never insert a track
// that the next real scan would immediately delete.
func (s *Scanner) admitPath(libRoot string, libID uint, abs string, excludes []*regexp.Regexp) (WalkResult, bool) {
	rel, err := filepath.Rel(filepath.Clean(libRoot), filepath.Clean(abs))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		slog.Warn("rescan: path outside library, skipping", "path", abs, "library", libRoot)
		return WalkResult{}, false
	}
	if !IsAudioFile(abs) {
		return WalkResult{}, false
	}
	if excludedByAnySegment(rel, excludes) {
		return WalkResult{}, false
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		return WalkResult{}, false
	}
	if !s.tagReader.CanRead(abs) {
		return WalkResult{}, false
	}
	return WalkResult{
		FilePath:  abs,
		LibraryID: libID,
		ModTime:   info.ModTime(),
		Dir:       filepath.Dir(abs),
	}, true
}

// excludedByAnySegment reports whether rel — a path relative to the library
// root — is excluded, testing every ancestor directory as well as the file
// itself.
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
