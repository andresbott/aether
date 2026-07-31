// Package covergen generates deterministic abstract cover-art images for
// albums that have no real cover. The same (seed, size) pair always produces
// byte-identical output.
//
// Six rendering styles are available. Generate picks one deterministically
// from the seed hash so a library of albums gets a diverse mix; GenerateStyle
// renders a specific style.
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

// Style identifies one of the rendering algorithms.
type Style int

const (
	// StyleClassic is the original look: a two-colour diagonal gradient
	// with translucent white shapes.
	StyleClassic Style = iota
	// StyleBauhaus tiles the canvas with bold geometric poster motifs.
	StyleBauhaus
	// StyleRings draws eccentric, wobbly, or interfering ring systems.
	StyleRings
	// StyleWaves paints a synthwave sunset of layered sine hills.
	StyleWaves
	// StylePoster composes a duotone split with an oversized disc motif.
	StylePoster
	// StyleRemix is the classic architecture with vivid duotone
	// backgrounds and colour-tinted shapes.
	StyleRemix

	numStyles // keep last
)

// Styles lists every available style in rendering-dispatch order.
func Styles() []Style {
	out := make([]Style, numStyles)
	for i := range out {
		out[i] = Style(i)
	}
	return out
}

func (s Style) String() string {
	switch s {
	case StyleClassic:
		return "classic"
	case StyleBauhaus:
		return "bauhaus"
	case StyleRings:
		return "rings"
	case StyleWaves:
		return "waves"
	case StylePoster:
		return "poster"
	case StyleRemix:
		return "remix"
	}
	return fmt.Sprintf("Style(%d)", int(s))
}

// ParseStyle maps a style name (as returned by Style.String) back to its
// Style. It reports false for unknown names.
func ParseStyle(name string) (Style, bool) {
	for _, s := range Styles() {
		if s.String() == name {
			return s, true
		}
	}
	return 0, false
}

// styleSpec couples a style's draw function with its post-processing.
type styleSpec struct {
	draw  func(img *image.RGBA, rng *rand.Rand)
	grain int // film-grain amplitude; 0 disables
}

var styleSpecs = [numStyles]styleSpec{
	StyleClassic: {draw: drawClassic, grain: 0},
	StyleBauhaus: {draw: drawBauhaus, grain: 0},
	StyleRings:   {draw: drawRings, grain: 3},
	StyleWaves:   {draw: drawWaves, grain: 3},
	StylePoster:  {draw: drawPoster, grain: 5},
	StyleRemix:   {draw: drawRemix, grain: 4},
}

// Generate produces a deterministic abstract cover as PNG bytes. The style is
// derived from the seed hash (see StyleFor), so different seeds spread across
// all styles; seed also drives all colour and shape choices. size is the
// output width and height in pixels.
func Generate(seed string, size int) ([]byte, error) {
	h := sha256.Sum256([]byte(seed))
	return render(h, styleFromHash(h), size)
}

// GenerateStyle produces a deterministic cover in the given style.
func GenerateStyle(seed string, size int, style Style) ([]byte, error) {
	if style < 0 || style >= numStyles {
		return nil, fmt.Errorf("covergen: unknown style %d", int(style))
	}
	h := sha256.Sum256([]byte(seed))
	return render(h, style, size)
}

// StyleFor reports which style Generate will pick for seed.
func StyleFor(seed string) Style {
	return styleFromHash(sha256.Sum256([]byte(seed)))
}

// styleFromHash selects the style from hash byte 16, which the RNG state
// (bytes 0..15) does not consume, so style choice and per-style randomness
// stay independent.
func styleFromHash(h [32]byte) Style {
	return Style(h[16] % uint8(numStyles))
}

// render draws the image at double resolution, downsamples 2x for
// anti-aliasing, applies the style's grain, and encodes PNG.
func render(h [32]byte, style Style, size int) ([]byte, error) {
	if size <= 0 {
		return nil, fmt.Errorf("covergen: size must be > 0, got %d", size)
	}

	rng := rngFromHash(h)
	spec := styleSpecs[style]

	big := image.NewRGBA(image.Rect(0, 0, size*2, size*2))
	spec.draw(big, rng)
	img := downsample2x(big)
	if spec.grain > 0 {
		addGrain(img, rng, spec.grain)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("covergen: encode png: %w", err)
	}
	return buf.Bytes(), nil
}

// rngFromHash builds a deterministic RNG from the seed hash, using the first
// 16 bytes as the two 64-bit state words for PCG.
func rngFromHash(h [32]byte) *rand.Rand {
	s1 := binary.BigEndian.Uint64(h[0:8])
	s2 := binary.BigEndian.Uint64(h[8:16])
	return rand.New(rand.NewPCG(s1, s2)) //nolint:gosec // G404: deterministic seeded RNG for reproducible cover art, not security-sensitive
}

// drawClassic is the original covergen look: two-colour diagonal gradient
// background with 2..4 translucent white shapes.
func drawClassic(img *image.RGBA, rng *rand.Rand) {
	drawGradient(img, rng)
	drawForeground(img, rng)
}
