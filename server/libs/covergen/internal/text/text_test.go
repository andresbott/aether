package text

import (
	"image"
	"image/color"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"

	"github.com/andresbott/aether/libs/covergen/internal/paint"
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
		faceAt, 18, AnchorLowerLeft, 12, 1, nil)
	mainOnlyPixels := nonBackground(imgMain, bg)
	if mainOnlyPixels == 0 {
		t.Fatal("Block with main only drew nothing")
	}

	// Render main+subtitle Block
	imgBoth := fillRGBA(240, 240, bg)
	Block(imgBoth, "TITLE", "subtitle",
		faceAt, 18, AnchorLowerLeft, 12, 1, nil)
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

func TestBlockOutlineDrawsFillAndBands(t *testing.T) {
	bg := color.RGBA{0, 0, 0, 255}
	faceAt := func(float64) font.Face { return basicfont.Face7x13 }
	o := Outline{
		Fill: color.NRGBA{250, 250, 250, 255},
		Bands: []OutlineBand{
			{Thickness: 3, Color: color.NRGBA{220, 40, 40, 255}},
			{Thickness: 3, Color: color.NRGBA{40, 80, 220, 255}},
		},
	}
	img := fillRGBA(240, 120, bg)
	BlockOutline(img, "TITLE", "sub", faceAt, 24, AnchorCenter, 12, 1, o)

	// The three treatment colours (fill + both bands) must all appear, proving the
	// glyph core and each concentric border rendered.
	seen := map[color.RGBA]bool{}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			seen[img.RGBAAt(x, y)] = true
		}
	}
	for _, c := range []color.RGBA{{250, 250, 250, 255}, {220, 40, 40, 255}, {40, 80, 220, 255}} {
		if !seen[c] {
			t.Errorf("BlockOutline did not draw colour %v (fill/band missing)", c)
		}
	}
}

func TestBlockOutlineNoBandsDrawsNothing(t *testing.T) {
	bg := color.RGBA{0, 0, 0, 255}
	img := fillRGBA(240, 120, bg)
	BlockOutline(img, "TITLE", "sub", func(float64) font.Face { return basicfont.Face7x13 },
		24, AnchorCenter, 12, 1, Outline{Fill: color.NRGBA{255, 255, 255, 255}})
	if n := nonBackground(img, bg); n != 0 {
		t.Fatalf("BlockOutline with no bands drew %d pixels, want 0", n)
	}
}

func TestInkForUsesWantWhenLegible(t *testing.T) {
	dark := fillRGBA(20, 20, color.RGBA{10, 10, 10, 255})
	want := color.NRGBA{230, 60, 60, 255} // bright red — high contrast over near-black
	ink, _ := inkFor(dark, dark.Bounds(), &want)
	if ink != want {
		t.Fatalf("legible custom ink not used: ink = %v, want %v", ink, want)
	}
}

func TestInkForKeepsColourWhenLowContrast(t *testing.T) {
	// A saturated red over a dark grey: the red does not contrast on its own, but
	// inkFor must keep it RED (hue) and only lighten it — never grey it to B/W.
	mid := fillRGBA(20, 20, color.RGBA{80, 80, 80, 255})
	want := color.NRGBA{150, 30, 30, 255}
	ink, _ := inkFor(mid, mid.Bounds(), &want)
	if int(ink.R) <= int(ink.G)+25 || int(ink.R) <= int(ink.B)+25 {
		t.Errorf("ink %v lost its red hue (must keep palette colour, not fall back to B/W)", ink)
	}
	if l := paint.Luminance(color.RGBA{ink.R, ink.G, ink.B, 255}); l <= 80 {
		t.Errorf("ink %v was not lightened to contrast the dark bg (lum %.0f, bg 80)", ink, l)
	}
}

func TestInkForNilIsAutoContrast(t *testing.T) {
	dark := fillRGBA(20, 20, color.RGBA{10, 10, 10, 255})
	ink, shadow := inkFor(dark, dark.Bounds(), nil)
	aInk, aShadow := AutoContrastColor(dark, dark.Bounds())
	if ink != aInk || shadow != aShadow {
		t.Fatalf("nil want must equal AutoContrastColor: got (%v,%v) want (%v,%v)", ink, shadow, aInk, aShadow)
	}
}

