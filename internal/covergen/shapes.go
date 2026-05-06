package covergen

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
)

type shapeKind int

const (
	shapeTriangle shapeKind = iota
	shapeInvTriangle
	shapeCircle
	shapeSquare
	shapeHalfCircle
	shapeDiamond
	shapeCornerTL
	shapeCornerTR
	shapeCornerBL
	shapeCornerBR
	shapeRightTriTL
	shapeRightTriTR
	shapeRightTriBL
	shapeRightTriBR
)

func isCornerShape(k shapeKind) bool {
	return k >= shapeCornerTL && k <= shapeCornerBR
}

func isRightTriShape(k shapeKind) bool {
	return k >= shapeRightTriTL && k <= shapeRightTriBR
}

// rightTriOffset returns how much to shift the 90° vertex from the canvas
// centre so the triangle's bounding box is centred on the canvas.
func rightTriOffset(k shapeKind, radius int) (int, int) {
	half := radius / 2
	switch k {
	case shapeRightTriTL:
		return -half, -half // vertex up-left, legs extend down-right
	case shapeRightTriTR:
		return half, -half
	case shapeRightTriBL:
		return -half, half
	case shapeRightTriBR:
		return half, half
	}
	return 0, 0
}

type composition int

const (
	compScatter composition = iota // shapes at random offsets
	compNested                     // shapes share centre, decreasing radius
	compStacked                    // shapes cascade along one axis
)

// drawForeground paints 2..4 white translucent shapes of the same kind over
// img. Each shape is filled with a linear alpha gradient from fully
// transparent to opaque-ish white, in a seeded direction, so the shape
// fades across its body. Composition is one of scatter / nested / stacked,
// giving distinctive layouts reminiscent of Apple Music genre tiles.
func drawForeground(img *image.RGBA, rng *rand.Rand) {
	kind := shapeKind(rng.IntN(14))
	size := img.Bounds().Dx()

	var cx, cy, primRadius int
	switch {
	case isCornerShape(kind):
		primRadius = int(float64(size) * (0.70 + rng.Float64()*0.20))
		cx, cy = cornerPos(kind, size)
	case isRightTriShape(kind):
		primRadius = int(float64(size) * (0.70 + rng.Float64()*0.20))
		ox, oy := rightTriOffset(kind, primRadius)
		cx = size/2 + ox
		cy = size/2 + oy
	default:
		primRadius = int(float64(size) * (0.28 + rng.Float64()*0.10))
		cx = size/2 + rng.IntN(size/10) - size/20
		cy = size/2 + rng.IntN(size/10) - size/20
		// Half circle's body only extends above its anchor (fy <= 0), so
		// shift the anchor down by half the radius to centre the visible
		// shape on the canvas.
		if kind == shapeHalfCircle {
			cy += primRadius / 2
		}
	}

	// Corner and right-triangle shapes always nest at their anchor so the
	// 90° vertex stays aligned across shapes. Others pick a composition.
	comp := compNested
	if !isCornerShape(kind) && !isRightTriShape(kind) {
		comp = composition(rng.IntN(3))
	}

	drawShape(img, kind, cx, cy, primRadius, 200, rng.Float64()*2*math.Pi)

	extra := 1 + rng.IntN(3) // 1..3 additional → 2..4 total
	alphas := []uint8{140, 100, 75}

	// Stacked composition needs a shared offset vector so every subsequent
	// shape drifts in the same direction (like Spa's rising circles).
	stackDX, stackDY := 0, 0
	if comp == compStacked {
		a := rng.Float64() * 2 * math.Pi
		step := size / 8
		stackDX = int(math.Round(math.Cos(a) * float64(step)))
		stackDY = int(math.Round(math.Sin(a) * float64(step)))
	}

	for i := 0; i < extra; i++ {
		shrink := 0.55 + rng.Float64()*0.15
		r := int(float64(primRadius) * math.Pow(shrink, float64(i+1)))
		// Recompute the anchor for shapes whose visible bounding box isn't
		// centred on their logical centre (half circle, right triangles):
		// shifting by r/2 keeps the shrunk shape visually aligned with the
		// primary.
		ecx, ecy := cx, cy
		switch {
		case isRightTriShape(kind):
			ox, oy := rightTriOffset(kind, r)
			ecx = size/2 + ox
			ecy = size/2 + oy
		case kind == shapeHalfCircle:
			ecx = size / 2
			ecy = size/2 + r/2
		}
		switch comp {
		case compScatter:
			ecx += rng.IntN(size/4) - size/8
			ecy += rng.IntN(size/4) - size/8
		case compStacked:
			ecx += stackDX * (i + 1)
			ecy += stackDY * (i + 1)
		}
		drawShape(img, kind, ecx, ecy, r, alphas[i], rng.Float64()*2*math.Pi)
	}
}

