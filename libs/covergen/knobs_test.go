package covergen_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/allstyles"
	"github.com/andresbott/aether/libs/covergen/fonts"
	"github.com/andresbott/aether/libs/covergen/rings"
)

func TestStyleKnobsRingsAreDeclared(t *testing.T) {
	knobs := rings.Style.Knobs()
	if len(knobs) == 0 {
		t.Fatal("rings.Style.Knobs() returned no knobs")
	}
	var spacing *covergen.Knob
	for i := range knobs {
		if knobs[i].Name == "rings.spacing" {
			spacing = &knobs[i]
		}
	}
	if spacing == nil {
		t.Fatal("rings.Style.Knobs() is missing rings.spacing")
	}
	if spacing.Default != 1.0 {
		t.Errorf("rings.spacing default = %v, want 1.0 (identity so defaults stay byte-identical)", spacing.Default)
	}
	if !(spacing.Min <= spacing.Default && spacing.Default <= spacing.Max) {
		t.Errorf("rings.spacing default %v outside [%v, %v]", spacing.Default, spacing.Min, spacing.Max)
	}
}

// Empty overrides must reproduce GenerateStyle exactly — the identity contract
// the whole "inert at defaults" guarantee rests on (goldens cover Generate/
// GenerateStyle, not GenerateWithKnobs, so this is its own guard).
func TestGenerateStyleWithKnobsDefaultsMatchGenerateStyle(t *testing.T) {
	g := allstyles.New()
	const seed, size = "adele|19", 128
	for _, style := range g.Styles() {
		base, err := g.GenerateStyle(seed, size, style)
		if err != nil {
			t.Fatalf("GenerateStyle(%s): %v", style.Name(), err)
		}
		got, err := g.GenerateWithKnobs(seed, size, style, nil)
		if err != nil {
			t.Fatalf("GenerateWithKnobs(%s, nil): %v", style.Name(), err)
		}
		if !bytes.Equal(base, got) {
			t.Errorf("%s: GenerateWithKnobs(nil) differs from GenerateStyle", style.Name())
		}
	}
}

func TestGenerateStyleWithKnobsSpacingChangesRings(t *testing.T) {
	g := allstyles.New()
	const seed, size = "adele|19", 64
	base, err := g.GenerateStyle(seed, size, rings.Style)
	if err != nil {
		t.Fatalf("GenerateStyle: %v", err)
	}
	tuned, err := g.GenerateWithKnobs(seed, size, rings.Style, map[string]float64{"rings.spacing": 2.5})
	if err != nil {
		t.Fatalf("GenerateWithKnobs: %v", err)
	}
	if bytes.Equal(base, tuned) {
		t.Error("overriding rings.spacing did not change the output")
	}
}

// knobTestSeeds is a fixed spread used to exercise knobs across the per-seed
// variants each style picks. Fixed (not random) so the test is deterministic.
var knobTestSeeds = []string{
	"adele|19", "daft punk|one more time", "miles davis|kind of blue",
	"metallica|the memory remains", "clutch|the regulator", "air|la femme d'argent",
	"est|behind the yashmak", "steve lacy|jazz adv", "boards of canada|children",
	"dimmu borgir|stormblast", "raised fist|get this right", "various artists|ugly",
}

// knobTestText is the overlay TestEveryKnobIsWiredAndInertAtDefault feeds
// through GenerateWithText to exercise the shared text.scale/text.opacity knobs
// (see renderKnobPair) — Draw never reads them, so only a non-empty Text drives
// DrawText and gives them anything to wire-check.
var knobTestText = covergen.Text{Main: "Test Artist", Subtitle: "Test Album"}

// TestEveryKnobIsWiredAndInertAtDefault is the Phase-4 guard for the knob
// rollout. For every style it asserts (a) nil overrides reproduce GenerateStyle
// exactly (knobs are inert at default), and (b) each declared knob, pushed away
// from its default, changes the output for at least one seed — so a knob that
// is declared but never read by the draw func (dead / mis-wired) fails here.
func TestEveryKnobIsWiredAndInertAtDefault(t *testing.T) {
	g := allstyles.New()
	fp := fonts.Default()
	const size = 96
	for _, style := range g.Styles() {
		for _, seed := range knobTestSeeds {
			def, err := g.GenerateStyle(seed, size, style)
			if err != nil {
				t.Fatalf("%s/%q: GenerateStyle: %v", style.Name(), seed, err)
			}
			got, err := g.GenerateWithKnobs(seed, size, style, nil)
			if err != nil {
				t.Fatalf("%s/%q: GenerateWithKnobs(nil): %v", style.Name(), seed, err)
			}
			if !bytes.Equal(def, got) {
				t.Fatalf("%s/%q: nil overrides differ from GenerateStyle (knob not inert at default)", style.Name(), seed)
			}
		}
		for _, k := range style.Knobs() {
			extra := wireCheckContext(k.Name)
			changed := false
			for _, target := range wireCheckTargets(k) {
				for _, seed := range knobTestSeeds {
					base, tuned := renderKnobPair(g, style, seed, size, k.Name, target, extra, fp)
					if !bytes.Equal(base, tuned) {
						changed = true
						break
					}
				}
				if changed {
					break
				}
			}
			if !changed {
				t.Errorf("%s: knob %q (default %v) changed no seed at either extreme — dead or mis-wired", style.Name(), k.Name, k.Default)
			}
		}
	}
}

