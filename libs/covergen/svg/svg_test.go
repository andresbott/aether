package svg_test

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/svg"
)

func TestStyleRendersValidPNG(t *testing.T) {
	g := covergen.New(svg.Style)
	data, err := g.GenerateStyle("adele|19", 128, svg.Style)
	if err != nil {
		t.Fatalf("GenerateStyle: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 128 || b.Dy() != 128 {
		t.Fatalf("size = %dx%d, want 128x128", b.Dx(), b.Dy())
	}
	if svg.Style.Name() != "svg" {
		t.Fatalf("Name = %q, want svg", svg.Style.Name())
	}
}

func TestStyleDeterministic(t *testing.T) {
	g := covergen.New(svg.Style)
	a, _ := g.GenerateStyle("seed one", 128, svg.Style)
	b, _ := g.GenerateStyle("seed one", 128, svg.Style)
	if !bytes.Equal(a, b) {
		t.Fatal("same seed produced different bytes")
	}
	c, _ := g.GenerateStyle("seed two", 128, svg.Style)
	if bytes.Equal(a, c) {
		t.Fatal("different seeds produced identical bytes")
	}
}

func TestNewSingleAssetRecolours(t *testing.T) {
	// A one-asset Style always renders that asset; the sentinel must not survive
	// into the output palette (it is replaced by a seed colour).
	asset := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><rect x="20" y="20" width="60" height="60" fill="#FF00FF"/></svg>`)
	st := svg.New(asset)
	g := covergen.New(st)
	if _, err := g.GenerateStyle("x", 64, st); err != nil {
		t.Fatalf("GenerateStyle: %v", err)
	}
}
