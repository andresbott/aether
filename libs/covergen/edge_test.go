package covergen

import (
	"image"
	"image/color"
	"testing"
)

func fillGradient(img *image.RGBA) {
	b := img.Bounds()
	w := b.Dx()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			v := uint8(x * 255 / (w - 1))
			img.SetRGBA(x, y, color.RGBA{R: v, G: 40, B: uint8(200 - x*150/(w-1)), A: 255})
		}
	}
}

func fillHalfSplit(img *image.RGBA) {
	b := img.Bounds()
	mid := b.Min.X + b.Dx()/2
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.RGBA{R: 20, G: 20, B: 40, A: 255}
			if x >= mid {
				c = color.RGBA{R: 240, G: 240, B: 255, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
}

func TestEdgeProminenceFlatIsZero(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	fillRGBA(img, color.RGBA{R: 50, G: 100, B: 150, A: 255})
	if e := edgeProminence(img); e != 0 {
		t.Fatalf("flat edgeProminence = %v, want 0", e)
	}
}

func TestEdgeProminenceSmoothGradientIsZero(t *testing.T) {
	// A smooth gradient has large global spread but per-pixel steps far below the
	// edge threshold, so no strong edges — this is exactly what global stddev misses.
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	fillGradient(img)
	if e := edgeProminence(img); e != 0 {
		t.Fatalf("smooth gradient edgeProminence = %v, want 0", e)
	}
}

func TestEdgeProminenceHardEdgeIsPositive(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	fillHalfSplit(img)
	e := edgeProminence(img)
	if e < 0.008 || e > 0.02 {
		t.Fatalf("hard-edge edgeProminence = %v, want ~0.01", e)
	}
}

func TestCoverScoreRejectsFeaturelessGradient(t *testing.T) {
	// The reported bug: a smooth gradient clears the colour-spread floor but has
	// no visible shapes, so its combined score must be < 1 (reject -> reseed).
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	fillGradient(img)
	if s := coverScore(img); s >= 1 {
		t.Fatalf("featureless gradient coverScore = %v, want < 1", s)
	}
}

func TestCoverScoreAcceptsShapeBearingImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	fillHalfSplit(img)
	if s := coverScore(img); s < 1 {
		t.Fatalf("shape-bearing image coverScore = %v, want >= 1", s)
	}
}

func TestCoverScoreRejectsFlat(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	fillRGBA(img, color.RGBA{R: 10, G: 20, B: 30, A: 255})
	if s := coverScore(img); s != 0 {
		t.Fatalf("flat coverScore = %v, want 0", s)
	}
}
