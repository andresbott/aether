package covergen

import (
	"image"
	"math"
	"math/rand/v2"
)

// drawWaves paints a synthwave-style poster: a vertical sky ramp glowing at
// the horizon, an optional flat sun disc (sometimes with scanline cuts), and
// opaque wave layers that darken and grow as they approach the viewer.
func drawWaves(img *image.RGBA, rng *rand.Rand) {
	sz := img.Bounds().Dx()
	fs := float64(sz)

	baseHue := rng.Float64() * 360
	span := 40 + rng.Float64()*80
	if rng.IntN(2) == 0 {
		span = -span
	}

	// Sky: dark at the top, glowing near the horizon.
	skyShift := 30 + rng.Float64()*60
	if rng.IntN(2) == 0 {
		skyShift = -skyShift
	}
	skyTop := hsl(baseHue+skyShift, 0.55+rng.Float64()*0.25, 0.12+rng.Float64()*0.10)
	glow := hsl(baseHue, 0.80+rng.Float64()*0.15, 0.62+rng.Float64()*0.12)
	horizonY := fs * (0.40 + rng.Float64()*0.12)
	for y := 0; y < sz; y++ {
		t := float64(y) / horizonY
		if t > 1 {
			t = 1
		}
		c := lerpRGBA(skyTop, glow, t)
		for x := 0; x < sz; x++ {
			img.SetRGBA(x, y, c)
		}
	}

	// Sun disc, flat, near the horizon.
	if rng.IntN(10) < 7 {
		sunR := fs * (0.10 + rng.Float64()*0.15)
		sunX := fs * (0.25 + rng.Float64()*0.50)
		sunY := horizonY - sunR*(rng.Float64()*0.8)
		sun := hsl(baseHue+rng.Float64()*40-20, 0.85, 0.80+rng.Float64()*0.12)
		scanlines := rng.IntN(2) == 0
		for y := 0; y < sz; y++ {
			fy := float64(y) - sunY
			if scanlines && fy > 0 {
				// Cut horizontal slits out of the lower half, wider near
				// the bottom.
				p := fy / sunR // 0..1 down the lower half
				if math.Mod(p*6, 1) < p*0.55 {
					continue
				}
			}
			for x := 0; x < sz; x++ {
				fx := float64(x) - sunX
				if fx*fx+fy*fy <= sunR*sunR {
					img.SetRGBA(x, y, sun)
				}
			}
		}
	}

	layers := 4 + rng.IntN(3)
	for i := 0; i < layers; i++ {
		front := float64(i) / float64(layers-1) // 0 back .. 1 front
		baseY := fs * (0.44 + 0.48*front)
		amp1 := fs * (0.025 + 0.075*front + rng.Float64()*0.03)
		amp2 := amp1 * (0.2 + rng.Float64()*0.4)
		freq1 := 0.8 + rng.Float64()*1.8
		freq2 := freq1 * (2 + rng.Float64())
		ph1 := rng.Float64() * 2 * math.Pi
		ph2 := rng.Float64() * 2 * math.Pi

		h := baseHue + span*front
		l := 0.60 - 0.46*front // recede bright, advance dark
		c := hsl(h, 0.65+rng.Float64()*0.25, l)

		for x := 0; x < sz; x++ {
			fx := float64(x) / fs
			y0 := baseY +
				amp1*math.Sin(2*math.Pi*freq1*fx+ph1) +
				amp2*math.Sin(2*math.Pi*freq2*fx+ph2)
			yi := int(y0)
			if yi < 0 {
				yi = 0
			}
			for y := yi; y < sz; y++ {
				img.SetRGBA(x, y, c)
			}
		}
	}
}
