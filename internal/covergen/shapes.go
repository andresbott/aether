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

// classicKnobs tune the original gradient+shapes look (consumed by
// drawForeground). Applied after the per-seed random draws (Default 1.0 =
// shipped look).
var classicKnobs = []Knob{
	{Name: "classic.shapes", Label: "Shape count", Min: 0, Max: 3, Step: 0.05, Default: 1},
	{Name: "classic.shapesSpread", Label: "Shape count spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "classic.position", Label: "Shape position", Min: 0, Max: 6, Step: 0.05, Default: 1},
	{Name: "classic.size", Label: "Shape size", Min: 0.3, Max: 2, Step: 0.05, Default: 1},
	{Name: "classic.sizeSpread", Label: "Shape size spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "classic.opacityCenter", Label: "Opacity center", Min: 0, Max: 1, Step: 0.05, Default: 0.6},
	{Name: "classic.opacityWidth", Label: "Opacity spread", Min: 0, Max: 0.5, Step: 0.02, Default: 0.15},
	{Name: "classic.saturation", Label: "Saturation", Min: 0, Max: 1.5, Step: 0.05, Default: 1},
	{Name: "classic.saturationSpread", Label: "Saturation spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "classic.hue", Label: "Hue", Min: 0, Max: 3, Step: 0.05, Default: 1.6},
	{Name: "classic.hueSpread", Label: "Hue spread", Min: 0, Max: 10, Step: 0.05, Default: 0},
}

// drawForeground paints 2..4 white translucent shapes of the same kind over
// img. Each shape is filled with a linear alpha gradient from fully
// transparent to opaque-ish white, in a seeded direction, so the shape
// fades across its body. Composition is one of scatter / nested / stacked,
// giving distinctive layouts reminiscent of Apple Music genre tiles.
func drawForeground(img *image.RGBA, rng *rand.Rand, ks knobSet) {
	kind := shapeKind(rng.IntN(14))
	size := img.Bounds().Dx()
	shapesMul := ks.Float("classic.shapes")
	shapesSpread := ks.Float("classic.shapesSpread")
	positionMul := ks.Float("classic.position")
	sizeMul := ks.Float("classic.size")
	sizeSpread := ks.Float("classic.sizeSpread")
	ocenter := ks.Float("classic.opacityCenter")
	owidth := ks.Float("classic.opacityWidth")
	randAlpha := func() uint8 {
		return clampU8(int(math.Round(clampFloat(sampleAround(rng, ocenter, owidth), 0, 1) * 255)))
	}
	// posShift displaces a shape from its anchor by an amount that grows with
	// the position knob and is zero at the default, so corner/edge-anchored
	// shapes stay put until position is cranked.
	posShift := func() int {
		if positionMul == 1 {
			return 0
		}
		return int(float64(rng.IntN(size/6)-size/12) * (positionMul*positionMul - 1))
	}

	var cx, cy, primRadius int
	switch {
	case isCornerShape(kind):
		rc := rng.Float64()
		primRadius = int(float64(size) * (sizeMul*(0.70+rc*0.20) + (sizeSpread-1)*0.20*(rc-0.5)))
		cx, cy = cornerPos(kind, size)
		cx += posShift()
		cy += posShift()
	case isRightTriShape(kind):
		rc := rng.Float64()
		primRadius = int(float64(size) * (sizeMul*(0.70+rc*0.20) + (sizeSpread-1)*0.20*(rc-0.5)))
		ox, oy := rightTriOffset(kind, primRadius)
		cx = size/2 + ox + posShift()
		cy = size/2 + oy + posShift()
	default:
		rd := rng.Float64()
		primRadius = int(float64(size) * (sizeMul*(0.28+rd*0.10) + (sizeSpread-1)*0.10*(rd-0.5)))
		cx = size/2 + int(float64(rng.IntN(size/10)-size/20)*positionMul*positionMul)
		cy = size/2 + int(float64(rng.IntN(size/10)-size/20)*positionMul*positionMul)
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

	drawShape(img, kind, cx, cy, primRadius, randAlpha(), rng.Float64()*2*math.Pi)

	ri := rng.IntN(3)
	extra := int(shapesMul*float64(1+ri) + (shapesSpread-1)*float64(ri-1)) // scaled 1..3 additional
	if extra < 0 {
		extra = 0
	}

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
			ecx = size/2 + ox + posShift()
			ecy = size/2 + oy + posShift()
		case kind == shapeHalfCircle:
			ecx = size/2 + posShift()
			ecy = size/2 + r/2 + posShift()
		}
		switch comp {
		case compScatter:
			ecx += int(float64(rng.IntN(size/4)-size/8) * positionMul * positionMul)
			ecy += int(float64(rng.IntN(size/4)-size/8) * positionMul * positionMul)
		case compStacked:
			ecx += stackDX * (i + 1)
			ecy += stackDY * (i + 1)
		}
		drawShape(img, kind, ecx, ecy, r, randAlpha(), rng.Float64()*2*math.Pi)
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
	if isCornerShape(kind) || isRightTriShape(kind) {
		return insideWedge(kind, dx, dy, radius)
	}
	return insideBasicShape(kind, dx, dy, radius)
}

// insideBasicShape covers the centred shapes (triangles, circle, square,
// half circle, diamond).
func insideBasicShape(kind shapeKind, dx, dy, radius int) bool {
	r := float64(radius)
	fx, fy := float64(dx), float64(dy)
	switch kind {
	case shapeTriangle:
		// Upward equilateral triangle: apex at (0, -r), base at y=+r/2.
		if fy > r/2 || fy < -r {
			return false
		}
		t := (fy + r) / (r * 1.5) // 0 at apex, 1 at base
		return math.Abs(fx) <= t*r
	case shapeInvTriangle:
		if fy < -r/2 || fy > r {
			return false
		}
		t := (r - fy) / (r * 1.5)
		return math.Abs(fx) <= t*r
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
	}
	return false
}

// insideWedge covers the corner and right-triangle shapes. Each is a right
// triangle whose 90° vertex sits at the origin with legs of length r; corner
// and right-tri variants share identical geometry and differ only in where the
// caller anchors them on the canvas.
func insideWedge(kind shapeKind, dx, dy, radius int) bool {
	r := float64(radius)
	fx, fy := float64(dx), float64(dy)
	switch kind {
	case shapeCornerTL, shapeRightTriTL:
		return fx >= 0 && fy >= 0 && fx+fy <= r
	case shapeCornerTR, shapeRightTriTR:
		return fx <= 0 && fy >= 0 && -fx+fy <= r
	case shapeCornerBL, shapeRightTriBL:
		return fx >= 0 && fy <= 0 && fx-fy <= r
	case shapeCornerBR, shapeRightTriBR:
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
