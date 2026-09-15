// Package poster implements the covergen "poster" style: a duotone split
// with an oversized disc motif.
package poster

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/internal/paint"
)

// New returns a poster style that colors itself from pal; nil uses
// covergen.DefaultPalette().
func New(pal covergen.Palette) covergen.Style {
	if pal == nil {
		pal = covergen.DefaultPalette()
	}
	return style{pal: pal}
}

// Style is the poster cover-art style with the default palette.
var Style covergen.Style = New(nil)

type style struct{ pal covergen.Palette }

func (s style) Name() string { return "poster" }
func (s style) Knobs() []covergen.Knob {
	out := append([]covergen.Knob(nil), posterKnobs...)
	for _, k := range s.pal.Knobs() {
		if d, ok := posterPaletteDefaults[k.Name]; ok {
			k.Default = d
		}
		out = append(out, k)
	}
	return append(out, covergen.GrainKnob(5))
}

// posterPaletteDefaults tunes the injected palette's knob defaults for poster. It
// ships on pastel (see allstyles.New): lifted, wider-varying saturation and a
// firmer (less soft) accent so the duotone reads boldly. Keys the injected
// palette does not declare are ignored.
var posterPaletteDefaults = map[string]float64{
	"palette.saturation":       1.2,
	"palette.saturationSpread": 3.1,
	"palette.softness":         0.45,
}

// posterKnobs tune the duotone poster. Applied after the per-seed random draws.
// Defaults below are a hand-tuned shipped look (not all identity).
var posterKnobs = []covergen.Knob{
	{Name: "poster.disc", Label: "Disc size", Min: 0.3, Max: 2, Step: 0.05, Default: 0.8},
	{Name: "poster.discSpread", Label: "Disc size spread", Min: 0, Max: 10, Step: 0.05, Default: 3},
	{Name: "poster.stripes", Label: "Stripe count", Min: 0, Max: 3, Step: 0.5, Default: 1},
	{Name: "poster.stripesSpread", Label: "Stripe count spread", Min: 0, Max: 10, Step: 0.05, Default: 3},
}

// Draw splits the canvas with a bold duotone divide, then composes a
// varying motif over it: solid disc, donut, disc two-toned by the divide, an
// edge half-disc, or a disc with a satellite dot — plus 1..3 accent stripes
// parallel to the split and an occasional small punctuation dot.
func (s style) Draw(img *image.RGBA, rng *rand.Rand, ks covergen.KnobSet) {
	sz := img.Bounds().Dx()
	fs := float64(sz)

	cs := s.pal.Colors(rng, ks)
	c1 := cs.Background
	c2 := cs.Ink
	white := color.RGBA{R: 245, G: 242, B: 235, A: 255}
	ink := cs.Ink

	theta := rng.Float64() * math.Pi
	nx, ny := math.Cos(theta), math.Sin(theta)
	px := fs * (0.35 + rng.Float64()*0.30)
	py := fs * (0.35 + rng.Float64()*0.30)
	div := func(x, y float64) float64 { return (x-px)*nx + (y-py)*ny }

	// Base duotone.
	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			c := c1
			if div(float64(x), float64(y)) >= 0 {
				c = c2
			}
			img.SetRGBA(x, y, c)
		}
	}

	drawPosterMotif(img, rng, div, theta, cs.Accent1, white, ink, ks)
	drawPosterStripes(img, rng, div, cs.Accent2, white, ks)

	// Occasional small punctuation dot in a quiet corner.
	if rng.IntN(10) < 3 {
		dx := fs * (0.10 + rng.Float64()*0.15)
		dy := fs * (0.10 + rng.Float64()*0.15)
		if rng.IntN(2) == 0 {
			dx = fs - dx
		}
		if rng.IntN(2) == 0 {
			dy = fs - dy
		}
		dr := fs * (0.02 + rng.Float64()*0.02)
		for y := 0; y < sz; y++ {
			for x := 0; x < sz; x++ {
				fx, fy := float64(x)-dx, float64(y)-dy
				if fx*fx+fy*fy <= dr*dr {
					img.SetRGBA(x, y, white)
				}
			}
		}
	}
}

