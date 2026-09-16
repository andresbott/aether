// Package halftone implements the covergen "halftone" style: an overlaid
// two-ink dot screen (newsprint / pop-art), where dot size follows a smooth
// intensity field.
package halftone

import (
	"image"
	"math"
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/internal/paint"
)

// New returns a halftone style that colors itself from pal; nil uses
// covergen.DefaultPalette().
func New(pal covergen.Palette) covergen.Style {
	if pal == nil {
		pal = covergen.DefaultPalette()
	}
	return style{pal: pal}
}

// Style is the halftone cover-art style with the default palette.
var Style covergen.Style = New(nil)

type style struct{ pal covergen.Palette }

func (s style) Name() string { return "halftone" }
func (s style) Knobs() []covergen.Knob {
	out := append([]covergen.Knob(nil), halftoneKnobs...)
	for _, k := range s.pal.Knobs() {
		if d, ok := halftonePaletteDefaults[k.Name]; ok {
			k.Default = d
		}
		out = append(out, k)
	}
	return append(out, covergen.GrainKnob(8))
}

// halftonePaletteDefaults tunes the injected palette's knob defaults for halftone.
// It ships on mono (see allstyles.New): a single-hue newsprint with fully lifted,
// widely spread saturation so the two plates read boldly. Keys the injected
// palette does not declare are ignored.
var halftonePaletteDefaults = map[string]float64{
	"palette.saturation":       1.5,
	"palette.saturationSpread": 5.2,
	"palette.toneSpread":       1.35,
}

// halftoneKnobs tune the dot screen. Multipliers/spreads applied after a per-seed
// random draw: pitch sets dot coarseness, contrast scales how fast dots grow with
// the intensity field (darker/denser at higher contrast).
var halftoneKnobs = []covergen.Knob{
	{Name: "halftone.pitch", Label: "Dot pitch", Min: 0.3, Max: 2.5, Step: 0.05, Default: 2.5},
	{Name: "halftone.pitchSpread", Label: "Dot pitch spread", Min: 0, Max: 10, Step: 0.05, Default: 2.3},
	{Name: "halftone.contrast", Label: "Dot contrast", Min: 0.3, Max: 2.5, Step: 0.05, Default: 0.85},
}

// screen is one rotated dot grid: a pitch, a rotation (cos/sin), the intensity
// field that drives dot size, and a fill cap letting dots merge in dark areas.
type screen struct {
	cos, sin float64
	pitch    float64
	contrast float64
	maxFill  float64
	field    func(fx, fy float64) float64
}

// covers reports whether image point (fx, fy) falls inside this screen's dot: it
// rotates the point into screen space, finds the nearest grid centre, samples the
// intensity field there, and tests the point against the resulting dot radius.
func (sc screen) covers(fx, fy float64) bool {
	sx := fx*sc.cos + fy*sc.sin
	sy := -fx*sc.sin + fy*sc.cos
	ci := math.Round(sx / sc.pitch)
	cj := math.Round(sy / sc.pitch)
	cxs, cys := ci*sc.pitch, cj*sc.pitch
	// Nearest centre back to image space (inverse rotation) to sample the field.
	cxi := cxs*sc.cos - cys*sc.sin
	cyi := cxs*sc.sin + cys*sc.cos
	g := sc.field(cxi, cyi) * sc.contrast
	if g <= 0 {
		return false
	}
	if g > 1 {
		g = 1
	}
	r := sc.pitch * sc.maxFill * math.Sqrt(g)
	dx, dy := sx-cxs, sy-cys
	return dx*dx+dy*dy <= r*r
}

// makeField builds a per-seed intensity field in [0,1]: either a rippled linear
// ramp or a radial falloff, optionally inverted.
func makeField(rng *rand.Rand, fs float64) func(fx, fy float64) float64 {
	inv := rng.IntN(2) == 0
	if rng.IntN(2) == 0 {
		ang := rng.Float64() * 2 * math.Pi
		dx, dy := math.Cos(ang), math.Sin(ang)
		rfreq := 0.6 + rng.Float64()*1.4
		ramp := rng.Float64() * 0.18
		rph := rng.Float64() * 2 * math.Pi
		return func(fx, fy float64) float64 {
			t := ((fx*dx+fy*dy)/fs + 1) / 2
			t += ramp * math.Sin(rfreq*2*math.Pi*(fx-fy)/fs+rph)
			if inv {
				t = 1 - t
			}
			return paint.ClampFloat(t, 0, 1)
		}
	}
	cx := fs * (0.2 + rng.Float64()*0.6)
	cy := fs * (0.2 + rng.Float64()*0.6)
	maxR := fs * (0.45 + rng.Float64()*0.45)
	return func(fx, fy float64) float64 {
		d := math.Hypot(fx-cx, fy-cy) / maxR
		t := 1 - d
		if inv {
			t = d
		}
		return paint.ClampFloat(t, 0, 1)
	}
}

// Draw fills the canvas with paper (the lighter of Background/Ink), then lays two
// dot screens over it at offset angles — a bright Accent1 plate and a darker
// Accent2-tinted plate — so dark field regions read as merged two-ink rosettes and
// light regions as bare paper.
func (s style) Draw(img *image.RGBA, rng *rand.Rand, ks covergen.KnobSet) {
	sz := img.Bounds().Dx()
	fs := float64(sz)
	pitchMul := ks.Float("halftone.pitch")
	pitchSpread := ks.Float("halftone.pitchSpread")
	contrast := ks.Float("halftone.contrast")

	cs := s.pal.Colors(rng, ks)
	paper, dark := cs.Background, cs.Ink
	if paint.Luminance(dark) > paint.Luminance(paper) {
		paper, dark = dark, paper
	}
	// Two ink plates: a bright Accent1 screen and a darker Accent2-tinted screen
	// (structural, newsprint-like), so both accents — and thus every palette knob —
	// drive the output.
	colorA := cs.Accent1
	colorB := paint.LerpRGBA(cs.Accent2, dark, 0.45)

	rp := rng.Float64()
	pitch := fs * (pitchMul*(0.035+rp*0.030) + (pitchSpread-1)*0.030*(rp-0.5))
	if pitch < fs*0.018 {
		pitch = fs * 0.018 // floor: keep dots resolvable and bounded
	}

	base := rng.Float64() * math.Pi
	const sep = 30 * math.Pi / 180 // classic screen-angle separation
	scrA := screen{
		cos: math.Cos(base), sin: math.Sin(base),
		pitch: pitch, contrast: contrast * (0.85 + rng.Float64()*0.3),
		maxFill: 0.70, field: makeField(rng, fs),
	}
	scrB := screen{
		cos: math.Cos(base + sep), sin: math.Sin(base + sep),
		pitch: pitch * (0.9 + rng.Float64()*0.2), contrast: contrast,
		maxFill: 0.72, field: makeField(rng, fs),
	}

	for y := 0; y < sz; y++ {
		fy := float64(y)
		for x := 0; x < sz; x++ {
			fx := float64(x)
			c := paper
			if scrA.covers(fx, fy) {
				c = colorA
			}
			if scrB.covers(fx, fy) {
				c = colorB
			}
			img.SetRGBA(x, y, c)
		}
	}
}
