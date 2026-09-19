package covergen

import (
	"crypto/sha256"
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
// knobs that tune it. It is self-describing via Name so callers (e.g. the lab)
// can offer a palette by name independently of the style using it.
type Palette interface {
	Name() string
	Knobs() []Knob
	Colors(rng *rand.Rand, ks KnobSet) ColorSet
}

// Palettes lists the built-in palettes, in registry order. It is the source the
// lab uses to populate its per-style palette picker; add a new palette here and
// it appears everywhere without further wiring. harmony stays first so it remains
// the natural default. Implementations of the extra palettes live in palettes.go.
func Palettes() []Palette {
	return []Palette{
		harmonyPalette{},
		monoPalette{},
		triadicPalette{},
		pastelPalette{},
		neonPalette{},
	}
}

// Colors resolves the ColorSet pal produces for seed, under knobs with overrides
// applied. It mirrors the render pipeline's seed->RNG and knob resolution (see
// renderStyle / newKnobSet), so a lab preview matches what a style paints when
// its Draw calls pal.Colors first. Pass a style's Knobs() as knobs so per-style
// palette-default overrides (e.g. classic's muted look) are reflected.
func Colors(pal Palette, seed string, knobs []Knob, overrides map[string]float64) ColorSet {
	h := sha256.Sum256([]byte(seed))
	return pal.Colors(rngFromHash(h), newKnobSet(knobs, overrides))
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

// PaletteByName returns the built-in palette whose Name matches name, or the
// default harmony palette when name is empty or unknown — so a stray or stale
// name can never break rendering. It is the name-keyed counterpart to
// DefaultPalette: the allstyles bundle uses it to pair each shipped style with
// the palette it was tuned for, and the lab uses it to resolve its picker.
func PaletteByName(name string) Palette {
	for _, p := range Palettes() {
		if p.Name() == name {
			return p
		}
	}
	return DefaultPalette()
}

func (harmonyPalette) Name() string  { return "harmony" }
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
