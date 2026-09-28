package rings_test

import (
	"bytes"
	"image/png"
	"strconv"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/rings"
)

// brokenRingsKnobs is the hand-tuned rings configuration that produced broken
// renders in the lab (values are all within the declared knob ranges).
var brokenRingsKnobs = map[string]float64{
	"rings.spacing": 0.75, "rings.spacingSpread": 5.3,
	"rings.wobble": 0.65, "rings.wobbleSpread": 7.15,
	"rings.corners": 0.6, "rings.cornersSpread": 3.15,
	"palette.saturation": 1.3, "palette.saturationSpread": 5.65,
	"palette.hue": 1.15, "palette.hueSpread": 2.1,
}

// TestRingsKnobsNeverPanicOrDegenerate renders many seeds with an extreme (but
// in-range) knob set and asserts every one produces a decodable image without
// panicking. Fails today: variant 2's step goes non-positive, indexing pal with
// a negative band.
func TestRingsKnobsNeverPanicOrDegenerate(t *testing.T) {
	g := covergen.New(rings.Style)
	for i := 0; i < 500; i++ {
		seed := "repro-" + strconv.Itoa(i)
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("seed %q panicked: %v", seed, r)
				}
			}()
			data, err := g.GenerateWithKnobs(seed, 256, rings.Style, brokenRingsKnobs)
			if err != nil {
				t.Fatalf("seed %q errored: %v", seed, err)
			}
			if _, err := png.Decode(bytes.NewReader(data)); err != nil {
				t.Fatalf("seed %q produced undecodable PNG: %v", seed, err)
			}
		}()
	}
}
