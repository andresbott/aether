package fonts

import (
	"embed"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

//go:embed assets/*.ttf
var testAssets embed.FS

func TestAssetsParseAndRender(t *testing.T) {
	files := []string{
		"assets/PTSerif-Regular.ttf",
		"assets/PTSans-Regular.ttf",
		"assets/ArchivoBlack-Regular.ttf",
		"assets/SpaceMono-Regular.ttf",
		"assets/Pacifico-Regular.ttf",
	}
	for _, f := range files {
		b, err := testAssets.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		ft, err := opentype.Parse(b)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		face, err := opentype.NewFace(ft, &opentype.FaceOptions{Size: 32, DPI: 72, Hinting: font.HintingFull})
		if err != nil {
			t.Fatalf("face %s: %v", f, err)
		}
		adv, ok := face.GlyphAdvance('A')
		if !ok || adv == fixed.Int26_6(0) {
			t.Fatalf("%s has no advance for 'A' (variable-font default not rendering?)", f)
		}
	}
}
