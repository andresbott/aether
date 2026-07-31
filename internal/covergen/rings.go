package covergen

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
)

// drawRings has three per-seed variants: classic eccentric rings, wobbly
// rings whose radius oscillates around the angle, and a two-centre
// interference pattern (moiré hyperbolas). Palettes mix a deep background
// with four vivid accents.
func drawRings(img *image.RGBA, rng *rand.Rand) {
	sz := img.Bounds().Dx()
	fs := float64(sz)

	accents := vivid(rng, 4)
	bg := hsl(rng.Float64()*360, 0.45+rng.Float64()*0.25, 0.08+rng.Float64()*0.10)
	var pal []color.RGBA
	if rng.IntN(2) == 0 {
		// Calmer: background breathes between accents.
		pal = []color.RGBA{bg, accents[0], bg, accents[1], bg, accents[2]}
	} else {
		// Louder: accents chain with one dark beat.
		pal = []color.RGBA{bg, accents[0], accents[1], bg, accents[2], accents[3]}
	}

	variant := rng.IntN(3)

	cx := fs * (0.15 + rng.Float64()*0.70)
	cy := fs * (0.15 + rng.Float64()*0.70)
	maxD := 0.0
	for _, p := range [][2]float64{{0, 0}, {fs, 0}, {0, fs}, {fs, fs}} {
		d := math.Hypot(p[0]-cx, p[1]-cy)
		if d > maxD {
			maxD = d
		}
	}

	if variant == 2 {
		// Interference: two regular ring systems, colour indexed by the sum
		// of both band numbers.
		cx2 := fs * (0.15 + rng.Float64()*0.70)
		cy2 := fs * (0.15 + rng.Float64()*0.70)
		// Keep the centres apart so the hyperbola family is visible.
		if math.Hypot(cx2-cx, cy2-cy) < fs*0.35 {
			cx2 = cx + fs*0.45*sign(cx2-cx)
			cy2 = cy + fs*0.45*sign(cy2-cy)
		}
		step := fs * (0.05 + rng.Float64()*0.06)
		for y := 0; y < sz; y++ {
			for x := 0; x < sz; x++ {
				d1 := math.Hypot(float64(x)-cx, float64(y)-cy)
				d2 := math.Hypot(float64(x)-cx2, float64(y)-cy2)
				band := int(d1/step) + int(d2/step)
				img.SetRGBA(x, y, pal[band%len(pal)])
			}
		}
		return
	}

	// Classic / wobble share the irregular band table.
	var edges []float64
	e := fs * (0.04 + rng.Float64()*0.10) // inner disc radius
	for e < maxD+fs*0.1 {
		edges = append(edges, e)
		e += fs * (0.035 + rng.Float64()*0.085)
	}
	edges = append(edges, maxD+fs*0.2)

	wobAmp, wobFreq, wobPh := 0.0, 0.0, 0.0
	if variant == 1 {
		wobAmp = fs * (0.012 + rng.Float64()*0.030)
		wobFreq = float64(3 + rng.IntN(7)) // 3..9 lobes
		wobPh = rng.Float64() * 2 * math.Pi
	}

	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			d := math.Hypot(dx, dy)
			if wobAmp > 0 {
				d += wobAmp * math.Sin(wobFreq*math.Atan2(dy, dx)+wobPh)
			}
			band := len(edges) - 1
			for i, ed := range edges {
				if d < ed {
					band = i
					break
				}
			}
			img.SetRGBA(x, y, pal[band%len(pal)])
		}
	}
}
