package covergen

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
)

// drawGradient fills img with a two-colour diagonal gradient. Both colours
// are HSL-derived from rng so the palette is always harmonious.
func drawGradient(img *image.RGBA, rng *rand.Rand) {
	c1, c2 := palette(rng)
	size := img.Bounds().Dx()
	// Gradient runs from top-left (c1) to bottom-right (c2).
	maxD := float64(size-1) * 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			t := float64(x+y) / maxD
			img.SetRGBA(x, y, lerpRGBA(c1, c2, t))
		}
	}
}

// palette picks two harmonious RGBA colours. Base hue is random; second hue
// is shifted 20..60 degrees either way. Overall brightness varies widely
// across seeds so different albums land in distinctly dark, muted, or
// pastel palettes.
func palette(rng *rand.Rand) (color.RGBA, color.RGBA) {
	hue1 := rng.Float64() * 360
	shift := 20 + rng.Float64()*40
	if rng.IntN(2) == 0 {
		shift = -shift
	}
	hue2 := math.Mod(hue1+shift+360, 360)

	// Per-seed brightness centre spans from near-black to near-white.
	base := 0.18 + rng.Float64()*0.62 // 0.18..0.80

	// Saturation tapers toward the extremes: pastels stay soft, very dark
	// palettes don't turn cartoonish.
	dist := math.Abs(base-0.49) / 0.31 // 0 at middle, 1 at extremes
	sat := 0.55 - 0.28*dist + rng.Float64()*0.08

	// Two gradient endpoints spread around the brightness centre.
	delta := 0.10 + rng.Float64()*0.08
	l1 := clampFloat(base-delta, 0.06, 0.94)
	l2 := clampFloat(base+delta, 0.06, 0.94)
	return hslToRGBA(hue1, sat, l1), hslToRGBA(hue2, sat, l2)
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// hslToRGBA converts HSL (h in degrees 0..360, s and l in 0..1) to RGBA with
// alpha 255.
func hslToRGBA(h, s, l float64) color.RGBA {
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

// lerpRGBA returns the linear interpolation of a and b at parameter t in [0,1].
func lerpRGBA(a, b color.RGBA, t float64) color.RGBA {
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
