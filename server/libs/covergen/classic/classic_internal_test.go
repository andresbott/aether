package classic

import (
	"image/color"
	"math/rand/v2"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
)

type stubPal struct{ cs covergen.ColorSet }

func (p stubPal) Name() string                                          { return "stub" }
func (p stubPal) Knobs() []covergen.Knob                                { return nil }
func (p stubPal) Colors(*rand.Rand, covergen.KnobSet) covergen.ColorSet { return p.cs }

func TestColorsExposesPalette(t *testing.T) {
	cs := covergen.ColorSet{Accent1: color.RGBA{9, 9, 9, 255}, Ink: color.RGBA{1, 1, 1, 255}}
	s := style{pal: stubPal{cs: cs}}
	if got := s.Colors(nil, covergen.KnobSet{}); got != cs {
		t.Fatalf("Colors = %v, want %v", got, cs)
	}
}
