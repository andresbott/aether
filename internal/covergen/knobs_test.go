package covergen_test

import (
	"bytes"
	"testing"

	"github.com/andresbott/aether/internal/covergen"
)

func TestStyleKnobsRingsAreDeclared(t *testing.T) {
	knobs := covergen.StyleRings.Knobs()
	if len(knobs) == 0 {
		t.Fatal("StyleRings.Knobs() returned no knobs")
	}
	var spacing *covergen.Knob
	for i := range knobs {
		if knobs[i].Name == "rings.spacing" {
			spacing = &knobs[i]
		}
	}
	if spacing == nil {
		t.Fatal("StyleRings.Knobs() is missing rings.spacing")
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
// GenerateStyle, not GenerateStyleWithKnobs, so this is its own guard).
func TestGenerateStyleWithKnobsDefaultsMatchGenerateStyle(t *testing.T) {
	const seed, size = "adele|19", 128
	for _, style := range covergen.Styles() {
		base, err := covergen.GenerateStyle(seed, size, style)
		if err != nil {
			t.Fatalf("GenerateStyle(%s): %v", style, err)
		}
		got, err := covergen.GenerateStyleWithKnobs(seed, size, style, nil)
		if err != nil {
			t.Fatalf("GenerateStyleWithKnobs(%s, nil): %v", style, err)
		}
		if !bytes.Equal(base, got) {
			t.Errorf("%s: GenerateStyleWithKnobs(nil) differs from GenerateStyle", style)
		}
	}
}

func TestGenerateStyleWithKnobsSpacingChangesRings(t *testing.T) {
	const seed, size = "adele|19", 64
	base, err := covergen.GenerateStyle(seed, size, covergen.StyleRings)
	if err != nil {
		t.Fatalf("GenerateStyle: %v", err)
	}
	tuned, err := covergen.GenerateStyleWithKnobs(seed, size, covergen.StyleRings, map[string]float64{"rings.spacing": 2.5})
	if err != nil {
		t.Fatalf("GenerateStyleWithKnobs: %v", err)
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

// TestEveryKnobIsWiredAndInertAtDefault is the Phase-4 guard for the knob
// rollout. For every style it asserts (a) nil overrides reproduce GenerateStyle
// exactly (knobs are inert at default), and (b) each declared knob, pushed away
// from its default, changes the output for at least one seed — so a knob that
// is declared but never read by the draw func (dead / mis-wired) fails here.
func TestEveryKnobIsWiredAndInertAtDefault(t *testing.T) {
	const size = 96
	for _, style := range covergen.Styles() {
		for _, seed := range knobTestSeeds {
			def, err := covergen.GenerateStyle(seed, size, style)
			if err != nil {
				t.Fatalf("%s/%q: GenerateStyle: %v", style, seed, err)
			}
			got, err := covergen.GenerateStyleWithKnobs(seed, size, style, nil)
			if err != nil {
				t.Fatalf("%s/%q: GenerateStyleWithKnobs(nil): %v", style, seed, err)
			}
			if !bytes.Equal(def, got) {
				t.Fatalf("%s/%q: nil overrides differ from GenerateStyle (knob not inert at default)", style, seed)
			}
		}
		for _, k := range style.Knobs() {
			target := k.Max
			if target == k.Default {
				target = k.Min
			}
			changed := false
			for _, seed := range knobTestSeeds {
				base, _ := covergen.GenerateStyleWithKnobs(seed, size, style, nil)
				tuned, _ := covergen.GenerateStyleWithKnobs(seed, size, style, map[string]float64{k.Name: target})
				if !bytes.Equal(base, tuned) {
					changed = true
					break
				}
			}
			if !changed {
				t.Errorf("%s: knob %q (default %v, tried %v) changed no seed — dead or mis-wired", style, k.Name, k.Default, target)
			}
		}
	}
}
