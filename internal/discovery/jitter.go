// Package discovery scores and ranks albums and playlists for the Discovery
// feed. It is deliberately free of database access: every function here is pure,
// so the formula can be tested without SQLite. The store gathers raw signals and
// calls in; see internal/store/discovery.go.
package discovery

import (
	"hash/fnv"
	"strconv"
)

// Jitter returns a stable pseudo-random value in [0,1) for one item under one
// seed. It is stateless rather than drawn from an RNG so an item gets the same
// jitter no matter which page it lands on — that is what makes a seeded feed
// pageable. kind ("al"/"pl") is part of the hash because an album and a playlist
// can share a numeric id.
func Jitter(seed int64, kind string, id uint) float64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strconv.FormatInt(seed, 10)))
	_, _ = h.Write([]byte(kind))
	_, _ = h.Write([]byte(strconv.FormatUint(uint64(id), 10)))
	// Drop the top bit before converting: uint64 -> float64 is exact only up to
	// 2^53, and dividing by 2^63 keeps the result strictly below 1.
	return float64(h.Sum64()&0x7fffffffffffffff) / float64(1<<63)
}
