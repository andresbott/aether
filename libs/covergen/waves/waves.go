// Package waves implements the covergen "waves" style: a synthwave sunset
// of layered sine hills.
package waves

import (
	"image"
	"math"
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/internal/paint"
	"github.com/andresbott/aether/libs/covergen/internal/text"
)

// New returns a waves style that colors itself from pal; nil uses
// covergen.DefaultPalette().
func New(pal covergen.Palette) covergen.Style {
	if pal == nil {
		pal = covergen.DefaultPalette()
	}
	return style{pal: pal}
}

// Style is the waves cover-art style with the default palette.
var Style covergen.Style = New(nil)

type style struct{ pal covergen.Palette }

func (s style) Name() string { return "waves" }
func (s style) Knobs() []covergen.Knob {
	out := append([]covergen.Knob(nil), wavesKnobs...)
	for _, k := range s.pal.Knobs() {
		if d, ok := wavesPaletteDefaults[k.Name]; ok {
			k.Default = d
		}
		out = append(out, k)
	}
	return append(append(out, covergen.GrainKnob(8)), covergen.TextKnobs()...)
}

// wavesPaletteDefaults tunes the injected palette's knob defaults for waves. It
// ships on triadic (see allstyles.New): softer, wider-varying saturation and a
// collapsed triad spread (the three hues sit near the base, jittered per seed)
// for a tonal synthwave sky. Keys the injected palette does not declare are
// ignored.
var wavesPaletteDefaults = map[string]float64{
	"palette.saturation":       0.8,
	"palette.saturationSpread": 2.1,
	"palette.spread":           0,
	"palette.spreadJitter":     0.65,
}

// wavesKnobs tune the synthwave poster. Most are multipliers/spreads applied
// after a per-seed random draw; the defaults are a hand-tuned look (not the
// identity), so the waves goldens reflect these values.
var wavesKnobs = []covergen.Knob{
	{Name: "waves.amp", Label: "Wave height", Min: 0, Max: 3, Step: 0.05, Default: 0.05},
	{Name: "waves.ampSpread", Label: "Wave height spread", Min: 0, Max: 10, Step: 0.05, Default: 5.65},
	{Name: "waves.freq", Label: "Wave frequency", Min: 0.2, Max: 3, Step: 0.05, Default: 0.25},
	{Name: "waves.freqSpread", Label: "Wave frequency spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "waves.layers", Label: "Layer count", Min: 0.5, Max: 2.5, Step: 0.05, Default: 2.35},
	{Name: "waves.layersSpread", Label: "Layer count spread", Min: 0, Max: 10, Step: 0.05, Default: 4.65},
	{Name: "waves.center", Label: "Wave center", Min: -0.3, Max: 0.3, Step: 0.02, Default: -0.06},
	{Name: "waves.centerSpread", Label: "Wave center spread", Min: 0, Max: 0.3, Step: 0.02, Default: 0.3},
}

// Draw paints a synthwave-style poster: a vertical sky ramp glowing at
// the horizon, an optional flat sun disc (sometimes with scanline cuts), and
// opaque wave layers that darken and grow as they approach the viewer.
func (s style) Draw(img *image.RGBA, rng *rand.Rand, ks covergen.KnobSet) {
	sz := img.Bounds().Dx()
	fs := float64(sz)
	amp := ks.Float("waves.amp")
	ampSpread := ks.Float("waves.ampSpread")
	freq := ks.Float("waves.freq")
	freqSpread := ks.Float("waves.freqSpread")
	layerMul := ks.Float("waves.layers")
	layersSpread := ks.Float("waves.layersSpread")
	waveCenter := ks.Float("waves.center")
	waveCenterSpread := ks.Float("waves.centerSpread")

	// Sky: dark at the top, glowing near the horizon.
	cs := s.pal.Colors(rng, ks)
	dark := cs.Background
	if paint.Luminance(cs.Ink) < paint.Luminance(cs.Background) {
		dark = cs.Ink
	}
	skyTop := dark
	glow := cs.Accent1
	centerOff := waveCenter
	if waveCenterSpread > 0 {
		centerOff += waveCenterSpread * (rng.Float64() - 0.5)
	}
	horizonY := fs * (0.40 + rng.Float64()*0.12 + centerOff)
	for y := 0; y < sz; y++ {
		t := float64(y) / horizonY
		if t > 1 {
			t = 1
		}
		c := paint.LerpRGBA(skyTop, glow, t)
		for x := 0; x < sz; x++ {
			img.SetRGBA(x, y, c)
		}
	}

	// Sun disc, flat, near the horizon.
	if rng.IntN(10) < 7 {
		sunR := fs * (0.10 + rng.Float64()*0.15)
		sunX := fs * (0.25 + rng.Float64()*0.50)
		sunY := horizonY - sunR*(rng.Float64()*0.8)
		sun := cs.Accent2
		scanlines := rng.IntN(2) == 0
		for y := 0; y < sz; y++ {
			fy := float64(y) - sunY
			if scanlines && fy > 0 {
				// Cut horizontal slits out of the lower half, wider near
				// the bottom.
				p := fy / sunR // 0..1 down the lower half
				if math.Mod(p*6, 1) < p*0.55 {
					continue
				}
			}
			for x := 0; x < sz; x++ {
				fx := float64(x) - sunX
				if fx*fx+fy*fy <= sunR*sunR {
					img.SetRGBA(x, y, sun)
				}
			}
		}
	}

	rl := rng.IntN(3)
	layers := int(layerMul*float64(4+rl) + (layersSpread-1)*float64(rl-1))
	if layers < 2 {
		layers = 2
	}
	for i := 0; i < layers; i++ {
		front := float64(i) / float64(layers-1) // 0 back .. 1 front
		baseY := fs * (0.44 + 0.48*front + centerOff)
		ramp := rng.Float64()
		amp1 := fs * (amp*(0.025+0.075*front+ramp*0.03) + (ampSpread-1)*0.03*(ramp-0.5))
		amp2 := amp1 * (0.2 + rng.Float64()*0.4)
		rfreq := rng.Float64()
		freq1 := freq*(0.8+rfreq*1.8) + (freqSpread-1)*1.8*(rfreq-0.5)
		freq2 := freq1 * (2 + rng.Float64())
		ph1 := rng.Float64() * 2 * math.Pi
		ph2 := rng.Float64() * 2 * math.Pi

		c := paint.LerpRGBA(cs.Accent1, dark, front)

		for x := 0; x < sz; x++ {
			fx := float64(x) / fs
			y0 := baseY +
				amp1*math.Sin(2*math.Pi*freq1*fx+ph1) +
				amp2*math.Sin(2*math.Pi*freq2*fx+ph2)
			yi := int(y0)
			if yi < 0 {
				yi = 0
			}
			for y := yi; y < sz; y++ {
				img.SetRGBA(x, y, c)
			}
		}
	}
}

// textClass is the classification waves renders its overlay in.
const textClass = covergen.FontScript

func (s style) TextClass() covergen.FontClass { return textClass }

// DrawText paints the album title + subtitle top-left in a script face.
func (s style) DrawText(img *image.RGBA, _ *rand.Rand, ks covergen.KnobSet, t covergen.Text, f covergen.Font) {
	px := float64(img.Bounds().Dx()) * wavesTextFrac * ks.Float(covergen.TextScaleKnobName)
	text.Block(img, t.Main, t.Subtitle, f.Face, px, text.AnchorUpperLeft, int(px*0.6), ks.Float(covergen.TextOpacityKnobName))
}

const wavesTextFrac = 0.090
