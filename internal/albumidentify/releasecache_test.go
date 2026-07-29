package albumidentify_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andresbott/aether/internal/albumidentify"
	"github.com/andresbott/aether/internal/artistimage"
)

// countingReleases is a ReleaseLookup that counts how often it was actually
// asked, which is how these tests observe cache hits. A real MusicBrainz lookup
// is throttled to one request per second, so every avoided call is a second the
// user does not wait.
type countingReleases struct {
	calls  atomic.Int32
	detail artistimage.ReleaseDetail
	err    error
	// block, when non-nil, holds every call until it is closed — used to prove a
	// cache hit does not queue behind an in-flight request.
	block chan struct{}
}

func (c *countingReleases) Release(_ context.Context, mbid string) (artistimage.ReleaseDetail, error) {
	c.calls.Add(1)
	if c.block != nil {
		<-c.block
	}
	if c.err != nil {
		return artistimage.ReleaseDetail{}, c.err
	}
	d := c.detail
	d.ReleaseMBID = mbid
	return d, nil
}

func detailFor(mbid string) artistimage.ReleaseDetail {
	return artistimage.ReleaseDetail{
		ReleaseMBID: mbid,
		Title:       "Album " + mbid,
		TrackCount:  2,
		DiscCount:   1,
		Tracks: []artistimage.ReleaseTrack{
			{DiscNumber: 1, TrackNumber: 1, Title: "One", RecordingMBID: "rec-1"},
			{DiscNumber: 1, TrackNumber: 2, Title: "Two", RecordingMBID: "rec-2"},
		},
	}
}

func TestReleaseCacheServesASecondLookupOfTheSameRelease(t *testing.T) {
	inner := &countingReleases{detail: detailFor("rel-A")}
	cached := albumidentify.NewCachingReleaseLookup(inner, 10)

	first, err := cached.Release(context.Background(), "rel-A")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := cached.Release(context.Background(), "rel-A")
	if err != nil {
		t.Fatalf("second: %v", err)
	}

	if got := inner.calls.Load(); got != 1 {
		t.Fatalf("expected 1 upstream lookup, got %d", got)
	}
	if second.Title != first.Title || len(second.Tracks) != len(first.Tracks) {
		t.Fatalf("cached detail differs: %+v vs %+v", second, first)
	}
}

func TestReleaseCacheKeysByMBID(t *testing.T) {
	inner := &countingReleases{detail: detailFor("ignored")}
	cached := albumidentify.NewCachingReleaseLookup(inner, 10)

	a, err := cached.Release(context.Background(), "rel-A")
	if err != nil {
		t.Fatal(err)
	}
	b, err := cached.Release(context.Background(), "rel-B")
	if err != nil {
		t.Fatal(err)
	}

	if got := inner.calls.Load(); got != 2 {
		t.Fatalf("expected 2 upstream lookups for 2 releases, got %d", got)
	}
	if a.ReleaseMBID == b.ReleaseMBID {
		t.Fatalf("expected distinct releases, both were %q", a.ReleaseMBID)
	}
}

// A failed lookup must stay retryable: MusicBrainz rate-limits and times out
// routinely, and caching that would leave an option permanently un-enriched.
func TestReleaseCacheDoesNotStoreFailures(t *testing.T) {
	inner := &countingReleases{err: errors.New("upstream boom")}
	cached := albumidentify.NewCachingReleaseLookup(inner, 10)

	for i := 0; i < 2; i++ {
		if _, err := cached.Release(context.Background(), "rel-A"); err == nil {
			t.Fatalf("lookup %d: expected an error", i)
		}
	}

	if got := inner.calls.Load(); got != 2 {
		t.Fatalf("expected a failed lookup to be retried, got %d calls", got)
	}
}

