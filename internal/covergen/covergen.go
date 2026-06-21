// Package covergen generates deterministic abstract cover-art images for
// albums that have no real cover. The same (seed, size) pair always produces
// byte-identical output.
package covergen

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"image/png"
	"math/rand/v2"
)

// Generate produces a deterministic abstract cover as PNG bytes.
// seed drives all colour and shape choices; size is the output width and
// height in pixels.
func Generate(seed string, size int) ([]byte, error) {
	if size <= 0 {
		return nil, fmt.Errorf("covergen: size must be > 0, got %d", size)
	}

	rng := newRNG(seed)
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	drawGradient(img, rng)
	drawForeground(img, rng)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("covergen: encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// newRNG builds a deterministic RNG from seed by hashing with SHA-256 and
// using the first 16 bytes as the two 64-bit state words for PCG.
func newRNG(seed string) *rand.Rand {
	h := sha256.Sum256([]byte(seed))
	s1 := binary.BigEndian.Uint64(h[0:8])
	s2 := binary.BigEndian.Uint64(h[8:16])
	return rand.New(rand.NewPCG(s1, s2)) //nolint:gosec // G404: deterministic seeded RNG for reproducible cover art, not security-sensitive
}
