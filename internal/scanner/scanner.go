// internal/scanner/scanner.go
package scanner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/andresbott/aether/internal/model"
	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
	"github.com/go-bumbu/tempo"
)

type ScanOptions struct {
	IsFull bool
}

type ScanStats struct {
	TracksProcessed int
	TracksNew       int
	TracksUpdated   int
	// TracksSkipped counts paths a caller supplied that the library does not
	// cover and that were therefore deliberately not indexed (outside the root,
	// not an audio extension, excluded, unreadable). Only RescanPaths can
	// produce these — it is handed an explicit path list — so a full Scan, whose
	// paths all come from its own walk, always leaves this zero. It exists so a
	// caller can tell "skipped by design" from "reconcile failed" instead of
	// inferring a shortfall from TracksProcessed.
	TracksSkipped int
	Errors        []error
}

type Scanner struct {
	cfg       Config
	store     *store.Store
	tagReader tags.Reader
}

func New(cfg Config, s *store.Store, tagReader tags.Reader) *Scanner {
	return &Scanner{cfg: cfg, store: s, tagReader: tagReader}
}

type tagResult struct {
	walk WalkResult
	meta tags.Metadata
	// audioHash is the file's metadata-invariant audio hash, or "" when it has
	// none. Read alongside the tags, because both are per-file work that wants
	// the worker pool rather than the reconcile transaction.
	audioHash string
}

// libraryWalk is one library plus the walk that cleared its guards. It exists so
// the guards can run for *every* library before *any* library is reconciled
// (see preflight) without walking the tree twice.
type libraryWalk struct {
	lib  *model.Library
	walk []WalkResult
}

func (s *Scanner) Scan(ctx context.Context, opts ScanOptions) (ScanStats, error) {
	scanStart := time.Now()
	stats := ScanStats{}

	libs, err := s.store.ListLibraries()
	if err != nil {
		return stats, fmt.Errorf("list libraries: %w", err)
	}
	if len(libs) == 0 {
		tempo.Info(ctx, "no libraries configured; nothing to scan")
		return stats, nil
	}

	// Phase 1: validate and walk everything. Nothing is written yet, so a guard
	// tripping here aborts the whole run atomically.
	walks, err := s.preflight(ctx, libs)
	if err != nil {
		return stats, err
	}

	// Phase 2: reconcile.
	for i := range walks {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}
		if err := s.scanLibrary(ctx, walks[i], scanStart, opts, &stats); err != nil {
			return stats, err
		}
	}

	if ctx.Err() == nil {
		if err := s.store.Cleanup(scanStart); err != nil {
			return stats, err
		}
	}

	return stats, nil
}

// preflight runs both sweep guards and the walk for every library before Scan
// reconciles the first one, and returns the walk results so phase 2 does not
// repeat the I/O (walking twice would also risk seeing two different trees).
//
// The two-phase split is what makes an aborted run harmless. Scan returns on the
// first library that fails a guard, but planTrackContinuity's candidate pool is
// deliberately not library-scoped — a move between two collections has to keep
// its row — so an unavailable library that is merely *later* in
// ListLibraries' name order used to have all of its rows stat ENOENT and land in
// `vanished` while an earlier library was still being reconciled. A single
// byte-identical new file there was enough to re-link an unreachable library's
// row, moving its stars, playlist memberships, history and library_id onto a file
// it has nothing to do with, and the guard then failed the scan too late to undo
// any of it. Validating first makes the abort happen before the first write.
//
// Still not covered, and not coverable here: an unreadable or unmounted *subtree*
// inside a root that is present (a per-directory mount that is gone leaves an
// empty mountpoint directory behind). Its files stat ENOENT exactly like deleted
// ones, so those rows can be swept — and, since the fingerprint cannot tell the
// difference either, re-linked onto a byte-identical new file. Requiring a
// vanished row's parent directory to still exist would break the primary use
// case, because reorganising a library moves whole directories. The narrowing to
// fs.ErrNotExist in planTrackContinuity is the only defence, and it only helps
// when the failure is a permission error rather than an empty mountpoint.
func (s *Scanner) preflight(ctx context.Context, libs []model.Library) ([]libraryWalk, error) {
	out := make([]libraryWalk, 0, len(libs))
	for i := range libs {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		lib := &libs[i]

		if err := checkLibraryRoot(lib.Path); err != nil {
			return nil, fmt.Errorf("library %q: %w", lib.Name, err)
		}

		excludes, err := compileExcludes(lib.ExcludePatterns)
		if err != nil {
			return nil, fmt.Errorf("library %q: %w", lib.Name, err)
		}

		walkResults, err := Walk([]model.Library{*lib}, excludes, lib.FollowSymlinks)
		if err != nil {
			return nil, err
		}

		// An absent tree is not an empty tree. Walk swallows every error, including
		// the root's, so a share that is present but unpopulated (a bare mountpoint)
		// looks exactly like a library the user emptied — and Cleanup would delete
		// every track of it, with the playlists, stars, play history and queue
		// entries attached to them. The DB still holding tracks is the only evidence
		// available, so it decides.
		if err := s.checkEmptyScanWithIndexedTracks(lib, walkResults); err != nil {
			return nil, err
		}

		out = append(out, libraryWalk{lib: lib, walk: walkResults})
	}
	return out, nil
}

