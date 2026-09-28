package covergen_test

import (
	"bytes"
	"image"
	"image/png"
	"math"
	"strconv"
	"testing"

	"github.com/andresbott/aether/libs/covergen/allstyles"
)

// coverVariation mirrors the render pipeline's internal colour-spread metric (the
// mean of the three per-channel standard deviations, 0..255 units).
func coverVariation(img image.Image) float64 {
	b := img.Bounds()
	n := float64(b.Dx() * b.Dy())
	if n == 0 {
		return 0
	}
	var sr, sg, sb, sr2, sg2, sb2 float64
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			rf, gf, bf := float64(r>>8), float64(g>>8), float64(bl>>8)
			sr += rf
			sg += gf
			sb += bf
			sr2 += rf * rf
			sg2 += gf * gf
			sb2 += bf * bf
		}
	}
	std := func(sum, sumsq float64) float64 {
		v := sumsq/n - (sum/n)*(sum/n)
		if v < 0 {
			v = 0
		}
		return math.Sqrt(v)
	}
	return (std(sr, sr2) + std(sg, sg2) + std(sb, sb2)) / 3
}

// coverEdgeProminence mirrors the render pipeline's internal edge metric: the
// fraction of pixels whose edge magnitude (max abs per-channel diff to the
// right/down neighbour) exceeds 20. A smooth gradient scores ~0.
func coverEdgeProminence(img image.Image) float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 2 || h < 2 {
		return 0
	}
	d := func(a, c uint32) float64 {
		x := float64(a>>8) - float64(c>>8)
		if x < 0 {
			x = -x
		}
		return x
	}
	var over int
	for y := b.Min.Y; y < b.Max.Y-1; y++ {
		for x := b.Min.X; x < b.Max.X-1; x++ {
			r0, g0, b0, _ := img.At(x, y).RGBA()
			r1, g1, b1, _ := img.At(x+1, y).RGBA()
			r2, g2, b2, _ := img.At(x, y+1).RGBA()
			e := max(d(r0, r1), d(g0, g1), d(b0, b1), d(r0, r2), d(g0, g2), d(b0, b2))
			if e > 20 {
				over++
			}
		}
	}
	return float64(over) / float64((w-1)*(h-1))
}

// TestEveryCoverIsNonDegenerate renders many seeds in every style and asserts no
// cover is degenerate on either axis: not a near-single colour (colour spread)
// and not a featureless gradient (edge prominence). Before the render pipeline's
// quality floors, bauhaus produced pure single-colour output and remix/classic
// produced smooth shapeless gradients (high colour spread, no visible shapes);
// the reseed loop deterministically replaces both. The thresholds here sit below
// the internal floors so the guarantee reads as the user-facing promise — "never
// a single flat colour, always some visible feature" — robust to the rare
// best-of-N render that lands just short of the floor it targets.
func TestEveryCoverIsNonDegenerate(t *testing.T) {
	const (
		notSingleColour = 5.0
		hasFeature      = 0.003
	)
	g := allstyles.New()
	for _, s := range g.Styles() {
		for i := 0; i < 300; i++ {
			seed := "guard-" + strconv.Itoa(i)
			data, err := g.GenerateStyle(seed, 256, s)
			if err != nil {
				t.Fatalf("style %s seed %q: %v", s.Name(), seed, err)
			}
			img, err := png.Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("style %s seed %q: decode: %v", s.Name(), seed, err)
			}
			if v := coverVariation(img); v < notSingleColour {
				t.Errorf("style %s seed %q: colour spread %.2f < %.2f — reads as a single colour", s.Name(), seed, v, notSingleColour)
			}
			if e := coverEdgeProminence(img); e < hasFeature {
				t.Errorf("style %s seed %q: edge prominence %.4f < %.4f — no visible shapes", s.Name(), seed, e, hasFeature)
			}
		}
	}
}
