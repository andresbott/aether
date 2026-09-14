// Package paint provides the style-agnostic colour, blend, and clamp helpers
// shared by covergen's rendering styles.
package paint

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
)

// Hsl converts HSL to RGBA like HslToRGBA but first normalises h into
// [0, 360), so callers can pass hue arithmetic results directly.
func Hsl(h, s, l float64) color.RGBA {
	return HslToRGBA(math.Mod(math.Mod(h, 360)+360, 360), s, l)
}

// HslToRGBA converts HSL (h in degrees 0..360, s and l in 0..1) to RGBA with
// alpha 255.
func HslToRGBA(h, s, l float64) color.RGBA {
	c := (1 - math.Abs(2*l-1)) * s
	hp := h / 60
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	var r, g, b float64
	switch {
	case hp < 1:
		r, g, b = c, x, 0
	case hp < 2:
		r, g, b = x, c, 0
	case hp < 3:
		r, g, b = 0, c, x
	case hp < 4:
		r, g, b = 0, x, c
	case hp < 5:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}
	m := l - c/2
	return color.RGBA{
		R: uint8(math.Round((r + m) * 255)),
		G: uint8(math.Round((g + m) * 255)),
		B: uint8(math.Round((b + m) * 255)),
		A: 255,
	}
}

// Palette picks two harmonious RGBA colours. Base hue is random; second hue
// is shifted 20..60 degrees either way. Overall brightness varies widely
// across seeds so different albums land in distinctly dark, muted, or
// pastel palettes.
func Palette(rng *rand.Rand, satMul, hueMul, satSpread float64) (color.RGBA, color.RGBA) {
	hue1 := rng.Float64() * 360
	shift := (20 + rng.Float64()*40) * hueMul
	if rng.IntN(2) == 0 {
		shift = -shift
	}
	hue2 := math.Mod(hue1+shift+360, 360)

	// Per-seed brightness centre spans from near-black to near-white.
	base := 0.18 + rng.Float64()*0.62 // 0.18..0.80

	// Saturation tapers toward the extremes: pastels stay soft, very dark
	// palettes don't turn cartoonish.
	dist := math.Abs(base-0.49) / 0.31 // 0 at middle, 1 at extremes
	rsat := rng.Float64()
	sat := ClampFloat(satMul*(0.55-0.28*dist+rsat*0.08)+(satSpread-1)*0.08*(rsat-0.5), 0, 1)

	// Two gradient endpoints spread around the brightness centre.
	delta := 0.10 + rng.Float64()*0.08
	l1 := ClampFloat(base-delta, 0.06, 0.94)
	l2 := ClampFloat(base+delta, 0.06, 0.94)
	return HslToRGBA(hue1, sat, l1), HslToRGBA(hue2, sat, l2)
}

// Vivid returns n saturated colours built from a random harmony scheme
// (complementary, triadic, analogous, split-complementary).
func Vivid(rng *rand.Rand, n int, satMul, hueMul, satSpread float64) []color.RGBA {
	base := rng.Float64() * 360
	schemes := [][]float64{
		{0, 180, 30, 210, 60},
		{0, 120, 240, 60, 180},
		{0, 30, 60, -30, 90},
		{0, 150, 210, 30, 180},
	}
	offs := schemes[rng.IntN(len(schemes))]
	out := make([]color.RGBA, n)
	for i := range out {
		h := base + offs[i%len(offs)]*hueMul
		rs := rng.Float64()
		s := ClampFloat(satMul*(0.65+rs*0.30)+(satSpread-1)*0.30*(rs-0.5), 0, 1)
		l := 0.40 + rng.Float64()*0.25
		out[i] = Hsl(h, s, l)
	}
	return out
}

// LerpRGBA returns the linear interpolation of a and b at parameter t in [0,1].
func LerpRGBA(a, b color.RGBA, t float64) color.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return color.RGBA{
		R: uint8(math.Round(float64(a.R) + (float64(b.R)-float64(a.R))*t)),
		G: uint8(math.Round(float64(a.G) + (float64(b.G)-float64(a.G))*t)),
		B: uint8(math.Round(float64(a.B) + (float64(b.B)-float64(a.B))*t)),
		A: 255,
	}
}

// BlendPixel alpha-blends src over the existing pixel at (x, y) in img.
// Uses straight-alpha "over" compositing.
func BlendPixel(img *image.RGBA, x, y int, src color.RGBA) {
	dst := img.RGBAAt(x, y)
	sa := float64(src.A) / 255
	img.SetRGBA(x, y, color.RGBA{
		R: uint8(math.Round(float64(src.R)*sa + float64(dst.R)*(1-sa))),
		G: uint8(math.Round(float64(src.G)*sa + float64(dst.G)*(1-sa))),
		B: uint8(math.Round(float64(src.B)*sa + float64(dst.B)*(1-sa))),
		A: 255,
	})
}

// ClampFloat clamps v to the closed interval [lo, hi].
func ClampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ClampU8 clamps v to the range of a uint8 (0..255).
func ClampU8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// Sign returns -1 for negative v and 1 otherwise (including zero).
func Sign(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// SampleAround draws a value from a normal distribution centred on center with
// standard deviation width: width is "how far samples typically stray" — 0
// always returns center, larger values make far-from-center draws more likely.
// It always consumes exactly one rng sample, so changing center/width shifts
// the value without reshuffling later draws. Callers clamp to their valid range.
func SampleAround(rng *rand.Rand, center, width float64) float64 {
	return center + width*rng.NormFloat64()
}

// HueMultiplier draws the per-cover hue-spread multiplier from a normal
// distribution centred on center with standard deviation spread. spread 0
// returns center deterministically with no draw (so it is identity at default);
// larger spread widens the bell curve, making lower/higher deltas more likely.
// Clamped to >= 0.
func HueMultiplier(rng *rand.Rand, center, spread float64) float64 {
	if spread <= 0 {
		return center
	}
	if v := center + spread*rng.NormFloat64(); v > 0 {
		return v
	}
	return 0
}

// Luminance returns the perceptual luminance of c in 0..255 (Rec. 601 weights).
// Callers use it to pick the darker of two colours.
func Luminance(c color.RGBA) float64 {
	return 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)
}
