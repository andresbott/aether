package paint_test

import (
	"image/color"
	"math"
	"testing"

	"github.com/andresbott/aether/libs/covergen/internal/paint"
)

func absU8(a, b uint8) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}

func TestRgbToHslRoundTrip(t *testing.T) {
	for _, c := range []color.RGBA{
		{200, 40, 40, 255}, {40, 60, 200, 255}, {19, 236, 19, 255}, {128, 128, 128, 255}, {0, 0, 0, 255},
	} {
		h, s, l := paint.RgbToHsl(c)
		got := paint.HslToRGBA(h, s, l)
		if absU8(got.R, c.R) > 2 || absU8(got.G, c.G) > 2 || absU8(got.B, c.B) > 2 {
			t.Errorf("round trip %v -> hsl(%.1f, %.2f, %.2f) -> %v", c, h, s, l, got)
		}
	}
}

func TestRgbToHslGreyHasZeroSaturation(t *testing.T) {
	h, s, l := paint.RgbToHsl(color.RGBA{128, 128, 128, 255})
	if s != 0 || h != 0 {
		t.Errorf("grey hsl = (%.1f, %.2f, _), want hue 0 / sat 0", h, s)
	}
	if math.Abs(l-128.0/255) > 0.01 {
		t.Errorf("grey lightness = %.3f, want ~0.502", l)
	}
}

// Complement of a hue is 180 degrees away — the property classic relies on for
// contrasting text.
func TestRgbToHslHueIsComplementable(t *testing.T) {
	hGreen, _, _ := paint.RgbToHsl(color.RGBA{20, 200, 20, 255}) // ~120
	comp := paint.Hsl(hGreen+180, 0.85, 0.5)                     // ~magenta
	// magenta has R and B high, G low.
	if int(comp.G) >= int(comp.R) || int(comp.G) >= int(comp.B) {
		t.Errorf("complement of green %v is not magenta-ish", comp)
	}
}
