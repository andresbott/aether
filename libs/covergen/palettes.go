package covergen

import (
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen/internal/paint"
)

// This file holds the built-in Palette implementations beyond the default
// harmony palette (see palette.go). Each produces the four ColorSet roles
// deterministically per seed and declares palette.*-prefixed knobs so the lab
// groups them under its palette picker. They deliberately do NOT reuse harmony's
// palette.hue / palette.hueSpread knob names: classic overrides palette.hue to 0
// for its muted look, which would collapse a triad or analogous spread, so the
// hue-shaping knobs here have their own names. palette.saturation is shared, so
// classic's mild saturation mute applies uniformly.
//
// Register new palettes in Palettes() (palette.go) to expose them in the lab.

// monoPalette is a single-hue palette: both accents are tonal variations of one
// hue, with a Background/Ink pair on that same hue pinned to opposite lightness
// ends. Cohesive and moody.
type monoPalette struct{}

var monoKnobs = []Knob{
	{Name: "palette.saturation", Label: "Saturation", Min: 0, Max: 1.5, Step: 0.05, Default: 1},
	{Name: "palette.saturationSpread", Label: "Saturation spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "palette.toneSpread", Label: "Tone spread", Min: 0, Max: 2, Step: 0.05, Default: 1},
}

func (monoPalette) Name() string  { return "mono" }
func (monoPalette) Knobs() []Knob { return monoKnobs }

func (monoPalette) Colors(rng *rand.Rand, ks KnobSet) ColorSet {
	satMul := ks.Float("palette.saturation")
	satSpread := ks.Float("palette.saturationSpread")
	tone := ks.Float("palette.toneSpread")

	hue := rng.Float64() * 360
	sat := func(r float64) float64 {
		return paint.ClampFloat(satMul*0.6+(satSpread-1)*0.25*(r-0.5), 0, 1)
	}

	// Two accents on the same hue, one lighter and one darker; the gap grows with
	// the tone knob.
	gap := 0.16 * tone
	s1 := sat(rng.Float64())
	a1 := paint.Hsl(hue, s1, paint.ClampFloat(0.55+gap, 0.15, 0.90))
	s2 := sat(rng.Float64())
	a2 := paint.Hsl(hue, s2, paint.ClampFloat(0.45-gap, 0.10, 0.85))

	rbg := rng.Float64()
	bgLight := 0.18 + rbg*0.62 // 0.18..0.80
	bgSat := paint.ClampFloat(satMul*0.25+(satSpread-1)*0.06*(rng.Float64()-0.5), 0, 1)
	background := paint.Hsl(hue, bgSat, bgLight)
	inkLight := 0.08
	if bgLight < 0.5 {
		inkLight = 0.94
	}
	ink := paint.Hsl(hue, 0.16, inkLight)

	return ColorSet{Background: background, Ink: ink, Accent1: a1, Accent2: a2}
}

// triadicPalette uses three hues 120 degrees apart: two vivid accents plus a
// coloured (non-neutral) Background drawn from the third hue, and a near-neutral
// Ink at the opposite lightness end. Bold and balanced.
type triadicPalette struct{}

var triadicKnobs = []Knob{
	{Name: "palette.saturation", Label: "Saturation", Min: 0, Max: 1.5, Step: 0.05, Default: 1},
	{Name: "palette.saturationSpread", Label: "Saturation spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "palette.spread", Label: "Triad spread", Min: 0, Max: 2, Step: 0.05, Default: 1},
	{Name: "palette.spreadJitter", Label: "Triad spread jitter", Min: 0, Max: 10, Step: 0.05, Default: 0},
}

func (triadicPalette) Name() string  { return "triadic" }
func (triadicPalette) Knobs() []Knob { return triadicKnobs }

func (triadicPalette) Colors(rng *rand.Rand, ks KnobSet) ColorSet {
	satMul := ks.Float("palette.saturation")
	satSpread := ks.Float("palette.saturationSpread")
	spreadMul := paint.HueMultiplier(rng, ks.Float("palette.spread"), ks.Float("palette.spreadJitter"))

	base := rng.Float64() * 360
	h0 := base
	h1 := base + 120*spreadMul
	h2 := base + 240*spreadMul

	sat := func(r float64) float64 {
		return paint.ClampFloat(satMul*(0.65+r*0.30)+(satSpread-1)*0.30*(r-0.5), 0, 1)
	}
	s1, l1 := sat(rng.Float64()), 0.45+rng.Float64()*0.12
	a1 := paint.Hsl(h0, s1, l1)
	s2, l2 := sat(rng.Float64()), 0.45+rng.Float64()*0.12
	a2 := paint.Hsl(h1, s2, l2)

	rbg := rng.Float64()
	bgLight := 0.20 + rbg*0.55 // 0.20..0.75
	bgSat := paint.ClampFloat(satMul*0.45+(satSpread-1)*0.10*(rng.Float64()-0.5), 0, 1)
	background := paint.Hsl(h2, bgSat, bgLight)
	inkLight := 0.08
	if bgLight < 0.5 {
		inkLight = 0.94
	}
	ink := paint.Hsl(h2, 0.14, inkLight)

	return ColorSet{Background: background, Ink: ink, Accent1: a1, Accent2: a2}
}

// pastelPalette produces soft, low-saturation, high-lightness accents on a light
// Background with a soft-dark Ink. Gentle and airy.
type pastelPalette struct{}

var pastelKnobs = []Knob{
	{Name: "palette.saturation", Label: "Saturation", Min: 0, Max: 1.5, Step: 0.05, Default: 1},
	{Name: "palette.saturationSpread", Label: "Saturation spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "palette.hueGap", Label: "Accent hue gap", Min: 0, Max: 3, Step: 0.05, Default: 1},
	{Name: "palette.softness", Label: "Softness", Min: 0.3, Max: 1.5, Step: 0.05, Default: 1},
}

func (pastelPalette) Name() string  { return "pastel" }
func (pastelPalette) Knobs() []Knob { return pastelKnobs }

func (pastelPalette) Colors(rng *rand.Rand, ks KnobSet) ColorSet {
	satMul := ks.Float("palette.saturation")
	satSpread := ks.Float("palette.saturationSpread")
	hueGap := ks.Float("palette.hueGap")
	soft := ks.Float("palette.softness")
	if soft < 0.3 {
		soft = 0.3
	}

	base := rng.Float64() * 360
	sat := func(r float64) float64 {
		// Lower saturation as softness rises; still tunable up via palette.saturation.
		return paint.ClampFloat(satMul*0.42/soft+(satSpread-1)*0.20*(r-0.5), 0, 1)
	}
	light := func(r float64) float64 {
		return paint.ClampFloat(0.78+(soft-1)*0.06+r*0.06, 0.60, 0.95)
	}
	s1, l1 := sat(rng.Float64()), light(rng.Float64())
	a1 := paint.Hsl(base, s1, l1)
	s2, l2 := sat(rng.Float64()), light(rng.Float64())
	a2 := paint.Hsl(base+40*hueGap, s2, l2)

	background := paint.Hsl(base, paint.ClampFloat(satMul*0.12, 0, 1), paint.ClampFloat(0.90+(soft-1)*0.03, 0.82, 0.96))
	ink := paint.Hsl(base, 0.18, 0.24)

	return ColorSet{Background: background, Ink: ink, Accent1: a1, Accent2: a2}
}

// neonPalette produces highly saturated, bright accents on a near-black
// Background with a light Ink. Punchy and high-energy.
type neonPalette struct{}

var neonKnobs = []Knob{
	{Name: "palette.saturation", Label: "Saturation", Min: 0, Max: 1.5, Step: 0.05, Default: 1},
	{Name: "palette.saturationSpread", Label: "Saturation spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "palette.hueGap", Label: "Accent hue gap", Min: 0, Max: 3, Step: 0.05, Default: 1},
	{Name: "palette.glow", Label: "Glow", Min: 0.2, Max: 1, Step: 0.05, Default: 0.6},
}

func (neonPalette) Name() string  { return "neon" }
func (neonPalette) Knobs() []Knob { return neonKnobs }

func (neonPalette) Colors(rng *rand.Rand, ks KnobSet) ColorSet {
	satMul := ks.Float("palette.saturation")
	satSpread := ks.Float("palette.saturationSpread")
	hueGap := ks.Float("palette.hueGap")
	glow := ks.Float("palette.glow")

	base := rng.Float64() * 360
	sat := func(r float64) float64 {
		return paint.ClampFloat(satMul*0.95+(satSpread-1)*0.10*(r-0.5), 0.30, 1)
	}
	s1, l1 := sat(rng.Float64()), paint.ClampFloat(glow+rng.Float64()*0.08, 0.25, 0.85)
	a1 := paint.Hsl(base, s1, l1)
	s2, l2 := sat(rng.Float64()), paint.ClampFloat(glow+rng.Float64()*0.08, 0.25, 0.85)
	a2 := paint.Hsl(base+150*hueGap, s2, l2)

	bgSat := paint.ClampFloat(satMul*0.40, 0, 1)
	bgLight := paint.ClampFloat(0.07+rng.Float64()*0.04, 0.04, 0.14)
	background := paint.Hsl(base, bgSat, bgLight)
	ink := paint.Hsl(base, 0.10, 0.90)

	return ColorSet{Background: background, Ink: ink, Accent1: a1, Accent2: a2}
}
