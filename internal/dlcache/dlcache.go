// Package dlcache is a small in-memory cache of downloaded image bytes keyed by
// URL. It exists so the metadata editor's pre-save probe and the save itself do
// not re-download the same provider image: the probe fills the cache, and both a
// repeated probe and the subsequent save read it back.
package dlcache

import (
	"container/list"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Cache memoizes (bytes, ext) results by key, evicting the least-recently-used
// entry once the cached bytes exceed the configured bound. The zero value is not
// usable; construct with New. A nil *Cache is valid and simply loads through (no
// caching), so callers can leave it unset to disable caching.
type Cache struct {
	mu       sync.Mutex
	ll       *list.List               // front = most recently used
	items    map[string]*list.Element // key -> *list.Element holding *entry
	curBytes int64

	ttl      time.Duration
	maxBytes int64

	// group collapses concurrent loads of the same key into one call, so two
	// rapid clicks on the same candidate download it once rather than each
	// waiting a turn behind the provider's rate limiter.
	group singleflight.Group

	// now supplies the clock; overridable in tests to drive TTL expiry.
	now func() time.Time
}

type entry struct {
	key  string
	data []byte
	ext  string
	at   time.Time
}

// New returns a cache that expires entries older than ttl and holds at most
// maxBytes of image data (a non-positive bound disables that limit).
func New(ttl time.Duration, maxBytes int64) *Cache {
	return &Cache{
		ll:       list.New(),
		items:    map[string]*list.Element{},
		ttl:      ttl,
		maxBytes: maxBytes,
		now:      time.Now,
	}
}

// GetOrLoad returns the cached bytes for key, or calls load and caches a
// successful, non-empty result before returning it. A load error is returned
// as-is and never cached.
func (c *Cache) GetOrLoad(key string, load func() ([]byte, string, error)) ([]byte, string, error) {
	if c == nil {
		return load()
	}
	if data, ext, ok := c.lookup(key); ok {
		return data, ext, nil
	}
	v, err, _ := c.group.Do(key, func() (any, error) {
		// Re-check under the flight: a concurrent caller may have just filled it.
		if data, ext, ok := c.lookup(key); ok {
			return result{data, ext}, nil
		}
		data, ext, err := load()
		if err != nil {
			return nil, err
		}
		if len(data) > 0 {
			c.store(key, data, ext)
		}
		return result{data, ext}, nil
	})
	if err != nil {
		return nil, "", err
	}
	r := v.(result)
	return r.data, r.ext, nil
}

// result carries a load's outcome through singleflight, which speaks any.
type result struct {
	data []byte
	ext  string
}

// lookup returns the live entry for key, dropping it first when it has expired.
func (c *Cache) lookup(key string) ([]byte, string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, "", false
	}
	e := el.Value.(*entry)
	if c.ttl > 0 && c.now().Sub(e.at) > c.ttl {
		c.remove(el)
		return nil, "", false
	}
	c.ll.MoveToFront(el)
	return e.data, e.ext, true
}

// store inserts or refreshes key, then evicts LRU entries until the byte bound
// holds (never evicting the entry just stored).
func (c *Cache) store(key string, data []byte, ext string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[key]; ok {
		e := el.Value.(*entry)
		c.curBytes += int64(len(data)) - int64(len(e.data))
		e.data, e.ext, e.at = data, ext, c.now()
		c.ll.MoveToFront(el)
	} else {
		c.items[key] = c.ll.PushFront(&entry{key: key, data: data, ext: ext, at: c.now()})
		c.curBytes += int64(len(data))
	}
	for c.maxBytes > 0 && c.curBytes > c.maxBytes && c.ll.Len() > 1 {
		c.remove(c.ll.Back())
	}
}

// remove drops el from the list, the index and the byte total. The caller holds
// c.mu.
func (c *Cache) remove(el *list.Element) {
	e := el.Value.(*entry)
	c.ll.Remove(el)
	delete(c.items, e.key)
	c.curBytes -= int64(len(e.data))
}
