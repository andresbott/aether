package identify

import (
	"container/list"
	"os"
	"sync"

	"github.com/andresbott/aether/libs/acoustid"
)

// DefaultCacheSize is the entry cap wired up by the application. One entry holds
// a file's candidate recordings — a few hundred bytes to a few KB — so a few
// thousand of them is a small amount of memory next to what identifying that
// many files would otherwise cost in fpcalc runs and rate-limited AcoustID
// calls.
const DefaultCacheSize = 5000

// cacheKey identifies a file BY CONTENT-ISH IDENTITY, not just by name: two
// different files can share a path over time (replaced, re-ripped, re-encoded),
// and a fingerprint answer belongs to the bytes, not to the name. Size and mtime
// are the cheap stand-in — a stat, no read.
//
// A tag save rewrites the file and moves mtime without changing the audio, so it
// costs a re-fingerprint. That is deliberate: nothing in a stat can tell a tag
// rewrite from a re-encode, and serving a stale fingerprint for genuinely
// different audio is the one failure mode worth spending a cache miss to avoid.
type cacheKey struct {
	path    string
	size    int64
	modUnix int64
}

// cacheEntry is one file's identification: the candidate recordings and the
// duration fpcalc measured.
type cacheEntry struct {
	recordings []acoustid.Recording
	duration   float64
}

// Cache remembers per-file fingerprint identifications so both identify flows —
// per-track (/metadata/identify) and album (/metadata/identify-album, via
// internal/albumidentify) — share one fingerprint pass. Both call
// Identifier.IdentifyFileWithDuration, so caching there is what makes
// "identify these songs, then identify them as an album" skip every fpcalc run
// and every AcoustID call, leaving only the album flow's MusicBrainz tracklist
// lookups to pay for.
//
// LRU with a hard entry cap: a long editing session over a big library must not
// grow it without bound, and the files a user keeps coming back to are the ones
// worth keeping.
//
// Safe for concurrent use.
type Cache struct {
	mu    sync.Mutex
	cap   int
	items map[cacheKey]*list.Element
	// order is most-recently-used at the front; the back is the eviction victim.
	order *list.List
}

// lruNode is what the list holds: the key travels with the value so eviction can
// find the map entry to delete.
type lruNode struct {
	key   cacheKey
	entry cacheEntry
}

// NewCache returns a cache holding at most size entries. A size below 1 yields
// nil — an explicitly disabled cache, which every call site tolerates.
func NewCache(size int) *Cache {
	if size < 1 {
		return nil
	}
	return &Cache{
		cap:   size,
		items: make(map[cacheKey]*list.Element, size),
		order: list.New(),
	}
}

// keyFor builds the cache key for a path. ok is false when the file cannot be
// stat'd (already gone, or a path the caller invented), in which case there is
// no identity to key on and the caller must bypass the cache rather than guess.
func keyFor(absPath string) (cacheKey, bool) {
	info, err := os.Stat(absPath)
	if err != nil || info.IsDir() {
		return cacheKey{}, false
	}
	return cacheKey{
		path:    absPath,
		size:    info.Size(),
		modUnix: info.ModTime().UnixNano(),
	}, true
}

// get returns a cached identification. A nil cache always misses, so callers do
// not need a nil check of their own.
func (c *Cache) get(key cacheKey) (cacheEntry, bool) {
	if c == nil {
		return cacheEntry{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return cacheEntry{}, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*lruNode).entry, true
}

// put stores an identification, evicting the least recently used entry when the
// cap is reached. A nil cache stores nothing.
func (c *Cache) put(key cacheKey, entry cacheEntry) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		el.Value.(*lruNode).entry = entry
		c.order.MoveToFront(el)
		return
	}
	c.items[key] = c.order.PushFront(&lruNode{key: key, entry: entry})
	for c.order.Len() > c.cap {
		oldest := c.order.Back()
		if oldest == nil {
			return
		}
		c.order.Remove(oldest)
		delete(c.items, oldest.Value.(*lruNode).key)
	}
}

// Len reports how many entries the cache holds. A nil cache holds none.
func (c *Cache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.order.Len()
}
