package covergen

import (
	"image"
	"image/color"
	"testing"
)

func fillRGBA(img *image.RGBA, c color.RGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func TestVariationFlatImageIsZero(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	fillRGBA(img, color.RGBA{R: 40, G: 80, B: 120, A: 255})
	if v := variation(img); v != 0 {
		t.Fatalf("flat image variation = %v, want 0", v)
	}
}

func TestVariationHalfSplitIsHalfRange(t *testing.T) {
	// Left half pure black, right half pure white: each channel is 0 on half
	// the pixels and 255 on the other half, so the standard deviation is 127.5.
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			c := color.RGBA{A: 255}
			if x >= 8 {
				c = color.RGBA{R: 255, G: 255, B: 255, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	v := variation(img)
	if v < 127 || v > 128 {
		t.Fatalf("half black/white variation = %v, want ~127.5", v)
	}
}
