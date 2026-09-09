package taskrunner

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestExclusionGroupSerializesAcrossTaskNames(t *testing.T) {
	r, err := NewRunner(Cfg{Parallelism: 2, QueueSize: 8, HistorySize: 8})
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	var concurrent, maxConcurrent int32
	run := func(ctx context.Context, _ *slog.Logger) error {
		n := atomic.AddInt32(&concurrent, 1)
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if n <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, n) {
				break
			}
		}
		time.Sleep(60 * time.Millisecond)
		atomic.AddInt32(&concurrent, -1)
		return nil
	}
	r.RegisterTask(run, "a", 1, ExclusionGroup("g"))
	r.RegisterTask(run, "b", 1, ExclusionGroup("g"))
	r.Start()
	defer func() { _ = r.Shutdown(context.Background()) }()

	if _, _, err := r.AddRun("a"); err != nil {
		t.Fatalf("AddRun a: %v", err)
	}
	if _, _, err := r.AddRun("b"); err != nil {
		t.Fatalf("AddRun b: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&maxConcurrent) > 0 && atomic.LoadInt32(&concurrent) == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := atomic.LoadInt32(&maxConcurrent); got != 1 {
		t.Fatalf("exclusion group did not serialize: maxConcurrent = %d, want 1", got)
	}
}
