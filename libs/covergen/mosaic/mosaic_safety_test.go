package mosaic_test

import (
	"bytes"
	"image/png"
	"strconv"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/mosaic"
)

// extremeMosaicKnobs pushes every mosaic and palette knob to an in-range
// extreme (densest cells, widest spread, thickest leading, hottest palette) to
// guard the per-pixel nearest-site scan and border test against panics or
// degenerate output.
var extremeMosaicKnobs = map[string]float64{
	"mosaic.cells": 3, "mosaic.cellsSpread": 10, "mosaic.border": 3,
	"palette.saturation": 1.5, "palette.saturationSpread": 10,
	"palette.hueGap": 3, "palette.glow": 1,
}

// TestMosaicKnobsNeverPanicOrDegenerate renders many seeds under extreme (but
// in-range) knobs and asserts every one produces a decodable image without
// panicking.
func TestMosaicKnobsNeverPanicOrDegenerate(t *testing.T) {
	g := covergen.New(mosaic.Style)
	for i := 0; i < 500; i++ {
		seed := "repro-" + strconv.Itoa(i)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("seed %q panicked: %v", seed, r)
				}
			}()
			data, err := g.GenerateWithKnobs(seed, 256, mosaic.Style, extremeMosaicKnobs)
			if err != nil {
				t.Fatalf("seed %q errored: %v", seed, err)
			}
			if _, err := png.Decode(bytes.NewReader(data)); err != nil {
				t.Fatalf("seed %q produced undecodable PNG: %v", seed, err)
			}
		}()
	}
}
