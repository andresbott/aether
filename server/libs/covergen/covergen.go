// Package covergen generates deterministic abstract cover-art images for
// albums that have no real cover. The same (seed, size) pair always produces
// byte-identical output.
//
// Rendering styles implement the Style interface (see style.go) and live in
// their own packages; a Generator dispatches across an explicit set of them.
package covergen

import (
	"encoding/binary"
	"math/rand/v2"
)

// rngFromHash builds a deterministic RNG from the seed hash, using the first
// 16 bytes as the two 64-bit state words for PCG.
func rngFromHash(h [32]byte) *rand.Rand {
	s1 := binary.BigEndian.Uint64(h[0:8])
	s2 := binary.BigEndian.Uint64(h[8:16])
	return rand.New(rand.NewPCG(s1, s2)) //nolint:gosec // G404: deterministic seeded RNG for reproducible cover art, not security-sensitive
}