// The resolver treats an empty ReleaseMBID as "not enriched" (see enrich), so an
// empty detail is not a usable answer and must not be cached either.
func TestReleaseCacheDoesNotStoreAnEmptyDetail(t *testing.T) {
	inner := &emptyLookup{}
	cached := albumidentify.NewCachingReleaseLookup(inner, 10)

	for i := 0; i < 2; i++ {
		if _, err := cached.Release(context.Background(), "rel-A"); err != nil {
			t.Fatalf("lookup %d: %v", i, err)
		}
	}
	if got := inner.calls.Load(); got != 2 {
		t.Fatalf("expected an empty detail to be looked up again, got %d calls", got)
	}
}

// emptyLookup returns a detail with no MBID, which is what a MusicBrainz miss
// looks like to the resolver.
type emptyLookup struct{ calls atomic.Int32 }

func (e *emptyLookup) Release(context.Context, string) (artistimage.ReleaseDetail, error) {
	e.calls.Add(1)
	return artistimage.ReleaseDetail{}, nil
}

func TestReleaseCacheEvictsTheLeastRecentlyUsed(t *testing.T) {
	inner := &countingReleases{detail: detailFor("ignored")}
	cached := albumidentify.NewCachingReleaseLookup(inner, 2)

	for _, mbid := range []string{"rel-A", "rel-B"} {
		if _, err := cached.Release(context.Background(), mbid); err != nil {
			t.Fatal(err)
		}
	}
	// Touch rel-A so rel-B becomes the least recently used.
	if _, err := cached.Release(context.Background(), "rel-A"); err != nil {
		t.Fatal(err)
	}
	if _, err := cached.Release(context.Background(), "rel-C"); err != nil {
		t.Fatal(err)
	}
	before := inner.calls.Load()

	if _, err := cached.Release(context.Background(), "rel-A"); err != nil {
		t.Fatal(err)
	}
	if got := inner.calls.Load(); got != before {
		t.Fatalf("expected the touched entry to survive, got %d extra calls", got-before)
	}
	if _, err := cached.Release(context.Background(), "rel-B"); err != nil {
		t.Fatal(err)
	}
	if got := inner.calls.Load(); got != before+1 {
		t.Fatalf("expected the evicted entry to be fetched again, got %d calls", got)
	}
}

// The cache sits IN FRONT of the throttled client, so a hit must return without
// waiting on anything upstream — that is what turns a repeat album identify from
// eight rate-limited seconds into an instant one.
func TestReleaseCacheHitDoesNotWaitOnTheUpstreamClient(t *testing.T) {
	inner := &countingReleases{detail: detailFor("rel-A"), block: make(chan struct{})}
	cached := albumidentify.NewCachingReleaseLookup(inner, 10)

	// Prime the entry with a call that is allowed to finish.
	close(inner.block)
	if _, err := cached.Release(context.Background(), "rel-A"); err != nil {
		t.Fatal(err)
	}

	// From here on the upstream would block forever; a cache hit must not reach it.
	inner.block = make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, err := cached.Release(context.Background(), "rel-A")
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("cached lookup: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a cache hit reached the blocking upstream client")
	}
}

func TestReleaseCacheIsSafeForConcurrentUse(t *testing.T) {
	inner := &countingReleases{detail: detailFor("ignored")}
	cached := albumidentify.NewCachingReleaseLookup(inner, 100)

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// A mix of repeats and fresh MBIDs, so both the hit and the miss path
			// run concurrently.
			mbid := "rel-" + string(rune('A'+i%4))
			if _, err := cached.Release(context.Background(), mbid); err != nil {
				t.Errorf("concurrent lookup: %v", err)
			}
		}(i)
	}
	wg.Wait()
}

// A zero or negative size disables caching rather than panicking, matching
// identify.NewCache.
func TestReleaseCacheSizeZeroPassesEveryLookupThrough(t *testing.T) {
	inner := &countingReleases{detail: detailFor("rel-A")}
	cached := albumidentify.NewCachingReleaseLookup(inner, 0)

	for i := 0; i < 2; i++ {
		if _, err := cached.Release(context.Background(), "rel-A"); err != nil {
			t.Fatal(err)
		}
	}
	if got := inner.calls.Load(); got != 2 {
		t.Fatalf("expected no caching at size 0, got %d calls", got)
	}
}
