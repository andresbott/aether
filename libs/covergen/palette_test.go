package covergen

import (
	"crypto/sha256"
	"image/color"
	"math"
	"strconv"
	"testing"

	"github.com/andresbott/aether/libs/covergen/internal/paint"
)

func TestDefaultPaletteDeterministic(t *testing.T) {
	ks := newKnobSet(DefaultPalette().Knobs(), nil)
	h := sha256.Sum256([]byte("seed"))
	a := DefaultPalette().Colors(rngFromHash(h), ks)
	b := DefaultPalette().Colors(rngFromHash(h), ks)
	if a != b {
		t.Fatalf("Colors not deterministic: %v vs %v", a, b)
	}
}

func TestDefaultPaletteFourOpaqueColors(t *testing.T) {
	ks := newKnobSet(DefaultPalette().Knobs(), nil)
	for i := 0; i < 200; i++ {
		h := sha256.Sum256([]byte("seed-" + strconv.Itoa(i)))
		cs := DefaultPalette().Colors(rngFromHash(h), ks)
		for _, c := range []color.RGBA{cs.Background, cs.Ink, cs.Accent1, cs.Accent2} {
			if c.A != 255 {
				t.Fatalf("seed %d: non-opaque color %v", i, c)
			}
		}
	}
}

func TestDefaultPaletteBackgroundInkContrast(t *testing.T) {
	ks := newKnobSet(DefaultPalette().Knobs(), nil)
	for i := 0; i < 200; i++ {
		h := sha256.Sum256([]byte("contrast-" + strconv.Itoa(i)))
		cs := DefaultPalette().Colors(rngFromHash(h), ks)
		if d := math.Abs(paint.Luminance(cs.Background) - paint.Luminance(cs.Ink)); d < 50 {
			t.Fatalf("seed %d: Background/Ink luminance gap %.1f < 50 (bg=%v ink=%v)", i, d, cs.Background, cs.Ink)
		}
	}
}

func TestDefaultPaletteSaturationKnobChangesColors(t *testing.T) {
	h := sha256.Sum256([]byte("knob"))
	base := DefaultPalette().Colors(rngFromHash(h), newKnobSet(DefaultPalette().Knobs(), nil))
	tuned := DefaultPalette().Colors(rngFromHash(h), newKnobSet(DefaultPalette().Knobs(), map[string]float64{"palette.saturation": 0}))
	if base == tuned {
		t.Fatal("palette.saturation override had no effect on colors")
	}
}
