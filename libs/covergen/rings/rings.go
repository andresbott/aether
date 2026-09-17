// Package rings implements the covergen "rings" style: eccentric, wobbly
// concentric ring systems.
package rings

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/internal/paint"
	"github.com/andresbott/aether/libs/covergen/internal/text"
)

// New returns a rings style that colors itself from pal; nil uses
// covergen.DefaultPalette().
func New(pal covergen.Palette) covergen.Style {
	if pal == nil {
		pal = covergen.DefaultPalette()
	}
	return style{pal: pal}
}

// Style is the rings cover-art style with the default palette.
var Style covergen.Style = New(nil)

type style struct{ pal covergen.Palette }

func (s style) Name() string { return "rings" }
func (s style) Knobs() []covergen.Knob {
	out := append([]covergen.Knob(nil), ringsKnobs...)
	out = append(out, s.pal.Knobs()...)
	out = append(out, covergen.GrainKnob(6))
	// rings prefers its own defaults for a couple of shared text-overlay knobs (a
	// fully-opaque title with no per-seed opacity jitter — baked from a lab URL),
	// without affecting the other styles.
	for _, k := range covergen.TextOverlayKnobs() {
		if d, ok := ringsTextDefaults[k.Name]; ok {
			k.Default = d
		}
		out = append(out, k)
	}
	return out
}

// ringsTextDefaults overrides the shared text-overlay knob defaults for rings only
// (baked from a lab URL, 2026-09-17). Keys not present keep the shared house-style
// defaults (see covergen.TextOverlayKnobs).
var ringsTextDefaults = map[string]float64{
	covergen.TextOpacityKnobName:       1,
	covergen.TextOpacitySpreadKnobName: 0,
}

// ringsKnobs are the tunable parameters of the rings style. Most are multipliers
// or spreads applied after a per-seed random draw. The defaults are a hand-tuned
// look (not the identity), so the rings goldens reflect these values;
// rings.spacing stays 1.0. rings.minBandWidth is an absolute floor (a fraction
// of the canvas) on the radial band pitch — it keeps bands bold enough to avoid
// a fine moiré.
var ringsKnobs = []covergen.Knob{
	{Name: "rings.spacing", Label: "Ring spacing", Min: 0.3, Max: 3.0, Step: 0.05, Default: 1.0},
	{Name: "rings.spacingSpread", Label: "Ring spacing spread", Min: 0, Max: 10, Step: 0.05, Default: 4.65},
	{Name: "rings.wobble", Label: "Wobble", Min: 0, Max: 3, Step: 0.05, Default: 0.75},
	{Name: "rings.wobbleSpread", Label: "Wobble spread", Min: 0, Max: 10, Step: 0.05, Default: 3.85},
	{Name: "rings.corners", Label: "Corners", Min: 0.2, Max: 4, Step: 0.05, Default: 0.2},
	{Name: "rings.cornersSpread", Label: "Corners spread", Min: 0, Max: 10, Step: 0.05, Default: 3.05},
	{Name: "rings.minBandWidth", Label: "Min band width", Min: 0.01, Max: 0.12, Step: 0.005, Default: 0.015},
	// The title is drawn wrapped in a single thick white border (a 70s "echo" look)
	// via the shared text overlay; textBorderWidth is the border's width as a fraction
	// of the title height.
	{Name: "rings.textBorderWidth", Label: "Text border width", Min: 0.02, Max: 0.6, Step: 0.005, Default: 0.29},
}

