package waves_test

import (
	"bytes"
	"image/color"
	"image/png"
	"math/rand/v2"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/waves"
)

func TestStyleRenders(t *testing.T) {
	g := covergen.New(waves.Style)
	data, err := g.GenerateStyle("smoke", 64, waves.Style)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if waves.Style.Name() != "waves" {
		t.Fatalf("Name = %q", waves.Style.Name())
	}
}

type stubPalette struct{ c covergen.ColorSet }

func (p stubPalette) Knobs() []covergen.Knob                                { return nil }
func (p stubPalette) Colors(*rand.Rand, covergen.KnobSet) covergen.ColorSet { return p.c }

func renderWith(t *testing.T, st covergen.Style) []byte {
	t.Helper()
	data, err := covergen.New(st).GenerateStyle("palette seed", 64, st)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestStyleUsesInjectedPalette(t *testing.T) {
	red := covergen.ColorSet{
		Background: color.RGBA{200, 20, 20, 255}, Ink: color.RGBA{10, 10, 10, 255},
		Accent1: color.RGBA{230, 60, 60, 255}, Accent2: color.RGBA{120, 0, 0, 255},
	}
	blue := covergen.ColorSet{
		Background: color.RGBA{20, 20, 200, 255}, Ink: color.RGBA{240, 240, 240, 255},
		Accent1: color.RGBA{60, 60, 230, 255}, Accent2: color.RGBA{0, 0, 120, 255},
	}
	if bytes.Equal(renderWith(t, waves.New(stubPalette{red})), renderWith(t, waves.New(stubPalette{blue}))) {
		t.Fatal("different palettes produced identical output; palette not consumed")
	}
}
