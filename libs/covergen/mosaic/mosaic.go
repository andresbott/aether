// Package mosaic implements the covergen "mosaic" style: a Voronoi
// stained-glass field of flat cells separated by dark leaded borders.
package mosaic

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/internal/text"
)

// New returns a mosaic style that colors itself from pal; nil uses
// covergen.DefaultPalette().
func New(pal covergen.Palette) covergen.Style {
	if pal == nil {
		pal = covergen.DefaultPalette()
	}
	return style{pal: pal}
}

// Style is the mosaic cover-art style with the default palette.
var Style covergen.Style = New(nil)

type style struct{ pal covergen.Palette }

func (s style) Name() string { return "mosaic" }
func (s style) Knobs() []covergen.Knob {
	out := append([]covergen.Knob(nil), mosaicKnobs...)
	for _, k := range s.pal.Knobs() {
		if d, ok := mosaicPaletteDefaults[k.Name]; ok {
			k.Default = d
		}
		out = append(out, k)
	}
	out = append(out, covergen.GrainKnob(5))
	// mosaic prefers its own defaults for several shared text-overlay knobs (a large
	// display-face title — baked from a lab URL), without affecting the other styles.
	for _, k := range covergen.TextOverlayKnobs() {
		if d, ok := mosaicTextDefaults[k.Name]; ok {
			k.Default = d
		}
		out = append(out, k)
	}
	return out
}

// mosaicPaletteDefaults tunes the injected palette's knob defaults for mosaic. It
// ships on neon (see allstyles.New) — a near-black leading with vivid, widely
// spread cell hues. Keys the injected palette does not declare are ignored.
var mosaicPaletteDefaults = map[string]float64{
	"palette.saturation":       0.85,
	"palette.saturationSpread": 7.4,
	"palette.hueGap":           2.05,
	"palette.glow":             0.4,
}

// mosaicTextDefaults overrides the shared text-overlay knob defaults for mosaic only
// (baked from a lab URL, 2026-09-17). Keys not present keep the shared house-style
// defaults (see covergen.TextOverlayKnobs).
var mosaicTextDefaults = map[string]float64{
	covergen.TextScaleKnobName:            2,
	covergen.TextSizeSpreadKnobName:       0.7,
	covergen.TextOpacitySpreadKnobName:    0,
	covergen.TextRoamKnobName:             0.65,
	covergen.TextTintKnobName:             0.75,
	covergen.TextSaturationKnobName:       0.6,
	covergen.TextSaturationSpreadKnobName: 0.65,
}

// mosaicKnobs tune the stained-glass field. Multipliers/spreads applied after a
// per-seed random draw; mosaic.border is a multiplier on the leading width (0
// removes the borders, leaving bare Voronoi cells).
var mosaicKnobs = []covergen.Knob{
	{Name: "mosaic.cells", Label: "Cell count", Min: 0.5, Max: 3, Step: 0.05, Default: 0.5},
	{Name: "mosaic.cellsSpread", Label: "Cell count spread", Min: 0, Max: 10, Step: 0.05, Default: 0},
	{Name: "mosaic.border", Label: "Leading width", Min: 0, Max: 3, Step: 0.05, Default: 2.85},
}

// maxSites hard-caps the Voronoi site count so the per-pixel nearest-site scan
// stays bounded regardless of extreme (but in-range) knob values.
const maxSites = 160

// Draw scatters Voronoi sites, flat-fills each cell with a palette colour, and
// inks the cell boundaries (where the two nearest sites are nearly equidistant)
// to read as leaded stained glass. The Ink borders and the per-cell colour
// contrast both guarantee visible edges.
func (s style) Draw(img *image.RGBA, rng *rand.Rand, ks covergen.KnobSet) {
	sz := img.Bounds().Dx()
	fs := float64(sz)
	cellsMul := ks.Float("mosaic.cells")
	cellsSpread := ks.Float("mosaic.cellsSpread")
	borderMul := ks.Float("mosaic.border")

	cs := s.pal.Colors(rng, ks)
	// Cells cycle over the four roles with the two accents favoured; Ink is the
	// leading colour, so keeping it rare in the fills stops it reading as noise.
	cellColors := []color.RGBA{cs.Accent1, cs.Accent2, cs.Background, cs.Accent1, cs.Accent2, cs.Ink}

	rc := rng.Float64()
	n := int(math.Round(cellsMul*(16+rc*22) + (cellsSpread-1)*10*(rc-0.5)))
	if n < 6 {
		n = 6
	}
	if n > maxSites {
		n = maxSites
	}

	type site struct {
		x, y float64
		c    color.RGBA
	}
	sites := make([]site, n)
	for i := range sites {
		sites[i] = site{
			x: rng.Float64() * fs,
			y: rng.Float64() * fs,
			c: cellColors[rng.IntN(len(cellColors))],
		}
	}

	// Leading half-width in the (d2-d1) metric: the perpendicular distance to a
	// Voronoi edge is ~(d2-d1)/2, so a pixel is "on the leading" when the gap
	// between its two nearest sites is under this.
	lead := fs * 0.010 * borderMul

	for y := 0; y < sz; y++ {
		fy := float64(y)
		for x := 0; x < sz; x++ {
			fx := float64(x)
			d1, d2 := math.MaxFloat64, math.MaxFloat64
			var nearest color.RGBA
			for i := range sites {
				dx, dy := fx-sites[i].x, fy-sites[i].y
				d := dx*dx + dy*dy
				if d < d1 {
					d2 = d1
					d1, nearest = d, sites[i].c
				} else if d < d2 {
					d2 = d
				}
			}
			c := nearest
			if lead > 0 && math.Sqrt(d2)-math.Sqrt(d1) < lead {
				c = cs.Ink
			}
			img.SetRGBA(x, y, c)
		}
	}
}

// textClasses are the classification(s) mosaic renders its overlay in; the
// pipeline picks a font from their union per seed.
var textClasses = []covergen.FontClass{covergen.FontDisplay}

func (s style) TextClasses() []covergen.FontClass { return textClasses }

// Colors exposes the per-cover ColorSet (see covergen.Colored) so the shared text
// overlay can tint the title in the palette's accent complement.
func (s style) Colors(rng *rand.Rand, ks covergen.KnobSet) covergen.ColorSet {
	return s.pal.Colors(rng, ks)
}

// DrawText paints the album title + subtitle in a clean sans face via the shared
// overlay (see covergen.DrawTextOverlay); mosaicAnchors sets its roam order.
func (s style) DrawText(img *image.RGBA, rng *rand.Rand, ks covergen.KnobSet, t covergen.Text, f covergen.Font, cs covergen.ColorSet) {
	covergen.DrawTextOverlay(img, rng, ks, t, f, cs, mosaicTextFrac, mosaicAnchors)
}

const mosaicTextFrac = 0.085

// mosaicAnchors is mosaic's roam order: index 0 (center) is the placement used at
// text.roam 0; higher roam widens the pool.
var mosaicAnchors = []text.Anchor{
	text.AnchorCenter, text.AnchorLowerCenter, text.AnchorLowerLeft,
	text.AnchorUpperLeft, text.AnchorLowerRight, text.AnchorUpperRight,
}
