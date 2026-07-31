package covergen

import (
	"image"
	"image/color"
	"math/rand/v2"
)

// drawBauhaus tiles the canvas 2x2 or 3x3 and fills each cell with a bold
// geometric motif (quarter disc, half disc, disc, bullseye, diagonal) using a
// cream / ink / two-accent poster palette.
func drawBauhaus(img *image.RGBA, rng *rand.Rand) {
	sz := img.Bounds().Dx()
	baseHue := rng.Float64() * 360

	cream := hsl(baseHue, 0.25+rng.Float64()*0.15, 0.90)
	ink := hsl(baseHue+rng.Float64()*40-20, 0.30, 0.13)
	acc1 := hsl(baseHue, 0.75+rng.Float64()*0.2, 0.50)
	off := []float64{150, 180, 210, 120}[rng.IntN(4)]
	acc2 := hsl(baseHue+off, 0.70+rng.Float64()*0.2, 0.55)
	pal := []color.RGBA{cream, ink, acc1, acc2}

	cells := 2 + rng.IntN(2)
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