func cornerPos(kind shapeKind, size int) (int, int) {
	switch kind {
	case shapeCornerTL:
		return 0, 0
	case shapeCornerTR:
		return size, 0
	case shapeCornerBL:
		return 0, size
	case shapeCornerBR:
		return size, size
	}
	return 0, 0
}

// drawShape paints kind at (cx, cy) with the given "radius" (half-extent)
// onto img. The shape is always white; each pixel's alpha is a linear
// gradient along gradientAngle within the shape's bounding circle, going
// from 0 at the "trailing" side to maxAlpha at the "leading" side.
func drawShape(img *image.RGBA, kind shapeKind, cx, cy, radius int, maxAlpha uint8, gradientAngle float64) {
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
			if !insideShape(kind, dx, dy, radius) {
				continue
			}
			// t in [0, 1] along the gradient direction, clamped at the
			// bounding-circle edges.
			t := (float64(dx)*gx + float64(dy)*gy + r) / (2 * r)
			if t < 0 {
				t = 0
			} else if t > 1 {
				t = 1
			}
			a := uint8(math.Round(float64(maxAlpha) * t))
			blendPixel(img, x, y, color.RGBA{R: 255, G: 255, B: 255, A: a})
		}
	}
}

// insideShape reports whether the offset (dx, dy) from the shape's centre
// lies inside a shape of kind with half-extent radius.
func insideShape(kind shapeKind, dx, dy, radius int) bool {
	r := float64(radius)
	fx, fy := float64(dx), float64(dy)
	switch kind {
	case shapeTriangle:
		// Upward equilateral triangle: apex at (0, -r), base at y=+r/2.
		if fy > r/2 || fy < -r {
			return false
		}
		t := (fy + r) / (r * 1.5) // 0 at apex, 1 at base
		halfBase := t * r
		return math.Abs(fx) <= halfBase
	case shapeInvTriangle:
		if fy < -r/2 || fy > r {
			return false
		}
		t := (r - fy) / (r * 1.5)
		halfBase := t * r
		return math.Abs(fx) <= halfBase
	case shapeCircle:
		return fx*fx+fy*fy <= r*r
	case shapeSquare:
		return math.Abs(fx) <= r && math.Abs(fy) <= r
	case shapeHalfCircle:
		if fy > 0 {
			return false
		}
		return fx*fx+fy*fy <= r*r
	case shapeDiamond:
		return math.Abs(fx)+math.Abs(fy) <= r
	case shapeCornerTL:
		// 90° at (0,0), legs along +x and +y to length r.
		return fx >= 0 && fy >= 0 && fx+fy <= r
	case shapeCornerTR:
		// 90° at (0,0), legs along -x and +y.
		return fx <= 0 && fy >= 0 && -fx+fy <= r
	case shapeCornerBL:
		// 90° at (0,0), legs along +x and -y.
		return fx >= 0 && fy <= 0 && fx-fy <= r
	case shapeCornerBR:
		// 90° at (0,0), legs along -x and -y.
		return fx <= 0 && fy <= 0 && -fx-fy <= r
	case shapeRightTriTL:
		return fx >= 0 && fy >= 0 && fx+fy <= r
	case shapeRightTriTR:
		return fx <= 0 && fy >= 0 && -fx+fy <= r
	case shapeRightTriBL:
		return fx >= 0 && fy <= 0 && fx-fy <= r
	case shapeRightTriBR:
		return fx <= 0 && fy <= 0 && -fx-fy <= r
	}
	return false
}

// blendPixel alpha-blends src over the existing pixel at (x, y) in img.
// Uses straight-alpha "over" compositing.
func blendPixel(img *image.RGBA, x, y int, src color.RGBA) {
	dst := img.RGBAAt(x, y)
	sa := float64(src.A) / 255
	img.SetRGBA(x, y, color.RGBA{
		R: uint8(math.Round(float64(src.R)*sa + float64(dst.R)*(1-sa))),
		G: uint8(math.Round(float64(src.G)*sa + float64(dst.G)*(1-sa))),
		B: uint8(math.Round(float64(src.B)*sa + float64(dst.B)*(1-sa))),
		A: 255,
	})
}