// Draw draws eccentric concentric rings with an optional angular wobble
// (lobes / corners). Palettes mix a deep base, a bright partner, and two
// accents cycled over the base. The band pitch is floored at
// rings.minBandWidth so rings stay bold and never collapse into a fine
// moiré.
func (s style) Draw(img *image.RGBA, rng *rand.Rand, ks covergen.KnobSet) {
	sz := img.Bounds().Dx()
	fs := float64(sz)
	// spacing scales the radial band pitch in every variant; 1.0 is the
	// shipped look. Applied after each rng draw so defaults stay identical.
	spacing := ks.Float("rings.spacing")
	spacingSpread := ks.Float("rings.spacingSpread")
	wobble := ks.Float("rings.wobble")
	wobbleSpread := ks.Float("rings.wobbleSpread")
	corners := ks.Float("rings.corners")
	cornersSpread := ks.Float("rings.cornersSpread")

	// minPitch floors the radial band pitch (the gap between successive edges)
	// at rings.minBandWidth of the canvas. spacingSpread widens the per-band
	// draw and can otherwise drive the pitch to zero or negative — collapsing
	// bands into a fine moiré and making the edge table non-monotonic. The floor
	// keeps bands bold and the loop bounded; maxRingBands hard-caps the count.
	minPitch := fs * ks.Float("rings.minBandWidth")
	const maxRingBands = 256

	cs := s.pal.Colors(rng, ks)
	// Deep base: whichever of Background/Ink is darker; the other is the bright partner.
	base, bright := cs.Background, cs.Ink
	if paint.Luminance(cs.Ink) < paint.Luminance(cs.Background) {
		base, bright = cs.Ink, cs.Background
	}
	var pal []color.RGBA
	if rng.IntN(2) == 0 {
		// Calmer: base breathes between accents.
		pal = []color.RGBA{base, cs.Accent1, base, cs.Accent2, base, bright}
	} else {
		// Louder: accents chain with one dark beat.
		pal = []color.RGBA{base, cs.Accent1, cs.Accent2, base, bright, cs.Accent1}
	}

	cx := fs * (0.15 + rng.Float64()*0.70)
	cy := fs * (0.15 + rng.Float64()*0.70)
	maxD := 0.0
	for _, p := range [][2]float64{{0, 0}, {fs, 0}, {0, fs}, {fs, fs}} {
		d := math.Hypot(p[0]-cx, p[1]-cy)
		if d > maxD {
			maxD = d
		}
	}

	// Irregular concentric band table.
	var edges []float64
	e := fs * (0.04 + rng.Float64()*0.10) // inner disc radius
	for e < maxD+fs*0.1 && len(edges) < maxRingBands {
		edges = append(edges, e)
		r := rng.Float64()
		inc := fs * (spacing*(0.035+r*0.085) + (spacingSpread-1)*0.085*(r-0.5))
		e += max(inc, minPitch) // strictly increasing -> monotonic edges, bounded loop
	}
	edges = append(edges, maxD+fs*0.2)

	// Wobble oscillates the ring radius with the angle: corners scales the lobe
	// count, and wobble=0 gives perfectly circular rings.
	rw := rng.Float64()
	wobAmp := fs * (wobble*(0.012+rw*0.030) + (wobbleSpread-1)*0.030*(rw-0.5))
	rf := rng.IntN(7)
	wobFreq := math.Round(corners*float64(3+rf) + (cornersSpread-1)*float64(rf-3)) // lobes / corners
	wobPh := rng.Float64() * 2 * math.Pi

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

// textClasses are the classification(s) rings renders its overlay in; the
// pipeline picks a font from their union per seed.
var textClasses = []covergen.FontClass{covergen.FontClean}

func (s style) TextClasses() []covergen.FontClass { return textClasses }

// Colors exposes the per-cover ColorSet (see covergen.Colored) so the shared text
// overlay can tint the title in the palette's accent complement.
func (s style) Colors(rng *rand.Rand, ks covergen.KnobSet) covergen.ColorSet {
	return s.pal.Colors(rng, ks)
}

// DrawText paints the album title + subtitle in a clean sans face via the shared
// overlay (see covergen.DrawTextOverlay); ringsAnchors sets its roam order. Rings wraps
// each letter in a single thick white border (see covergen.WithOutline) for a retro /
// 70s look; rings.textBorderWidth tunes its thickness.
func (s style) DrawText(img *image.RGBA, rng *rand.Rand, ks covergen.KnobSet, t covergen.Text, f covergen.Font, cs covergen.ColorSet) {
	var opts []covergen.TextOption
	if w := ks.Float("rings.textBorderWidth"); w > 0 {
		opts = append(opts, covergen.WithOutline(w))
	}
	covergen.DrawTextOverlay(img, rng, ks, t, f, cs, ringsTextFrac, ringsAnchors, opts...)
}

const ringsTextFrac = 0.080

// ringsAnchors is rings's roam order: index 0 (lower-center) is the placement used
// at text.roam 0; higher roam widens the pool.
var ringsAnchors = []text.Anchor{
	text.AnchorLowerCenter, text.AnchorLowerLeft, text.AnchorLowerRight,
	text.AnchorCenter, text.AnchorUpperLeft, text.AnchorUpperRight,
}
