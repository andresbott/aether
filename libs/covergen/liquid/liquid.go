// Package liquid implements the covergen "liquid" style: merging metaball blobs
// whose summed field is thresholded into flat colour bands (a lava-lamp look).
package liquid

import (
	"image"
	"image/color"
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen"
)

// New returns a liquid style that colors itself from pal; nil uses
// covergen.DefaultPalette().
func New(pal covergen.Palette) covergen.Style {
	if pal == nil {
		pal = covergen.DefaultPalette()
	}
	return style{pal: pal}
}

// Style is the liquid cover-art style with the default palette.
var Style covergen.Style = New(nil)

type style struct{ pal covergen.Palette }

func (s style) Name() string { return "liquid" }
func (s style) Knobs() []covergen.Knob {
	out := append([]covergen.Knob(nil), liquidKnobs...)
	for _, k := range s.pal.Knobs() {
		if d, ok := liquidPaletteDefaults[k.Name]; ok {
			k.Default = d
		}
		out = append(out, k)
	}
	return append(out, covergen.GrainKnob(2))
}

// liquidPaletteDefaults tunes the injected palette's knob defaults for liquid. It
// ships on triadic (see allstyles.New): a coloured field with two vivid,
// triad-separated blob colours (widened per-seed by the triad spread jitter). Keys
// the injected palette does not declare are ignored.
var liquidPaletteDefaults = map[string]float64{
	"palette.saturation":       1,
	"palette.saturationSpread": 1.1,
	"palette.spread":           1.25,
	"palette.spreadJitter":     5.85,
}

// liquidKnobs tune the metaball field. Multipliers/spreads applied after a
// per-seed random draw: blobs sets how many merge, threshold scales the field
// level at which a blob's edge falls (higher = tighter blobs, more background).
var liquidKnobs = []covergen.Knob{
	{Name: "liquid.blobs", Label: "Blob count", Min: 0.5, Max: 3, Step: 0.05, Default: 2.1},
	{Name: "liquid.blobsSpread", Label: "Blob count spread", Min: 0, Max: 10, Step: 0.05, Default: 7.35},
	{Name: "liquid.threshold", Label: "Blob tightness", Min: 0.3, Max: 3, Step: 0.05, Default: 0.85},
}

// maxBlobs hard-caps the metaball count so the per-pixel field sum stays bounded
// under extreme (but in-range) knob values.
const maxBlobs = 14

// coreLevel is the field multiple (over the edge threshold) at which the inner
// core band begins: a blob's Accent2 body runs from its edge up to this, and the
// hotter Accent1 core fills the rest.
const coreLevel = 4.0

// Draw scatters metaball blobs, sums their radial fields, and thresholds the sum
// into three flat bands — Background field, Accent2 body, Accent1 core — so the
// merged silhouettes read as crisp-edged liquid rather than a soft gradient.
func (s style) Draw(img *image.RGBA, rng *rand.Rand, ks covergen.KnobSet) {
	sz := img.Bounds().Dx()
	fs := float64(sz)
	blobsMul := ks.Float("liquid.blobs")
	blobsSpread := ks.Float("liquid.blobsSpread")
	threshold := ks.Float("liquid.threshold")

	cs := s.pal.Colors(rng, ks)

	rc := rng.Float64()
	n := int(blobsMul*(3+rc*3) + (blobsSpread-1)*2*(rc-0.5) + 0.5)
	if n < 2 {
		n = 2
	}
	if n > maxBlobs {
		n = maxBlobs
	}

	type blob struct {
		x, y, w float64 // centre and weight (radius^2)
	}
	blobs := make([]blob, n)
	for i := range blobs {
		r := fs * (0.12 + rng.Float64()*0.16)
		blobs[i] = blob{
			x: fs * (0.15 + rng.Float64()*0.70),
			y: fs * (0.15 + rng.Float64()*0.70),
			w: r * r,
		}
	}
	// eps keeps the field finite at a blob centre; small relative to blob area so
	// centres always exceed coreLevel and every blob has a visible core.
	eps := (fs * 0.02) * (fs * 0.02)

	// t1 is the edge level (field == weight/radius^2 == 1 at an isolated blob's
	// rim), scaled by the threshold knob; t2 begins the core band.
	t1 := threshold
	t2 := threshold * coreLevel

	for y := 0; y < sz; y++ {
		fy := float64(y)
		for x := 0; x < sz; x++ {
			fx := float64(x)
			var f float64
			for i := range blobs {
				dx, dy := fx-blobs[i].x, fy-blobs[i].y
				f += blobs[i].w / (dx*dx + dy*dy + eps)
			}
			var c color.RGBA
			switch {
			case f >= t2:
				c = cs.Accent1
			case f >= t1:
				c = cs.Accent2
			default:
				c = cs.Background
			}
			img.SetRGBA(x, y, c)
		}
	}
}
