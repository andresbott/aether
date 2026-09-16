package text

import (
	"image"
	"image/color"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

func fillRGBA(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func nonBackground(img *image.RGBA, bg color.RGBA) int {
	n := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.RGBAAt(x, y) != bg {
				n++
			}
		}
	}
	return n
}

func TestDrawStringMarksPixels(t *testing.T) {
	bg := color.RGBA{0, 0, 0, 255}
	img := fillRGBA(200, 60, bg)
	adv := DrawString(img, basicfont.Face7x13, "Hi", 5, 30, color.RGBA{255, 255, 255, 255})
	if adv <= 0 {
		t.Fatalf("advance = %d, want > 0", adv)
	}
	if nonBackground(img, bg) == 0 {
		t.Fatal("DrawString drew nothing")
	}
}

func TestAutoContrastColor(t *testing.T) {
	dark := fillRGBA(20, 20, color.RGBA{10, 10, 10, 255})
	ink, _ := AutoContrastColor(dark, dark.Bounds())
	if ink.R < 128 {
		t.Fatalf("ink over dark = %v, want light", ink)
	}
	light := fillRGBA(20, 20, color.RGBA{240, 240, 240, 255})
	ink2, _ := AutoContrastColor(light, light.Bounds())
	if ink2.R > 128 {
		t.Fatalf("ink over light = %v, want dark", ink2)
	}
}

func TestBlockDrawsBothLines(t *testing.T) {
	bg := color.RGBA{0, 0, 0, 255}
	faceAt := func(float64) font.Face { return basicfont.Face7x13 }

	// Render main-only Block
	imgMain := fillRGBA(240, 240, bg)
	Block(imgMain, "TITLE", "",
		faceAt, 18, AnchorLowerLeft, 12, 1)
	mainOnlyPixels := nonBackground(imgMain, bg)
	if mainOnlyPixels == 0 {
		t.Fatal("Block with main only drew nothing")
	}

	// Render main+subtitle Block
	imgBoth := fillRGBA(240, 240, bg)
	Block(imgBoth, "TITLE", "subtitle",
		faceAt, 18, AnchorLowerLeft, 12, 1)
	bothPixels := nonBackground(imgBoth, bg)
	if bothPixels == 0 {
		t.Fatal("Block with both lines drew nothing")
	}

	// Verify subtitle added pixels (proves both lines rendered)
	if bothPixels <= mainOnlyPixels {
		t.Fatalf("Block with subtitle drew %d pixels, main-only drew %d; subtitle must add pixels",
			bothPixels, mainOnlyPixels)
	}
}
