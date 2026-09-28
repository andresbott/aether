package lowpoly_test

import (
	"bytes"
	"image/png"
	"strconv"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/lowpoly"
)

// extremeLowpolyKnobs pushes every lowpoly and palette knob to an in-range
// extreme (densest grid, widest spread, maximum vertex jitter, hottest palette)
// to guard the lattice build and triangle rasteriser against panics or
// degenerate output.
var extremeLowpolyKnobs = map[string]float64{
	"lowpoly.density": 3, "lowpoly.densitySpread": 10, "lowpoly.jitter": 0.7,
	"palette.saturation": 1.5, "palette.saturationSpread": 10,
	"palette.spread": 2, "palette.spreadJitter": 10,
}

// TestLowpolyKnobsNeverPanicOrDegenerate renders many seeds under extreme (but
// in-range) knobs and asserts every one produces a decodable image without
// panicking.
func TestLowpolyKnobsNeverPanicOrDegenerate(t *testing.T) {
	g := covergen.New(lowpoly.Style)
	for i := 0; i < 500; i++ {
		seed := "repro-" + strconv.Itoa(i)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("seed %q panicked: %v", seed, r)
				}
			}()
			data, err := g.GenerateWithKnobs(seed, 256, lowpoly.Style, extremeLowpolyKnobs)
			if err != nil {
				t.Fatalf("seed %q errored: %v", seed, err)
			}
			if _, err := png.Decode(bytes.NewReader(data)); err != nil {
				t.Fatalf("seed %q produced undecodable PNG: %v", seed, err)
			}
		}()
	}
}