// scanLibrary is phase 2: everything from the LastScanStartedAt stamp onwards,
// for a library preflight has already validated and walked.
func (s *Scanner) scanLibrary(ctx context.Context, lw libraryWalk, scanStart time.Time, opts ScanOptions, stats *ScanStats) error {
	lib, walkResults := lw.lib, lw.walk

	// Stamped in phase 2 on purpose: a library whose run aborted in preflight must
	// not claim it was scanned.
	now := time.Now()
	lib.LastScanStartedAt = &now
	if err := s.store.UpdateLibrary(lib); err != nil {
		return fmt.Errorf("update library scan timestamp: %w", err)
	}

	allPaths := make([]string, len(walkResults))
	for i, wr := range walkResults {
		allPaths[i] = wr.FilePath
	}
	if err := s.store.BulkUpdateLastSeen(allPaths, scanStart); err != nil {
		return err
	}

	var toProcess []WalkResult
	if opts.IsFull {
		toProcess = walkResults
	} else {
		toProcess = s.filterChanged(walkResults)
	}

	workers := s.cfg.TagReadWorkers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	tagResults := make([]tagResult, 0, len(toProcess))
	var mu sync.Mutex
	var wg sync.WaitGroup
	ch := make(chan WalkResult, workers*2)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for wr := range ch {
				if ctx.Err() != nil {
					return
				}
				if !s.tagReader.CanRead(wr.FilePath) {
					continue
				}
				meta, err := s.tagReader.Read(ctx, wr.FilePath)
				if err != nil {
					mu.Lock()
					stats.Errors = append(stats.Errors, err)
					mu.Unlock()
					continue
				}
				// Hashed in the worker, next to the tag read: it is bounded
				// per-file I/O (audiohash reads at most 256 KiB of payload) and
				// belongs on the pool rather than in reconcile's per-track
				// transaction. Only files this pass reads get one, so an
				// incremental scan hashes exactly what changed and a steady
				// state hashes nothing.
				hash := audioHashOf(wr.FilePath)
				mu.Lock()
				tagResults = append(tagResults, tagResult{walk: wr, meta: meta, audioHash: hash})
				mu.Unlock()
			}
		}()
	}

	for _, wr := range toProcess {
		if ctx.Err() != nil {
			break
		}
		ch <- wr
	}
	close(ch)
	wg.Wait()

	if ctx.Err() != nil {
		return ctx.Err()
	}

	rec, err := s.reconcile(ctx, lib.Path, tagResults, scanStart)
	if err != nil {
		return err
	}
	stats.TracksProcessed += rec.Processed
	stats.TracksNew += rec.New
	stats.TracksUpdated += rec.Updated

	return nil
}

func (s *Scanner) filterChanged(results []WalkResult) []WalkResult {
	paths := make([]string, len(results))
	for i, wr := range results {
		paths[i] = wr.FilePath
	}
	modMap, err := s.store.FilterChanged(paths)
	if err != nil {
		slog.Warn("filterChanged store error; falling back to full rescan", "err", err)
		return results
	}
	var out []WalkResult
	for _, wr := range results {
		dbMod, found := modMap[wr.FilePath]
		if !found || dbMod.Before(wr.ModTime) {
			out = append(out, wr)
		}
	}
	return out
}

func compileExcludes(jsonPatterns string) ([]*regexp.Regexp, error) {
	if jsonPatterns == "" {
		return nil, nil
	}
	patterns, err := decodeExcludePatterns(jsonPatterns)
	if err != nil {
		return nil, err
	}
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			slog.Warn("invalid exclude pattern, skipping", "pattern", p, "err", err)
			continue
		}
		out = append(out, re)
	}
	return out, nil
}

// checkLibraryRoot refuses to scan a root the walk could not read. filepath.WalkDir
// reports the root's own stat error to the walk function, which swallows it
// (walk.go), so without this an unmounted library scans "successfully" with zero
// results and Cleanup deletes everything in it.
func checkLibraryRoot(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("root %q is unavailable: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("root %q is not a directory", path)
	}
	return nil
}

// checkEmptyScanWithIndexedTracks refuses to continue when a library walk found
// zero files while the database still holds tracks for it, as this indicates an
// unmounted share or permission issue rather than a genuinely emptied library.
func (s *Scanner) checkEmptyScanWithIndexedTracks(lib *model.Library, walkResults []WalkResult) error {
	if len(walkResults) > 0 {
		return nil
	}
	indexed, err := s.store.CountTracksForLibrary(lib.ID)
	if err != nil {
		return fmt.Errorf("library %q: count indexed tracks: %w", lib.Name, err)
	}
	if indexed > 0 {
		// The remedy has a price and has to say so: Track.LibraryID carries
		// constraint:OnDelete:CASCADE, so deleting the library is exactly the
		// hard-delete this guard just refused to perform.
		return fmt.Errorf("library %q: no audio files under %q but %d tracks are indexed; "+
			"refusing to delete them — check that the path is mounted; if it really is gone, "+
			"delete the library in Settings → Libraries, which also removes those %d tracks and "+
			"everything attached to them (playlist entries, stars, play history)",
			lib.Name, lib.Path, indexed, indexed)
	}
	return nil
}
