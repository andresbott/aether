package halftone_test

import (
	"bytes"
	"image/png"
	"strconv"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/halftone"
)

// extremeHalftoneKnobs pushes every halftone and palette knob to an in-range
// extreme (finest pitch, widest spread, hardest contrast, hottest palette) to
// guard the rotated-screen sampling against panics or degenerate output.
var extremeHalftoneKnobs = map[string]float64{
	"halftone.pitch": 0.3, "halftone.pitchSpread": 10, "halftone.contrast": 2.5,
	"palette.saturation": 1.5, "palette.saturationSpread": 10,
	"palette.toneSpread": 2,
}

// TestHalftoneKnobsNeverPanicOrDegenerate renders many seeds under extreme (but
// in-range) knobs and asserts every one produces a decodable image without
// panicking.
func TestHalftoneKnobsNeverPanicOrDegenerate(t *testing.T) {
	g := covergen.New(halftone.Style)
	for i := 0; i < 500; i++ {
		seed := "repro-" + strconv.Itoa(i)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("seed %q panicked: %v", seed, r)
				}
			}()
			data, err := g.GenerateWithKnobs(seed, 256, halftone.Style, extremeHalftoneKnobs)
			if err != nil {
				t.Fatalf("seed %q errored: %v", seed, err)
			}
			if _, err := png.Decode(bytes.NewReader(data)); err != nil {
				t.Fatalf("seed %q produced undecodable PNG: %v", seed, err)
			}
		}()
	}
}
