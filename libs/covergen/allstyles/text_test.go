package allstyles_test

import (
	"bytes"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/allstyles"
	"github.com/andresbott/aether/libs/covergen/fonts"
)

func TestEveryStyleDrawsText(t *testing.T) {
	fp := fonts.Default()
	g := allstyles.New()
	for _, s := range g.Styles() {
		td, ok := s.(covergen.TextDrawer)
		if !ok {
			t.Fatalf("style %q is not a TextDrawer", s.Name())
		}
		if len(td.TextClasses()) == 0 {
			t.Fatalf("style %q declares no text classes", s.Name())
		}
		plain, err := g.GenerateStyle("seed-xyz", 256, s)
		if err != nil {
			t.Fatal(err)
		}
		withText, err := g.GenerateWithText("seed-xyz", 256, s, nil,
			covergen.Text{Main: "Midnight Drive", Subtitle: "The Wanderers"}, fp)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(plain, withText) {
			t.Fatalf("style %q: text overlay did not change output", s.Name())
		}
	}
}

func TestEveryStyleDeclaresTextKnobs(t *testing.T) {
	want := []string{
		covergen.TextScaleKnobName, covergen.TextSizeSpreadKnobName,
		covergen.TextOpacityKnobName, covergen.TextOpacitySpreadKnobName,
		covergen.TextRoamKnobName, covergen.TextTintKnobName,
		covergen.TextSaturationKnobName, covergen.TextSaturationSpreadKnobName,
	}
	for _, s := range allstyles.New().Styles() {
		have := map[string]bool{}
		for _, k := range s.Knobs() {
			have[k.Name] = true
		}
		for _, name := range want {
			if !have[name] {
				t.Errorf("style %q missing text-overlay knob %q", s.Name(), name)
			}
		}
	}
}