func TestAlignX(t *testing.T) {
	if x := alignX(AnchorLowerLeft, 10, 100, 40); x != 10 {
		t.Errorf("left align = %d, want 10", x)
	}
	if x := alignX(AnchorCenter, 10, 100, 40); x != 40 {
		t.Errorf("center align = %d, want 40", x)
	}
	if x := alignX(AnchorLowerRight, 10, 100, 40); x != 70 {
		t.Errorf("right align = %d, want 70", x)
	}
}

func TestBlockRightAnchorRenders(t *testing.T) {
	bg := color.RGBA{0, 0, 0, 255}
	img := fillRGBA(240, 240, bg)
	Block(img, "TITLE", "sub", func(float64) font.Face { return basicfont.Face7x13 },
		18, AnchorLowerRight, 12, 1, nil)
	if nonBackground(img, bg) == 0 {
		t.Fatal("lower-right anchor drew nothing")
	}
}

func TestLeastBusyAnchorPicksFlatRegion(t *testing.T) {
	// Flat gray everywhere, then make the LEFT half busy with alternating
	// black/white bars (high luminance variance).
	img := fillRGBA(240, 240, color.RGBA{120, 120, 120, 255})
	for y := 0; y < 240; y++ {
		for x := 0; x < 120; x++ {
			c := color.RGBA{0, 0, 0, 255}
			if (x/4)%2 == 0 {
				c = color.RGBA{255, 255, 255, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	faceAt := func(float64) font.Face { return basicfont.Face7x13 }
	// Lower-left box sits over the busy half, lower-right over the flat half.
	got := LeastBusyAnchor(img, "TITLE", "sub", faceAt, 18, 12,
		[]Anchor{AnchorLowerLeft, AnchorLowerRight})
	if got != AnchorLowerRight {
		t.Fatalf("LeastBusyAnchor = %v, want AnchorLowerRight (the flat side)", got)
	}
}

func TestLeastBusyAnchorEdgeCases(t *testing.T) {
	img := fillRGBA(240, 240, color.RGBA{50, 50, 50, 255})
	faceAt := func(float64) font.Face { return basicfont.Face7x13 }
	if got := LeastBusyAnchor(img, "A", "", faceAt, 18, 12, nil); got != AnchorLowerLeft {
		t.Errorf("no candidates = %v, want AnchorLowerLeft", got)
	}
	if got := LeastBusyAnchor(img, "A", "", faceAt, 18, 12, []Anchor{AnchorCenter}); got != AnchorCenter {
		t.Errorf("single candidate = %v, want AnchorCenter", got)
	}
}

func TestColorSpreadFlatVsMultiColor(t *testing.T) {
	flat := fillRGBA(20, 20, color.RGBA{30, 200, 120, 255})
	if s := colorSpread(flat, flat.Bounds()); s > 1 {
		t.Fatalf("flat single-colour region colorSpread = %.2f, want ~0", s)
	}
	// Half one strong colour, half another: clearly "more than one colour".
	split := fillRGBA(20, 20, color.RGBA{20, 20, 20, 255})
	for y := 0; y < 20; y++ {
		for x := 10; x < 20; x++ {
			split.SetRGBA(x, y, color.RGBA{230, 40, 200, 255})
		}
	}
	if s := colorSpread(split, split.Bounds()); s < 40 {
		t.Fatalf("two-colour region colorSpread = %.2f, want large (>40)", s)
	}
}

func TestColorSpreadCatchesPureHueSplit(t *testing.T) {
	// A red/blue split is a strong colour change but its green channel (which
	// dominates perceptual luminance) never moves. colorSpread must still see it
	// clearly — that's the point of measuring colour, not luminance.
	img := fillRGBA(20, 20, color.RGBA{255, 0, 0, 255})
	for y := 0; y < 20; y++ {
		for x := 10; x < 20; x++ {
			img.SetRGBA(x, y, color.RGBA{0, 0, 255, 255})
		}
	}
	if s := colorSpread(img, img.Bounds()); s < 40 {
		t.Fatalf("red/blue split colorSpread = %.2f, want large (>40)", s)
	}
}

func TestPickSizeAnchorPrefersCalmAnchorAtLargestSize(t *testing.T) {
	sizes := []float64{40, 30, 20} // preference order: largest first
	// LowerLeft is calm at every size; UpperLeft never is. No need to shrink.
	spreadAt := func(_ float64, a Anchor) float64 {
		if a == AnchorLowerLeft {
			return 5
		}
		return 100
	}
	a, px := pickSizeAnchor(sizes, []Anchor{AnchorUpperLeft, AnchorLowerLeft}, spreadAt, 10)
	if a != AnchorLowerLeft || px != 40 {
		t.Fatalf("got (%v, %.0f), want (AnchorLowerLeft, 40): relocate to the calm anchor at the largest size, no shrink", a, px)
	}
}

func TestPickSizeAnchorShrinksWhenLargeSizesBusy(t *testing.T) {
	sizes := []float64{40, 30, 20}
	// Every anchor is busy until the block shrinks to 20.
	spreadAt := func(px float64, _ Anchor) float64 {
		if px <= 20 {
			return 5
		}
		return 100
	}
	_, px := pickSizeAnchor(sizes, []Anchor{AnchorUpperLeft, AnchorLowerLeft}, spreadAt, 10)
	if px != 20 {
		t.Fatalf("got px %.0f, want 20: must shrink to the first (largest) size that clears the threshold", px)
	}
}

func TestPickSizeAnchorFallsBackToCalmest(t *testing.T) {
	sizes := []float64{40, 30, 20}
	// Nothing clears threshold 10; the lowest spread anywhere is LowerLeft at 20.
	spreadAt := func(px float64, a Anchor) float64 {
		switch {
		case a == AnchorLowerLeft && px == 20:
			return 15
		case a == AnchorLowerLeft:
			return 25
		default:
			return 50
		}
	}
	a, px := pickSizeAnchor(sizes, []Anchor{AnchorUpperLeft, AnchorLowerLeft}, spreadAt, 10)
	if a != AnchorLowerLeft || px != 20 {
		t.Fatalf("got (%v, %.0f), want (AnchorLowerLeft, 20): with nothing under threshold, return the lowest-spread (anchor, size)", a, px)
	}
}

func TestFitPlacementRelocatesToFlatRegion(t *testing.T) {
	img := fillRGBA(240, 240, color.RGBA{120, 120, 120, 255})
	// Busy left half: two strong colours interleaved.
	for y := 0; y < 240; y++ {
		for x := 0; x < 120; x++ {
			c := color.RGBA{0, 0, 0, 255}
			if (x/4)%2 == 0 {
				c = color.RGBA{255, 40, 200, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	faceAt := func(float64) font.Face { return basicfont.Face7x13 }
	a, px := FitPlacement(img, "TITLE", "sub", faceAt, 18, 12, 0.6, 20,
		[]Anchor{AnchorLowerLeft, AnchorLowerRight})
	if a != AnchorLowerRight {
		t.Fatalf("FitPlacement anchor = %v, want AnchorLowerRight (the flat half)", a)
	}
	if px != 18 {
		t.Fatalf("FitPlacement px = %.1f, want 18 (a calm spot exists at full size, so no shrink)", px)
	}
}

func TestFitPlacementNoCandidates(t *testing.T) {
	img := fillRGBA(240, 240, color.RGBA{50, 50, 50, 255})
	faceAt := func(float64) font.Face { return basicfont.Face7x13 }
	if a, px := FitPlacement(img, "A", "", faceAt, 18, 12, 0.6, 20, nil); a != AnchorLowerLeft || px != 18 {
		t.Errorf("no candidates = (%v, %.0f), want (AnchorLowerLeft, 18)", a, px)
	}
}