// renderKnobPair renders style at seed twice — once at style.Knobs() defaults,
// once with knob name overridden to target — and returns both PNG encodings for
// comparison. extra is applied to BOTH renders (see wireCheckContext) so the two
// still differ only by name, letting a knob that a style's other defaults happen
// to mask be checked in a context where it can act. Text-overlay knobs (any whose
// name carries a "text" token: the shared text.scale/text.opacity centres plus the
// text.sizeSpread/opacitySpread/roam/tint variety knobs, see
// covergen.TextOverlayKnobs) are read only inside DrawText, which GenerateWithKnobs
// never reaches (it always renders an empty Text); those are routed through
// GenerateWithText with a non-empty overlay, the only path that exercises them.
// Every other knob keeps the plain GenerateWithKnobs check.
func renderKnobPair(g *covergen.Generator, style covergen.Style, seed string, size int, name string, target float64, extra map[string]float64, fp covergen.FontProvider) (base, tuned []byte) {
	baseOv := mergeOverrides(extra, nil)
	tunedOv := mergeOverrides(extra, map[string]float64{name: target})
	if isTextOverlayKnob(name) {
		base, _ = g.GenerateWithText(seed, size, style, baseOv, knobTestText, fp)
		tuned, _ = g.GenerateWithText(seed, size, style, tunedOv, knobTestText, fp)
		return base, tuned
	}
	base, _ = g.GenerateWithKnobs(seed, size, style, baseOv)
	tuned, _ = g.GenerateWithKnobs(seed, size, style, tunedOv)
	return base, tuned
}

// wireCheckTargets returns the knob values the wire-check pushes name toward: the Max
// and Min extremes that differ from its default. Trying BOTH matters for the pool-sized
// variety knobs (text.tint / text.roam) — poolSize buckets a 0..1 range into 1..n, so a
// default and the nearer extreme can share a bucket (e.g. tint 0.75 and 1 both map to
// the full pool) and render identically; the far extreme still moves it. A knob that
// changes nothing at either extreme is genuinely dead.
func wireCheckTargets(k covergen.Knob) []float64 {
	var out []float64
	if k.Max != k.Default {
		out = append(out, k.Max)
	}
	if k.Min != k.Default {
		out = append(out, k.Min)
	}
	return out
}

// wireCheckContext returns knob overrides applied to both renders of name's
// wire-check, so a knob genuinely read by the engine but masked by a style's other
// defaults is still verified as wired (not flagged dead). The text.saturation knobs
// only tint a palette-derived ink, so a style whose text.tint default never selects
// one (auto black/white only, e.g. waves at tint 0) would otherwise mask them; forcing
// text.tint high puts the tinted ink in play so the check reflects the engine wiring,
// not the style's tint choice. text.saturationSpread additionally scales a per-seed
// jitter around the saturation CENTRE, so a style shipping saturation 0 (e.g. lowpoly)
// would zero it out — its check also forces a non-zero saturation centre. Returns nil
// for every other knob, preserving the plain default-context check.
func wireCheckContext(name string) map[string]float64 {
	switch name {
	case covergen.TextSaturationKnobName:
		return map[string]float64{covergen.TextTintKnobName: 1}
	case covergen.TextSaturationSpreadKnobName:
		return map[string]float64{covergen.TextTintKnobName: 1, covergen.TextSaturationKnobName: 0.85}
	}
	return nil
}

// mergeOverrides unions a and b (b wins on conflict), returning nil when both are
// empty so the default-context path stays byte-identical to a nil override.
func mergeOverrides(a, b map[string]float64) map[string]float64 {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	out := make(map[string]float64, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// isTextOverlayKnob reports whether a knob is read only in the text overlay, so
// it must be wire-checked through GenerateWithText. All such knob names carry a
// "text" token (text.scale, text.opacity, text.roam, text.tint, ...).
func isTextOverlayKnob(name string) bool {
	return strings.Contains(strings.ToLower(name), "text")
}
