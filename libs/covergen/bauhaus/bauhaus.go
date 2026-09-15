// Package bauhaus implements the covergen "bauhaus" style: a poster grid of
// bold geometric motifs in a cream / ink / two-accent palette.
package bauhaus

import (
	"image"
	"image/color"
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen"
)

// New returns a bauhaus style that colors itself from pal; nil uses
// covergen.DefaultPalette().
func New(pal covergen.Palette) covergen.Style {
	if pal == nil {
		pal = covergen.DefaultPalette()
	}
	return style{pal: pal}
}

// Style is the bauhaus cover-art style with the default palette.
var Style covergen.Style = New(nil)

type style struct{ pal covergen.Palette }

func (s style) Name() string { return "bauhaus" }
func (s style) Knobs() []covergen.Knob {
	out := append([]covergen.Knob(nil), bauhausKnobs...)
	for _, k := range s.pal.Knobs() {
		if d, ok := bauhausPaletteDefaults[k.Name]; ok {
			k.Default = d
		}
		out = append(out, k)
	}
	return append(out, covergen.GrainKnob(9))
}

// bauhausPaletteDefaults tunes the injected palette's knob defaults for bauhaus.
// It ships on neon (see allstyles.New): a hotter saturation and a wider accent
// hue gap. Keys the injected palette does not declare are ignored, so the lab
// can still swap in any palette.
var bauhausPaletteDefaults = map[string]float64{
	"palette.saturation": 1.1,
	"palette.hueGap":     1.2,
}

// bauhausKnobs tune the poster grid. Most are multipliers/spreads applied after
// a per-seed random draw; the defaults are a hand-tuned look (not the identity),
// so the bauhaus goldens reflect these values.
var bauhausKnobs = []covergen.Knob{
	{Name: "bauhaus.cells", Label: "Grid density", Min: 0.5, Max: 3, Step: 0.25, Default: 0.75},
	{Name: "bauhaus.cellsSpread", Label: "Grid density spread", Min: 0, Max: 10, Step: 0.05, Default: 2.15},
}

// Draw tiles the canvas 2x2 or 3x3 and fills each cell with a bold
// geometric motif (quarter disc, half disc, disc, bullseye, diagonal) using a
// cream / ink / two-accent poster palette.
func (s style) Draw(img *image.RGBA, rng *rand.Rand, ks covergen.KnobSet) {
	sz := img.Bounds().Dx()
	cellMul := ks.Float("bauhaus.cells")
	cellsSpread := ks.Float("bauhaus.cellsSpread")
	cs := s.pal.Colors(rng, ks)
	pal := []color.RGBA{cs.Background, cs.Ink, cs.Accent1, cs.Accent2}

	rcell := rng.IntN(2)
	cells := int(cellMul*float64(2+rcell) + (cellsSpread-1)*(float64(rcell)-0.5))
	if cells < 1 {
		cells = 1
	}
	cellSize := sz / cells

	for cy := 0; cy < cells; cy++ {
		for cx := 0; cx < cells; cx++ {
			x0, y0 := cx*cellSize, cy*cellSize
			x1, y1 := x0+cellSize, y0+cellSize
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
