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
		if td.TextClass() == "" {
			t.Fatalf("style %q has empty TextClass", s.Name())
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
	for _, s := range allstyles.New().Styles() {
		var hasScale, hasOpacity bool
		for _, k := range s.Knobs() {
			hasScale = hasScale || k.Name == covergen.TextScaleKnobName
			hasOpacity = hasOpacity || k.Name == covergen.TextOpacityKnobName
		}
		if !hasScale || !hasOpacity {
			t.Fatalf("style %q missing text knobs", s.Name())
		}
	}
}
