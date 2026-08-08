// internal/scanner/scanner.go
package scanner

import (
	"context"
	"fmt"
	"log/slog"
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

	for i := range libs {
		if ctx.Err() != nil {
			return stats, ctx.Err()
		}
		if err := s.scanLibrary(ctx, &libs[i], scanStart, opts, &stats); err != nil {
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

func (s *Scanner) scanLibrary(ctx context.Context, lib *model.Library, scanStart time.Time, opts ScanOptions, stats *ScanStats) error {
	now := time.Now()
	lib.LastScanStartedAt = &now
	if err := s.store.UpdateLibrary(lib); err != nil {
		return fmt.Errorf("update library scan timestamp: %w", err)
	}

	excludes, err := compileExcludes(lib.ExcludePatterns)
	if err != nil {
		return fmt.Errorf("library %q: %w", lib.Name, err)
	}

	walkResults, err := Walk([]model.Library{*lib}, excludes, lib.FollowSymlinks)
	if err != nil {
		return err
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
				mu.Lock()
				tagResults = append(tagResults, tagResult{walk: wr, meta: meta})
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