func inCircle(x, y, cx, cy, r float64) bool {
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= r*r
}

// drawPosterMotif paints the main disc motif over the duotone base: solid
// disc, donut, divide-split disc, edge half-disc, or disc with satellite.
func drawPosterMotif(img *image.RGBA, rng *rand.Rand, div func(x, y float64) float64, theta float64, c3, white, ink color.RGBA, ks covergen.KnobSet) {
	sz := img.Bounds().Dx()
	fs := float64(sz)

	motif := rng.IntN(5)
	ccx := fs * (0.25 + rng.Float64()*0.50)
	ccy := fs * (0.25 + rng.Float64()*0.50)
	rdisc := rng.Float64()
	cr := fs * (ks.Float("poster.disc")*(0.20+rdisc*0.18) + (ks.Float("poster.discSpread")-1)*0.18*(rdisc-0.5))
	discAlt := white
	if rng.IntN(2) == 0 {
		discAlt = ink
	}
	edgeBottom := rng.IntN(2) == 0

	var pick func(fx, fy float64) (color.RGBA, bool)
	switch motif {
	case 0: // solid disc
		pick = func(fx, fy float64) (color.RGBA, bool) {
			return c3, inCircle(fx, fy, ccx, ccy, cr)
		}
	case 1: // donut
		pick = func(fx, fy float64) (color.RGBA, bool) {
			return c3, inCircle(fx, fy, ccx, ccy, cr) && !inCircle(fx, fy, ccx, ccy, cr*0.55)
		}
	case 2: // disc two-toned by the divide
		pick = func(fx, fy float64) (color.RGBA, bool) {
			if !inCircle(fx, fy, ccx, ccy, cr) {
				return c3, false
			}
			if div(fx, fy) >= 0 {
				return c3, true
			}
			return discAlt, true
		}
	case 3: // half-disc rising from a canvas edge
		ecx, ecy := ccx, fs
		if !edgeBottom {
			ecx, ecy = fs, ccy
		}
		pick = func(fx, fy float64) (color.RGBA, bool) {
			return c3, inCircle(fx, fy, ecx, ecy, cr*1.4)
		}
	default: // disc plus satellite dot
		sx := ccx + cr*1.6*math.Cos(theta+1.2)
		sy := ccy + cr*1.6*math.Sin(theta+1.2)
		pick = func(fx, fy float64) (color.RGBA, bool) {
			if inCircle(fx, fy, sx, sy, cr*0.28) {
				return discAlt, true
			}
			return c3, inCircle(fx, fy, ccx, ccy, cr)
		}
	}

	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			if c, ok := pick(float64(x), float64(y)); ok {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

// drawPosterStripes rules 1..3 thin accent stripes parallel to the divide.
func drawPosterStripes(img *image.RGBA, rng *rand.Rand, div func(x, y float64) float64, c3, white color.RGBA, ks covergen.KnobSet) {
	sz := img.Bounds().Dx()
	fs := float64(sz)

	rstripe := rng.IntN(3)
	nStripes := int(ks.Float("poster.stripes")*float64(1+rstripe) + (ks.Float("poster.stripesSpread")-1)*float64(rstripe-1))
	if nStripes < 0 {
		nStripes = 0
	}
	stripeC := white
	if rng.IntN(3) == 0 {
		stripeC = c3
	}
	baseOff := fs * (0.06 + rng.Float64()*0.14)
	if rng.IntN(2) == 0 {
		baseOff = -baseOff
	}
	gap := fs * (0.035 + rng.Float64()*0.03)
	stripeW := fs * (0.010 + rng.Float64()*0.018)
	for y := 0; y < sz; y++ {
		for x := 0; x < sz; x++ {
			d := div(float64(x), float64(y))
			for s := 0; s < nStripes; s++ {
				off := baseOff + float64(s)*gap*paint.Sign(baseOff)
				if math.Abs(d-off) <= stripeW {
					img.SetRGBA(x, y, stripeC)
					break
				}
			}
		}
	}
}
