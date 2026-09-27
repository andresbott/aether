package scanfolder

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// bounded is what keeps a dead mount from hanging a request or the startup: the
// probe runs aside and the caller stops waiting after d.
func TestBoundedGivesUpOnASlowProbe(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	start := time.Now()
	err := bounded(20*time.Millisecond, "slow-probe", "the probe", func() error {
		<-release // a stat that never returns
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "did not answer") {
		t.Fatalf("err = %v, want a timeout error", err)
	}
	if time.Since(start) > time.Second {
		t.Fatalf("bounded blocked for %s", time.Since(start))
	}
}

func TestBoundedPassesTheProbeResultThrough(t *testing.T) {
	boom := errors.New("boom")
	if err := bounded(time.Second, "result-through", "the probe", func() error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the probe's own error", err)
	}
	if err := bounded(time.Second, "result-through", "the probe", func() error { return nil }); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}

// The point of sharing: a root that hangs forever costs one blocked probe, not
// one per question. Fifty callers ask about the same hung key; one probe starts.
func TestBoundedSharesOneInFlightProbePerKey(t *testing.T) {
	release := make(chan struct{})
	var started atomic.Int32
	probe := func() error {
		started.Add(1)
		<-release // a stat that never returns while the callers are asking
		return nil
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := bounded(20*time.Millisecond, "hung-root", "the probe", probe); err == nil {
				t.Error("a hung probe must time out")
			}
		}()
	}
	wg.Wait()
	close(release)

	if got := started.Load(); got != 1 {
		t.Fatalf("started %d probes for one hung key, want 1", got)
	}
}

// Sharing must not turn into caching: once a probe has returned, the next call
// probes again.
func TestBoundedProbesAgainOnceTheFlightHasLanded(t *testing.T) {
	var calls atomic.Int32
	probe := func() error { calls.Add(1); return nil }
	for i := 0; i < 3; i++ {
		if err := bounded(time.Second, "landed", "the probe", probe); err != nil {
			t.Fatal(err)
		}
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("probe ran %d times, want 3", got)
	}
}
