package scanfolder

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// bounded is what keeps a dead mount from hanging a request or the startup: the
// probe runs aside and the caller stops waiting after d.
func TestBoundedGivesUpOnASlowProbe(t *testing.T) {
	release := make(chan struct{})
	defer close(release)
	start := time.Now()
	err := bounded(20*time.Millisecond, "the probe", func() error {
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
	if err := bounded(time.Second, "the probe", func() error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the probe's own error", err)
	}
	if err := bounded(time.Second, "the probe", func() error { return nil }); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
}
