package scanner

import (
	"context"
	"errors"
	"testing"
)

// A track whose reconcile transaction fails once — the work succeeded but the
// commit did not — and then succeeds on retry must be counted exactly once, not
// once per attempt. Guards the double-count that happens when New/Updated are
// bumped inside the retried transaction instead of after it commits.
func TestReconcileOneCountsRetriedSuccessOnce(t *testing.T) {
	var stats reconcileStats
	attempts := 0
	reconcileOne(context.Background(), &stats, "Artist/Album/01.mp3", func() (bool, error) {
		attempts++
		if attempts == 1 {
			return true, errors.New("injected commit-phase failure")
		}
		return true, nil
	})

	if attempts != 2 {
		t.Fatalf("expected one retry (2 attempts), got %d", attempts)
	}
	if stats.New != 1 {
		t.Fatalf("expected New counted once after the retried commit, got %d", stats.New)
	}
	if stats.Updated != 0 {
		t.Fatalf("expected Updated=0, got %d", stats.Updated)
	}
	if stats.Processed != 1 {
		t.Fatalf("expected Processed=1, got %d", stats.Processed)
	}
	if stats.Failed != 0 {
		t.Fatalf("expected Failed=0, got %d", stats.Failed)
	}
}

// A run that keeps failing after its retry is counted once in Failed and not in
// New/Updated/Processed, so one unsaveable track is a reported shortfall, not a
// silent drop or a phantom success.
func TestReconcileOnePersistentFailureCountsFailed(t *testing.T) {
	var stats reconcileStats
	attempts := 0
	reconcileOne(context.Background(), &stats, "Artist/Album/02.mp3", func() (bool, error) {
		attempts++
		return false, errors.New("permanent failure")
	})

	if attempts != 2 {
		t.Fatalf("expected the failing run to be retried once (2 attempts), got %d", attempts)
	}
	if stats.Failed != 1 {
		t.Fatalf("expected Failed=1, got %d", stats.Failed)
	}
	if stats.New != 0 || stats.Updated != 0 || stats.Processed != 0 {
		t.Fatalf("a failed track must not be counted as new/updated/processed: %+v", stats)
	}
}

// A run failing under a cancelled context is not retried and not counted: a
// cancelled scan aborts and is reported as an error elsewhere, so it must not
// inflate the failure count.
func TestReconcileOneCancelledContextNotCounted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var stats reconcileStats
	attempts := 0
	reconcileOne(ctx, &stats, "Artist/Album/03.mp3", func() (bool, error) {
		attempts++
		return false, errors.New("failed while cancelled")
	})

	if attempts != 1 {
		t.Fatalf("expected no retry once the context is cancelled (1 attempt), got %d", attempts)
	}
	if stats.Failed != 0 || stats.New != 0 || stats.Updated != 0 || stats.Processed != 0 {
		t.Fatalf("a cancelled run must not be counted at all: %+v", stats)
	}
}
