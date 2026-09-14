package svg

import (
	"image"
	"image/color"
	"math/rand/v2"
	"testing"
)

func TestRasterizeMotifShapeAndTransparency(t *testing.T) {
	src := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="45" fill="#FF0000"/></svg>`)
	img, err := rasterizeMotif(src, 64, 64)
	if err != nil {
		t.Fatalf("rasterizeMotif: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("size = %dx%d, want 64x64", b.Dx(), b.Dy())
	}
	if c := img.RGBAAt(32, 32); c.A == 0 {
		t.Errorf("centre is transparent; expected the circle to fill it")
	}
	if c := img.RGBAAt(0, 0); c.A != 0 {
		t.Errorf("corner alpha = %d; expected 0 (transparent outside circle)", c.A)
	}
}

func TestPaintGradientVaries(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	paintGradient(img, deterministicRNG(), rgba(255, 0, 0), rgba(0, 0, 255))
	if img.RGBAAt(0, 0) == img.RGBAAt(31, 31) {
		t.Errorf("gradient endpoints identical; expected a ramp")
	}
}

func deterministicRNG() *rand.Rand  { return rand.New(rand.NewPCG(1, 2)) }
func rgba(r, g, b uint8) color.RGBA { return color.RGBA{R: r, G: g, B: b, A: 255} }
