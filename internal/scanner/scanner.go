// internal/scanner/scanner.go
package scanner

import (
	"context"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/andresbott/aether/internal/store"
	"github.com/andresbott/aether/internal/tags"
)

type ScanOptions struct {
	IsFull bool
}

type ScanStats struct {
	TracksProcessed int
	TracksNew       int
	TracksUpdated   int
	Errors          []error
}

type Scanner struct {
	cfg       Config
	store     *store.Store
	tagReader tags.Reader
	excludes  []*regexp.Regexp
}

func New(cfg Config, s *store.Store, tagReader tags.Reader) *Scanner {
	var excludes []*regexp.Regexp
	for _, p := range cfg.ExcludePatterns {
		if re, err := regexp.Compile(p); err == nil {
			excludes = append(excludes, re)
		}
	}
	return &Scanner{cfg: cfg, store: s, tagReader: tagReader, excludes: excludes}
}

type tagResult struct {
	walk WalkResult
	meta tags.Metadata
}

func (s *Scanner) Scan(ctx context.Context, opts ScanOptions) (ScanStats, error) {
	scanStart := time.Now()
	stats := ScanStats{}

	walkResults, err := Walk(s.cfg.MusicPaths, s.excludes, s.cfg.FollowSymlinks)
	if err != nil {
		return stats, err
	}

	// Bulk-update LastSeenAt for all encountered files
	allPaths := make([]string, len(walkResults))
	for i, wr := range walkResults {
		allPaths[i] = wr.FilePath
	}
	if err := s.bulkUpdateLastSeen(allPaths, scanStart); err != nil {
		return stats, err
	}

	// Filter for incremental: skip unchanged files
	var toProcess []WalkResult
	if opts.IsFull {
		toProcess = walkResults
	} else {
		toProcess = s.filterChanged(walkResults)
	}

	// Read tags in parallel
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
				meta, err := s.tagReader.Read(wr.FilePath)
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
		return stats, ctx.Err()
	}

	// Reconcile
	reconcileStats, err := s.reconcile(ctx, tagResults, scanStart)
	if err != nil {
		return stats, err
	}
	stats.TracksProcessed = reconcileStats.Processed
	stats.TracksNew = reconcileStats.New
	stats.TracksUpdated = reconcileStats.Updated

	// Cleanup
	if ctx.Err() == nil {
		if err := s.cleanup(scanStart); err != nil {
			return stats, err
		}
	}

	return stats, nil
}

func (s *Scanner) filterChanged(results []WalkResult) []WalkResult {
	paths := make([]string, len(results))
	for i, wr := range results {
		paths[i] = wr.FilePath
	}

	modMap, err := s.store.FilterChanged(paths)
	if err != nil {
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

func (s *Scanner) bulkUpdateLastSeen(allPaths []string, scanTime time.Time) error {
	return s.store.BulkUpdateLastSeen(allPaths, scanTime)
}
