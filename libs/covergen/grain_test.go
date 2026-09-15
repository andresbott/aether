package covergen_test

import (
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/allstyles"
	"github.com/andresbott/aether/libs/covergen/svg"
)

// grainDefault returns the resolved default of a style's shared grain knob and
// whether the style declares one at all.
func grainDefault(s covergen.Style) (float64, bool) {
	for _, k := range s.Knobs() {
		if k.Name == covergen.GrainKnobName {
			return k.Default, true
		}
	}
	return 0, false
}

// TestEveryStyleDeclaresGrainKnobWithShippedDefault guards two invariants at once:
// every built-in style exposes the shared grain knob (so grain is tunable
// everywhere, per the lab work), and its default equals the style's shipped grain
// amount (the value tuned in the lab). Pinning the amount here means a stray edit
// to a GrainKnob() call fails loudly — a guarantee the local goldens can't provide
// on CI, where they are skipped.
func TestEveryStyleDeclaresGrainKnobWithShippedDefault(t *testing.T) {
	want := map[string]float64{
		"classic": 7,
		"bauhaus": 9,
		"rings":   6,
		"waves":   8,
		"poster":  5,
		"remix":   4,
		"svg":     4,
	}

	styles := append(allstyles.All(nil), svg.Style)
	if len(styles) != len(want) {
		t.Fatalf("checking %d styles but have %d expectations; update want", len(styles), len(want))
	}

	for _, s := range styles {
		def, ok := grainDefault(s)
		if !ok {
			t.Errorf("style %q declares no %q knob", s.Name(), covergen.GrainKnobName)
			continue
		}
		if w, known := want[s.Name()]; !known {
			t.Errorf("unexpected style %q; add it to want", s.Name())
		} else if def != w {
			t.Errorf("style %q grain default = %v, want %v (must match its shipped amount)", s.Name(), def, w)
		}
	}
}
