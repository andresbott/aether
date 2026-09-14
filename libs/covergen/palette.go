package covergen

import (
	"image/color"
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen/internal/paint"
)

// ColorSet is the four named colors every palette provides for one cover.
type ColorSet struct {
	Background color.RGBA // dominant field / surface
	Ink        color.RGBA // high-contrast partner to Background (detail, marks)
	Accent1    color.RGBA // primary saturated pop
	Accent2    color.RGBA // secondary saturated pop, harmonious with Accent1
}

// Palette deterministically produces a ColorSet for one cover and declares the
// knobs that tune it.
type Palette interface {
	Knobs() []Knob
	Colors(rng *rand.Rand, ks KnobSet) ColorSet
}

// paletteKnobs tune the default HarmonyPalette. saturation/hue are multipliers
// (identity 1); the *Spread knobs widen the per-seed draw (identity 1 for
// saturation, 0 for hue so hue is deterministic at default).
var paletteKnobs = []Knob{
	{Name: "palette.saturation", Label: "Saturation", Min: 0, Max: 1.5, Step: 0.05, Default: 1},
	{Name: "palette.saturationSpread", Label: "Saturation spread", Min: 0, Max: 10, Step: 0.05, Default: 1},
	{Name: "palette.hue", Label: "Hue", Min: 0, Max: 3, Step: 0.05, Default: 1},
	{Name: "palette.hueSpread", Label: "Hue spread", Min: 0, Max: 10, Step: 0.05, Default: 0},
}

// harmonyPalette is the default palette: two vivid harmony-scheme accents plus a
// Background/Ink pair pinned to opposite lightness ends for guaranteed contrast.
type harmonyPalette struct{}

// DefaultPalette returns the built-in harmony palette.
func DefaultPalette() Palette { return harmonyPalette{} }

func (harmonyPalette) Knobs() []Knob { return paletteKnobs }

func (harmonyPalette) Colors(rng *rand.Rand, ks KnobSet) ColorSet {
	satMul := ks.Float("palette.saturation")
	satSpread := ks.Float("palette.saturationSpread")
	hueMul := paint.HueMultiplier(rng, ks.Float("palette.hue"), ks.Float("palette.hueSpread"))

	// Two vivid accents from a harmony scheme (complementary/triadic/...).
	accents := paint.Vivid(rng, 2, satMul, hueMul, satSpread)

	// Background: per-seed lightness centre. Ink: opposite lightness end,
	// near-neutral, so the pair always contrasts.
	baseHue := rng.Float64() * 360
	rbg := rng.Float64()
	bgLight := 0.18 + rbg*0.62 // 0.18..0.80
	bgSat := paint.ClampFloat(satMul*0.35+(satSpread-1)*0.08*(rng.Float64()-0.5), 0, 1)
	background := paint.HslToRGBA(baseHue, bgSat, bgLight)
	inkLight := 0.10
	if bgLight < 0.5 {
		inkLight = 0.92
	}
	ink := paint.HslToRGBA(baseHue, 0.18, inkLight)

	return ColorSet{Background: background, Ink: ink, Accent1: accents[0], Accent2: accents[1]}
}
