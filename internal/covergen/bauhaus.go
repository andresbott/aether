package covergen

import (
	"image"
	"image/color"
	"math/rand/v2"
)

// bauhausKnobs tune the poster grid. Applied after the per-seed random draws
// (Default 1.0 = shipped look).
var bauhausKnobs = []Knob{
	{Name: "bauhaus.cells", Label: "Grid density", Min: 0.5, Max: 3, Step: 0.25, Default: 1},
	{Name: "bauhaus.cellsSpread", Label: "Grid density spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "bauhaus.saturation", Label: "Saturation", Min: 0, Max: 1.5, Step: 0.05, Default: 1},
	{Name: "bauhaus.saturationSpread", Label: "Saturation spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "bauhaus.hue", Label: "Hue", Min: 0, Max: 3, Step: 0.05, Default: 1},
	{Name: "bauhaus.hueSpread", Label: "Hue spread", Min: 0, Max: 10, Step: 0.05, Default: 0},
}

// drawBauhaus tiles the canvas 2x2 or 3x3 and fills each cell with a bold
// geometric motif (quarter disc, half disc, disc, bullseye, diagonal) using a
// cream / ink / two-accent poster palette.
func drawBauhaus(img *image.RGBA, rng *rand.Rand, ks knobSet) {
	sz := img.Bounds().Dx()
	baseHue := rng.Float64() * 360
	cellMul := ks.Float("bauhaus.cells")
	cellsSpread := ks.Float("bauhaus.cellsSpread")
	sat := ks.Float("bauhaus.saturation")
	satSpread := ks.Float("bauhaus.saturationSpread")
	hueMul := hueMultiplier(rng, ks.Float("bauhaus.hue"), ks.Float("bauhaus.hueSpread"))

	rc := rng.Float64()
	cream := hsl(baseHue, clampFloat(sat*(0.25+rc*0.15)+(satSpread-1)*0.15*(rc-0.5), 0, 1), 0.90)
	ink := hsl(baseHue+rng.Float64()*40-20, 0.30, 0.13)
	ra := rng.Float64()
	acc1 := hsl(baseHue, clampFloat(sat*(0.75+ra*0.2)+(satSpread-1)*0.2*(ra-0.5), 0, 1), 0.50)
	off := []float64{150, 180, 210, 120}[rng.IntN(4)]
	ra2 := rng.Float64()
	acc2 := hsl(baseHue+off*hueMul, clampFloat(sat*(0.70+ra2*0.2)+(satSpread-1)*0.2*(ra2-0.5), 0, 1), 0.55)
	pal := []color.RGBA{cream, ink, acc1, acc2}

	rcell := rng.IntN(2)
	cells := int(cellMul*float64(2+rcell) + (cellsSpread-1)*(float64(rcell)-0.5))
	if cells < 1 {
		cells = 1
	}
	cs := sz / cells

	for cy := 0; cy < cells; cy++ {
		for cx := 0; cx < cells; cx++ {
			x0, y0 := cx*cs, cy*cs
			x1, y1 := x0+cs, y0+cs
			if cx == cells-1 {
				x1 = sz
			}
			if cy == cells-1 {
				y1 = sz
			}

			drawBauhausCell(img, rng, pal, x0, y0, x1, y1)
		}
	}
}

// drawBauhausCell fills one grid cell with a randomly chosen motif drawn in
// two (or three, for the bullseye) palette colours.
func drawBauhausCell(img *image.RGBA, rng *rand.Rand, pal []color.RGBA, x0, y0, x1, y1 int) {
	bgIdx := rng.IntN(len(pal))
	fgIdx := (bgIdx + 1 + rng.IntN(len(pal)-1)) % len(pal)
	bg, fg := pal[bgIdx], pal[fgIdx]
	motif := rng.IntN(6)

	// Motif geometry parameters chosen once per cell.
	corner := rng.IntN(4)
	edge := rng.IntN(4)
	innerIdx := (fgIdx + 1 + rng.IntN(len(pal)-1)) % len(pal)
	inner := pal[innerIdx]

	w, h := x1-x0, y1-y0
	r := float64(min(w, h))
	pick := bauhausMotif(motif, w, h, r, corner, edge, fg, inner)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			// Local coordinates in [0, w) x [0, h).
			c, ok := pick(float64(x-x0), float64(y-y0))
			if !ok {
				c = bg
			}
			img.SetRGBA(x, y, c)
		}
	}
}

// bauhausMotif returns a function classifying a cell-local point: it reports
// the motif colour and true when the point falls inside the motif.
func bauhausMotif(motif, w, h int, r float64, corner, edge int, fg, inner color.RGBA) func(lx, ly float64) (color.RGBA, bool) {
	fw, fh := float64(w), float64(h)
	switch motif {
	case 0: // quarter disc anchored at a corner
		qx := []float64{0, fw, 0, fw}[corner]
		qy := []float64{0, 0, fh, fh}[corner]
		return func(lx, ly float64) (color.RGBA, bool) {
			return fg, (lx-qx)*(lx-qx)+(ly-qy)*(ly-qy) <= r*r
		}
	case 1: // half disc, flat side on an edge
		var ex, ey float64
		switch edge {
		case 0:
			ex, ey = fw/2, 0
		case 1:
			ex, ey = fw, fh/2
		case 2:
			ex, ey = fw/2, fh
		default:
			ex, ey = 0, fh/2
		}
		hr := r / 2
		return func(lx, ly float64) (color.RGBA, bool) {
			return fg, (lx-ex)*(lx-ex)+(ly-ey)*(ly-ey) <= hr*hr
		}
	case 2: // inscribed disc
		return func(lx, ly float64) (color.RGBA, bool) {
			dx, dy := lx-fw/2, ly-fh/2
			return fg, dx*dx+dy*dy <= (r/2)*(r/2)
		}
	case 3: // diagonal half
		flip := corner >= 2
		return func(lx, ly float64) (color.RGBA, bool) {
			onSide := lx*fh+ly*fw <= fw*fh
			return fg, onSide != flip
		}
	case 4: // bullseye
		return func(lx, ly float64) (color.RGBA, bool) {
			dx, dy := lx-fw/2, ly-fh/2
			d2 := dx*dx + dy*dy
			if d2 <= (r/4)*(r/4) {
				return inner, true
			}
			return fg, d2 <= (r/2)*(r/2)
		}
	default: // solid cell
		return func(lx, ly float64) (color.RGBA, bool) {
			return fg, false
		}
	}
}
