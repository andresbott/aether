package allstyles

import (
	"testing"

	"github.com/andresbott/aether/libs/covergen"
)

// Every built-in style must declare a shipping palette that resolves to a real
// palette of that name, so the lab (and anyone else) can default to it.
func TestDefaultPaletteNameCoversAllStyles(t *testing.T) {
	for _, s := range All(covergen.DefaultPalette()) {
		name := DefaultPaletteName(s.Name())
		if name == "" {
			t.Errorf("style %q has no shipping palette", s.Name())
			continue
		}
		if got := covergen.PaletteByName(name).Name(); got != name {
			t.Errorf("style %q: shipping palette %q is not a real palette (resolved to %q)", s.Name(), name, got)
		}
	}
}

func TestDefaultPaletteNamePairsRingsWithNeon(t *testing.T) {
	if got := DefaultPaletteName("rings"); got != "neon" {
		t.Errorf("rings should ship on neon, got %q", got)
	}
}

func TestDefaultPaletteNameUnknownIsEmpty(t *testing.T) {
	if got := DefaultPaletteName("svg"); got != "" {
		t.Errorf("a style with no pairing should return \"\", got %q", got)
	}
}
