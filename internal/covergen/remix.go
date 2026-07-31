package covergen

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
)

type remixShape int

const (
	rsCircle remixShape = iota
	rsSquare
	rsDiamond
	rsTriangle
	rsInvTriangle
	rsHalfCircle
)

// drawRemix keeps the classic covergen architecture — gradient background,
// 3..4 nested/stacked/scattered translucent shapes with alpha gradients —
// but swaps in vivid duotone backgrounds (diagonal, vertical, horizontal or
// radial) and tints the shapes with accent colours instead of white-only.
func drawRemix(img *image.RGBA, rng *rand.Rand) {
	sz := img.Bounds().Dx()
	fs := float64(sz)
	cols := vivid(rng, 4)
	c1, c2 := cols[0], cols[1]

	// Background gradient.
	dir := rng.IntN(4)
	rcx := fs * (0.2 + rng.Float64()*0.6)
	rcy := fs * (0.2 + rng.Float64()*0.6)
	rmax := 0.0
	for _, p := range [][2]float64{{0, 0}, {fs, 0}, {0, fs}, {fs, fs}} {
		d := math.Hypot(p[0]-rcx, p[1]-rcy)
		if d > rmax {
			rmax = d
		}
	}
	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			var t float64
			switch dir {
			case 0:
				t = float64(x+y) / (2 * (fs - 1))
			case 1:
				t = float64(y) / (fs - 1)
			case 2:
				t = float64(x) / (fs - 1)
			default:
				t = math.Hypot(float64(x)-rcx, float64(y)-rcy) / rmax
			}
			img.SetRGBA(x, y, lerpRGBA(c1, c2, t))
		}
	}

	// Foreground shapes.
	kind := remixShape(rng.IntN(6))
	comp := rng.IntN(3) // 0 scatter, 1 nested, 2 stacked
	shapeCol := cols[2]
	altCol := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	if rng.IntN(4) == 0 {
		altCol = cols[3]
	}

	primR := int(fs * (0.30 + rng.Float64()*0.14))
	cx := sz/2 + rng.IntN(sz/8) - sz/16
	cy := sz/2 + rng.IntN(sz/8) - sz/16
	if kind == rsHalfCircle {
		cy += primR / 2
	}

	drawRemixShape(img, kind, cx, cy, primR, shapeCol, 230, rng.Float64()*2*math.Pi)

	stackDX, stackDY := 0, 0
	if comp == 2 {
		a := rng.Float64() * 2 * math.Pi
		step := sz / 7
		stackDX = int(math.Round(math.Cos(a) * float64(step)))
		stackDY = int(math.Round(math.Sin(a) * float64(step)))
	}

	extra := 2 + rng.IntN(2) // 2..3 additional → 3..4 total
	alphas := []uint8{180, 140, 110}
	for i := 0; i < extra; i++ {
		shrink := 0.55 + rng.Float64()*0.15
		r := int(float64(primR) * math.Pow(shrink, float64(i+1)))
		ecx, ecy := cx, cy
		if kind == rsHalfCircle {
			ecx = sz / 2
			ecy = sz/2 + r/2
		}
		switch comp {
		case 0:
			ecx += rng.IntN(sz/3) - sz/6
			ecy += rng.IntN(sz/3) - sz/6
		case 2:
			ecx += stackDX * (i + 1)
			ecy += stackDY * (i + 1)
		}
		col := altCol
		if i%2 == 1 {
			col = shapeCol
		}
		drawRemixShape(img, kind, ecx, ecy, r, col, alphas[i%len(alphas)], rng.Float64()*2*math.Pi)
	}

	// Occasional thin outline ring to break the composition.
	if rng.IntN(3) == 0 {
		ox := fs * (0.25 + rng.Float64()*0.5)
		oy := fs * (0.25 + rng.Float64()*0.5)
		or := fs * (0.30 + rng.Float64()*0.25)
		ow := fs * 0.012
		for y := 0; y < sz; y++ {
			for x := 0; x < sz; x++ {
				d := math.Hypot(float64(x)-ox, float64(y)-oy)
				if math.Abs(d-or) <= ow {
					blendPixel(img, x, y, color.RGBA{R: 255, G: 255, B: 255, A: 200})
				}
			}
		}
	}
}

// drawRemixShape mirrors drawShape: fill kind at (cx, cy) with an alpha
// gradient along gradientAngle from a floor up to maxAlpha, tinted col.
func drawRemixShape(img *image.RGBA, kind remixShape, cx, cy, radius int, col color.RGBA, maxAlpha uint8, gradientAngle float64) {
	bounds := img.Bounds()
	x0 := max(bounds.Min.X, cx-radius-1)
	y0 := max(bounds.Min.Y, cy-radius-1)
	x1 := min(bounds.Max.X, cx+radius+1)
	y1 := min(bounds.Max.Y, cy+radius+1)

	gx := math.Cos(gradientAngle)
	gy := math.Sin(gradientAngle)
	r := float64(radius)

	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			dx, dy := x-cx, y-cy
			if !insideRemixShape(kind, dx, dy, radius) {
				continue
			}
			t := (float64(dx)*gx + float64(dy)*gy + r) / (2 * r)
			if t < 0 {
				t = 0
			} else if t > 1 {
				t = 1
			}
			// Keep a floor so the trailing edge doesn't vanish entirely.
			a := uint8(math.Round(float64(maxAlpha) * (0.25 + 0.75*t)))
			blendPixel(img, x, y, color.RGBA{R: col.R, G: col.G, B: col.B, A: a})
		}
	}
}

func insideRemixShape(kind remixShape, dx, dy, radius int) bool {
	r := float64(radius)
	fx, fy := float64(dx), float64(dy)
	switch kind {
	case rsCircle:
		return fx*fx+fy*fy <= r*r
	case rsSquare:
		return math.Abs(fx) <= r && math.Abs(fy) <= r
	case rsDiamond:
		return math.Abs(fx)+math.Abs(fy) <= r
	case rsTriangle:
		if fy > r/2 || fy < -r {
			return false
		}
		t := (fy + r) / (r * 1.5)
		return math.Abs(fx) <= t*r
	case rsInvTriangle:
		if fy < -r/2 || fy > r {
			return false
		}
		t := (r - fy) / (r * 1.5)
		return math.Abs(fx) <= t*r
	case rsHalfCircle:
		if fy > 0 {
			return false
		}
		return fx*fx+fy*fy <= r*r
	}
	return false
}
