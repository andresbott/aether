package dlcache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// discard calls GetOrLoad and drops the result; the tests below assert via the
// load counters and side effects, not the returned bytes.
func discard(c *Cache, key string, load func() ([]byte, string, error)) {
	_, _, _ = c.GetOrLoad(key, load)
}

func TestGetOrLoad_MissLoadsAndCaches(t *testing.T) {
	c := New(time.Minute, 1<<20)
	calls := 0
	load := func() ([]byte, string, error) {
		calls++
		return []byte("image"), "png", nil
	}

	data, ext, err := c.GetOrLoad("http://x/a.png", load)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "image" || ext != "png" {
		t.Fatalf("got %q/%q, want image/png", data, ext)
	}

	// A second call for the same key must serve from cache, not re-load.
	data2, ext2, err := c.GetOrLoad("http://x/a.png", load)
	if err != nil {
		t.Fatalf("unexpected error on cached call: %v", err)
	}
	if string(data2) != "image" || ext2 != "png" {
		t.Fatalf("cached got %q/%q, want image/png", data2, ext2)
	}
	if calls != 1 {
		t.Fatalf("load called %d times, want 1 (second call should hit cache)", calls)
	}
}

func TestGetOrLoad_ErrorNotCached(t *testing.T) {
	c := New(time.Minute, 1<<20)
	calls := 0
	boom := errors.New("boom")
	load := func() ([]byte, string, error) {
		calls++
		if calls == 1 {
			return nil, "", boom
		}
		return []byte("ok"), "jpg", nil
	}

	if _, _, err := c.GetOrLoad("k", load); !errors.Is(err, boom) {
		t.Fatalf("first call err = %v, want boom", err)
	}
	data, _, err := c.GetOrLoad("k", load)
	if err != nil {
		t.Fatalf("second call err: %v", err)
	}
	if string(data) != "ok" {
		t.Fatalf("got %q, want ok", data)
	}
	if calls != 2 {
		t.Fatalf("load called %d times, want 2 (an errored load must not be cached)", calls)
	}
}

func TestGetOrLoad_ExpiredEntryReloads(t *testing.T) {
	c := New(10*time.Minute, 1<<20)
	base := time.Unix(0, 0)
	cur := base
	c.now = func() time.Time { return cur }
	calls := 0
	load := func() ([]byte, string, error) {
		calls++
		return []byte("v"), "png", nil
	}

	discard(c, "k", load)            // stored at base
	cur = base.Add(11 * time.Minute) // now past the 10m ttl
	discard(c, "k", load)            // must reload

	if calls != 2 {
		t.Fatalf("load called %d times, want 2 (entry should expire after ttl)", calls)
	}
}

func TestGetOrLoad_EvictsLeastRecentlyUsedOverByteCap(t *testing.T) {
	// A 9-byte cap holds two of these 4-byte payloads but not three.
	c := New(time.Hour, 9)
	loader := func(payload string, n *int) func() ([]byte, string, error) {
		return func() ([]byte, string, error) {
			*n++
			return []byte(payload), "png", nil
		}
	}
	var na, nb, nc int

	discard(c, "a", loader("aaaa", &na)) // 4 bytes cached
	discard(c, "b", loader("bbbb", &nb)) // 8 bytes cached
	discard(c, "c", loader("cccc", &nc)) // 12 > 9: evict LRU ("a")

	// "a" was evicted, so re-reading it must load again.
	discard(c, "a", loader("aaaa", &na))
	if na != 2 {
		t.Fatalf("a loaded %d times, want 2 (a should have been evicted)", na)
	}
	// "c" is the most recent, still cached.
	discard(c, "c", loader("cccc", &nc))
	if nc != 1 {
		t.Fatalf("c loaded %d times, want 1 (c should still be cached)", nc)
	}
}

func TestGetOrLoad_ConcurrentCallsLoadOnce(t *testing.T) {
	c := New(time.Hour, 1<<20)
	var calls int64
	release := make(chan struct{})
	// load blocks until released, so the flight stays open while every
	// goroutine arrives — without dedup each would run its own load.
	load := func() ([]byte, string, error) {
		atomic.AddInt64(&calls, 1)
		<-release
		return []byte("v"), "png", nil
	}

	const n = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			discard(c, "k", load)
		}()
	}
	close(start)
	time.Sleep(30 * time.Millisecond) // let all goroutines reach GetOrLoad
	close(release)
	wg.Wait()

	if got := atomic.LoadInt64(&calls); got != 1 {
		t.Fatalf("load called %d times, want 1 (concurrent identical calls must dedup)", got)
	}
}

func TestGetOrLoad_NilCacheLoadsDirectly(t *testing.T) {
	var c *Cache // nil: caching disabled
	calls := 0
	data, ext, err := c.GetOrLoad("k", func() ([]byte, string, error) {
		calls++
		return []byte("x"), "png", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "x" || ext != "png" {
		t.Fatalf("got %q/%q, want x/png", data, ext)
	}
	if calls != 1 {
		t.Fatalf("load called %d times, want 1", calls)
	}
}

func TestGetOrLoad_EmptyResultNotCached(t *testing.T) {
	c := New(time.Minute, 1<<20)
	calls := 0
	load := func() ([]byte, string, error) {
		calls++
		if calls == 1 {
			return nil, "", nil // empty, but no error
		}
		return []byte("later"), "png", nil
	}

	if data, _, _ := c.GetOrLoad("k", load); len(data) != 0 {
		t.Fatalf("first call data = %q, want empty", data)
	}
	data, _, _ := c.GetOrLoad("k", load)
	if string(data) != "later" {
		t.Fatalf("got %q, want later", data)
	}
	if calls != 2 {
		t.Fatalf("load called %d times, want 2 (an empty result must not be cached)", calls)
	}
}
