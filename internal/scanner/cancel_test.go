// internal/scanner/cancel_test.go
package scanner_test

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/scanner"
	"github.com/andresbott/aether/internal/tags"
)

// blockingReader blocks in Read until the context is done, counting how many
// reads were entered. It stands in for a tag read that outlives the caller — an
// ffprobe on an unresponsive mount, or a slow network file.
type blockingReader struct {
	started atomic.Int32
	honored atomic.Int32
}

func (r *blockingReader) CanRead(absPath string) bool { return scanner.IsAudioFile(absPath) }

func (r *blockingReader) Read(ctx context.Context, _ string) (tags.Metadata, error) {
	r.started.Add(1)
	select {
	case <-ctx.Done():
		r.honored.Add(1)
		return tags.Metadata{}, ctx.Err()
	case <-time.After(30 * time.Second):
		// Reached only if the context never arrives — the bug this guards against.
		return tags.Metadata{}, nil
	}
}

// TestRescanPathsCancelsBlockedTagRead is the end-to-end payoff of threading a
// context through tags.Reader: a rescan whose caller goes away unblocks its
// in-progress tag read instead of waiting for it to finish. The scanner already
// checked ctx.Err() between files, so cancellation used to take effect only at
// file boundaries and could not interrupt one read.
func TestRescanPathsCancelsBlockedTagRead(t *testing.T) {
	st := testScanStore(t)
	dir := t.TempDir()
	createTestFiles(t, dir, []string{"Album/01.mp3"})
	lib := seedLibrary(t, st, dir, nil)

	reader := &blockingReader{}
	s := scanner.New(scanner.Config{}, st, reader)

	ctx, cancel := context.WithCancel(context.Background())
	type result struct {
		stats scanner.ScanStats
		err   error
	}
	done := make(chan result, 1)
	go func() {
		stats, err := s.RescanPaths(ctx, lib.ID, []string{filepath.Join(dir, "Album", "01.mp3")})
		done <- result{stats: stats, err: err}
	}()

	// Wait until the read is actually under way, so the cancel lands mid-read
	// rather than before the scan reaches it.
	deadline := time.After(5 * time.Second)
	for reader.started.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("tag read never started")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
	cancel()

	// Returning promptly is the point: without a context-aware reader the call
	// would sit in Read for its full 30s regardless of the cancel.
	select {
	case got := <-done:
		// A failed read is recorded per file and the rescan carries on (it must not
		// abort a multi-file edit over one bad file), so the cancellation surfaces
		// in stats.Errors rather than as a returned error.
		if !hasCanceledErr(got.stats.Errors) {
			t.Errorf("expected a context.Canceled entry in stats.Errors, got %v (err=%v)",
				got.stats.Errors, got.err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("rescan did not return after cancellation — the tag read is not context-aware")
	}
	if reader.honored.Load() == 0 {
		t.Error("the reader never observed the canceled context")
	}
}

func hasCanceledErr(errs []error) bool {
	for _, err := range errs {
		if errors.Is(err, context.Canceled) {
			return true
		}
	}
	return false
}
