package covergen_test

import (
	"bytes"
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
			target := k.Max
			if target == k.Default {
				target = k.Min
			}
			changed := false
			for _, seed := range knobTestSeeds {
				base, tuned := renderKnobPair(g, style, seed, size, k.Name, target, fp)
				if !bytes.Equal(base, tuned) {
					changed = true
					break
				}
			}
			if !changed {
				t.Errorf("%s: knob %q (default %v, tried %v) changed no seed — dead or mis-wired", style.Name(), k.Name, k.Default, target)
			}
		}
	}
}

// renderKnobPair renders style at seed twice — once at style.Knobs() defaults,
// once with knob name overridden to target — and returns both PNG encodings for
// comparison. text.scale and text.opacity (see covergen.TextKnobs) are declared
// by every TextDrawer style but read only inside DrawText, which
// GenerateWithKnobs never reaches (it always renders with an empty Text); those
// two are routed through GenerateWithText with a non-empty overlay instead, the
// only path that actually exercises them. Every other knob keeps the plain
// GenerateWithKnobs check.
func renderKnobPair(g *covergen.Generator, style covergen.Style, seed string, size int, name string, target float64, fp covergen.FontProvider) (base, tuned []byte) {
	if name == covergen.TextScaleKnobName || name == covergen.TextOpacityKnobName {
		base, _ = g.GenerateWithText(seed, size, style, nil, knobTestText, fp)
		tuned, _ = g.GenerateWithText(seed, size, style, map[string]float64{name: target}, knobTestText, fp)
		return base, tuned
	}
	base, _ = g.GenerateWithKnobs(seed, size, style, nil)
	tuned, _ = g.GenerateWithKnobs(seed, size, style, map[string]float64{name: target})
	return base, tuned
}
