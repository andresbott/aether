package albumidentify

import (
	"container/list"
	"context"
	"sync"

	"github.com/andresbott/aether/internal/artistimage"
)

// DefaultReleaseCacheSize is the entry cap the application wires up. One entry
// is a release's tracklist — a few KB at most — so a few hundred of them cost
// little next to what they save: MusicBrainz is throttled to ONE request per
// second, and Resolve enriches up to MaxEnrichedOptions (8) options per run.
// That throttle, not the fingerprinting, is what makes a repeated album identify
// feel slow once internal/identify's cache covers the fpcalc/AcoustID pass.
const DefaultReleaseCacheSize = 500

// CachingReleaseLookup wraps a ReleaseLookup with an LRU cache keyed by release
// MBID. It deliberately sits IN FRONT of the throttled MusicBrainz client rather
// than inside it: a cache hit must not consume a rate-limiter token, or the
// second-per-request wait would still be paid and the cache would save nothing
// the user can feel.
//
// A release's tracklist is stable reference data — far safer to hold than a
// fingerprint answer — so entries are kept for the process lifetime, bounded
// only by the LRU cap.
//
// Safe for concurrent use.
type CachingReleaseLookup struct {
	inner ReleaseLookup

	mu    sync.Mutex
	cap   int
	items map[string]*list.Element
	// order is most-recently-used at the front; the back is evicted first.
	order *list.List
}

// releaseNode is what the LRU list holds; the key travels with the value so
// eviction can find the map entry to delete.
type releaseNode struct {
	mbid   string
	detail artistimage.ReleaseDetail
}

// NewCachingReleaseLookup wraps inner with a cache of at most size entries. A
// size below 1 disables caching: every lookup passes straight through, which is
// what a caller that explicitly wants no cache gets.
func NewCachingReleaseLookup(inner ReleaseLookup, size int) *CachingReleaseLookup {
	return &CachingReleaseLookup{
		inner: inner,
		cap:   size,
		items: make(map[string]*list.Element),
		order: list.New(),
	}
}

// Release returns the release detail for mbid, from cache when possible.
//
// Only a usable answer is stored. A failure is not cached — MusicBrainz
// rate-limits and times out routinely, and a cached failure would leave an
// option permanently un-enriched. A detail with no ReleaseMBID is not cached
// either: enrich treats that as "not enriched", so it is a miss, not an answer.
func (c *CachingReleaseLookup) Release(
	ctx context.Context, mbid string,
) (artistimage.ReleaseDetail, error) {
	if hit, ok := c.get(mbid); ok {
		return hit, nil
	}
	detail, err := c.inner.Release(ctx, mbid)
	if err != nil {
		return detail, err
	}
	if detail.ReleaseMBID != "" {
		c.put(mbid, detail)
	}
	return detail, nil
}

func (c *CachingReleaseLookup) get(mbid string) (artistimage.ReleaseDetail, bool) {
	if c.cap < 1 {
		return artistimage.ReleaseDetail{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[mbid]
	if !ok {
		return artistimage.ReleaseDetail{}, false
	}
	c.order.MoveToFront(el)
	return el.Value.(*releaseNode).detail, true
}

func (c *CachingReleaseLookup) put(mbid string, detail artistimage.ReleaseDetail) {
	if c.cap < 1 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[mbid]; ok {
		el.Value.(*releaseNode).detail = detail
		c.order.MoveToFront(el)
		return
	}
	c.items[mbid] = c.order.PushFront(&releaseNode{mbid: mbid, detail: detail})
	for c.order.Len() > c.cap {
		oldest := c.order.Back()
		if oldest == nil {
			return
		}
		c.order.Remove(oldest)
		delete(c.items, oldest.Value.(*releaseNode).mbid)
	}
}
