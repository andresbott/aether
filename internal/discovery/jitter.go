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
	return hashRatio(h.Sum64())
}

// hashRatio maps a 64-bit hash into [0,1). It takes the low 53 bits and divides
// by 2^53 because float64 represents every integer below 2^53 exactly, so the
// quotient is exactly below 1. Masking to 63 bits and dividing by 2^63 would NOT
// be safe: float64(2^63-1) rounds UP to 2^63 (round-to-nearest, not toward
// zero), yielding exactly 1.0.
//
// It is a separate function so a test can feed it the worst-case hash directly —
// the failing values are ~1 in 10^16, far past what sampling Jitter can reach.
func hashRatio(sum uint64) float64 {
	const mask53 = uint64(1)<<53 - 1
	return float64(sum&mask53) / float64(uint64(1)<<53)
}
