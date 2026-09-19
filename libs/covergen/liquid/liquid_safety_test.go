package liquid_test

import (
	"bytes"
	"image/png"
	"strconv"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/liquid"
)

// extremeLiquidKnobs pushes every liquid and palette knob to an in-range extreme
// (most blobs, widest spread, tightest threshold, hottest palette) to guard the
// per-pixel field sum and band thresholding against panics or degenerate output.
var extremeLiquidKnobs = map[string]float64{
	"liquid.blobs": 3, "liquid.blobsSpread": 10, "liquid.threshold": 3,
	"palette.saturation": 1.5, "palette.saturationSpread": 10,
	"palette.spread": 2, "palette.spreadJitter": 10,
}

// TestLiquidKnobsNeverPanicOrDegenerate renders many seeds under extreme (but
// in-range) knobs and asserts every one produces a decodable image without
// panicking.
func TestLiquidKnobsNeverPanicOrDegenerate(t *testing.T) {
	g := covergen.New(liquid.Style)
	for i := 0; i < 500; i++ {
		seed := "repro-" + strconv.Itoa(i)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("seed %q panicked: %v", seed, r)
				}
			}()
			data, err := g.GenerateWithKnobs(seed, 256, liquid.Style, extremeLiquidKnobs)
			if err != nil {
				t.Fatalf("seed %q errored: %v", seed, err)
			}
			if _, err := png.Decode(bytes.NewReader(data)); err != nil {
				t.Fatalf("seed %q produced undecodable PNG: %v", seed, err)
			}
		}()
	}
}
