package covergen_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand/v2"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
)

type stubStyle struct{ name string }

func (s stubStyle) Name() string           { return s.name }
func (s stubStyle) Knobs() []covergen.Knob { return nil }
func (s stubStyle) Draw(img *image.RGBA, _ *rand.Rand, _ covergen.KnobSet) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 1, A: 255})
		}
	}
}

func TestGeneratorRendersAndDispatches(t *testing.T) {
	g := covergen.New(stubStyle{"a"}, stubStyle{"b"})
	data, err := g.Generate("seed", 32)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("decode: %v", err)
	}
	first := g.StyleFor("seed").Name()
	second := g.StyleFor("seed").Name()
	if first != second {
		t.Fatal("StyleFor not deterministic")
	}
	if _, ok := g.ByName("b"); !ok {
		t.Fatal("ByName(b) missing")
	}
	if _, ok := g.ByName("nope"); ok {
		t.Fatal("ByName(nope) should be absent")
	}
	if _, err := covergen.New().Generate("seed", 32); err == nil {
		t.Fatal("empty generator should error")
	}
}
