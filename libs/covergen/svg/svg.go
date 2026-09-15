package svg

import (
	"embed"
	"image"
	"image/color"
	"image/draw"
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/internal/paint"
)

//go:embed assets/*.svg
var assetFS embed.FS

// Style is the svg cover-art style over the embedded assets/ rotation.
var Style covergen.Style = New(loadEmbedded()...)

// New builds an svg Style over an explicit, ordered set of SVG sources. The lab
// uses it to preview a single candidate through the real render pipeline.
func New(assets ...[]byte) covergen.Style {
	cp := make([][]byte, len(assets))
	copy(cp, assets)
	return style{assets: cp}
}

func loadEmbedded() [][]byte {
	entries, err := assetFS.ReadDir("assets") // sorted by filename
	if err != nil {
		return nil
	}
	var out [][]byte
	for _, e := range entries {
		b, err := assetFS.ReadFile("assets/" + e.Name())
		if err == nil {
			out = append(out, b)
		}
	}
	return out
}

type style struct{ assets [][]byte }

func (style) Name() string { return "svg" }
func (style) Knobs() []covergen.Knob {
	return append(append([]covergen.Knob(nil), svgKnobs...), covergen.GrainKnob(4))
}

var svgKnobs = []covergen.Knob{
	{Name: "svg.scale", Label: "Motif size", Min: 0.2, Max: 1.2, Step: 0.05, Default: 0.7},
	{Name: "svg.scaleSpread", Label: "Motif size spread", Min: 0, Max: 10, Step: 0.05, Default: 3.0},
	{Name: "svg.saturation", Label: "Saturation", Min: 0, Max: 1.5, Step: 0.05, Default: 1.0},
	{Name: "svg.saturationSpread", Label: "Saturation spread", Min: 0, Max: 10, Step: 0.05, Default: 5.0},
	{Name: "svg.hue", Label: "Hue", Min: 0, Max: 3, Step: 0.05, Default: 1.5},
	{Name: "svg.hueSpread", Label: "Hue spread", Min: 0, Max: 10, Step: 0.05, Default: 3.0},
}

func (s style) Draw(img *image.RGBA, rng *rand.Rand, ks covergen.KnobSet) {
	cols := paint.Vivid(rng, 5, ks.Float("svg.saturation"),
		paint.HueMultiplier(rng, ks.Float("svg.hue"), ks.Float("svg.hueSpread")),
		ks.Float("svg.saturationSpread"))
	paintGradient(img, rng, cols[0], cols[1])
	if len(s.assets) == 0 {
		return
	}
	src := s.assets[rng.IntN(len(s.assets))]
	recol := substituteSentinels(src, [3]color.RGBA{cols[2], cols[3], cols[4]})

	sz := img.Bounds().Dx()
	rs := rng.Float64()
	frac := paint.ClampFloat(ks.Float("svg.scale")+ks.Float("svg.scaleSpread")*0.03*(rs-0.5), 0.15, 1.4)
	w := int(float64(sz) * frac)
	if w < 1 {
		w = 1
	}
	motif, err := rasterizeMotif(recol, w, w)
	if err != nil {
		return // background-only fallback; Validate keeps shipped assets safe
	}
	maxOff := sz / 12
	off := func() int {
		if maxOff < 1 {
			return 0
		}
		return rng.IntN(2*maxOff+1) - maxOff
	}
	cx := sz/2 + off()
	cy := sz/2 + off()
	draw.Draw(img, image.Rect(cx-w/2, cy-w/2, cx-w/2+w, cy-w/2+w), motif, image.Point{}, draw.Over)
}
