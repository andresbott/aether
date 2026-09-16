// Package text rasterizes short strings onto covergen's RGBA canvases. It is
// deliberately covergen-agnostic (it takes font faces and a face factory, not
// covergen types), so styles compose it however they place their overlay.
package text

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/andresbott/aether/libs/covergen/internal/paint"
)

// subRatio is the subtitle size as a fraction of the main line size.
const subRatio = 0.62

// Anchor places a text block within the canvas.
type Anchor int

const (
	AnchorLowerLeft Anchor = iota
	AnchorLowerCenter
	AnchorCenter
	AnchorUpperLeft
)

// DrawString draws s in face and col with its baseline origin at (x, y), over
// img, returning the advance width. It wraps font.Drawer, which composites the
// glyph coverage mask with draw.Over — anti-aliased, and alpha-blended when
// col.A < 255.
func DrawString(img *image.RGBA, face font.Face, s string, x, y int, col color.RGBA) int {
	d := font.Drawer{Dst: img, Src: image.NewUniform(col), Face: face, Dot: fixed.P(x, y)}
	d.DrawString(s)
	return (d.Dot.X - fixed.I(x)).Round()
}

// Measure returns the advance width and the face ascent/descent (px) of s.
func Measure(face font.Face, s string) (w, ascent, descent int) {
	m := face.Metrics()
	return font.MeasureString(face, s).Round(), m.Ascent.Round(), m.Descent.Round()
}

// AutoContrastColor samples the mean luminance of img under box and returns a
// near-black or near-white ink that reads against it, plus a translucent shadow
// of the opposite polarity. Deterministic; consumes no rng.
func AutoContrastColor(img *image.RGBA, box image.Rectangle) (ink, shadow color.RGBA) {
	box = box.Intersect(img.Bounds())
	var sum float64
	var n int
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			sum += paint.Luminance(img.RGBAAt(x, y))
			n++
		}
	}
	mean := 0.0
	if n > 0 {
		mean = sum / float64(n)
	}
	if mean < 128 {
		return color.RGBA{245, 245, 245, 255}, color.RGBA{0, 0, 0, 170}
	}
	return color.RGBA{15, 15, 15, 255}, color.RGBA{255, 255, 255, 170}
}

// Block draws a two-line overlay (main above a smaller subtitle) anchored within
// img. It shrinks mainPx until the wider line fits the canvas minus margins,
// auto-picks a contrasting ink + shadow, and applies opacity. faceAt builds a
// face at a pixel height (a style passes its Font's Face method).
func Block(img *image.RGBA, main, sub string, faceAt func(pxHeight float64) font.Face, mainPx float64, anchor Anchor, marginPx int, opacity float64) {
	if main == "" && sub == "" {
		return
	}
	if mainPx < 1 {
		mainPx = 1
	}
	maxW := img.Bounds().Dx() - 2*marginPx
	for mainPx > 6 {
		mf := faceAt(mainPx)
		sf := faceAt(mainPx * subRatio)
		mw := font.MeasureString(mf, main).Round()
		sw := font.MeasureString(sf, sub).Round()
		if (main == "" || mw <= maxW) && (sub == "" || sw <= maxW) {
			break
		}
		mainPx *= 0.9
	}
	mf := faceAt(mainPx)
	sf := faceAt(mainPx * subRatio)
	mm := mf.Metrics()
	sm := sf.Metrics()

	mainH, mainW := 0, 0
	if main != "" {
		mainH = (mm.Ascent + mm.Descent).Round()
		mainW = font.MeasureString(mf, main).Round()
	}
	subH, subW := 0, 0
	if sub != "" {
		subH = (sm.Ascent + sm.Descent).Round()
		subW = font.MeasureString(sf, sub).Round()
	}
	gap := 0
	if main != "" && sub != "" {
		gap = int(mainPx * 0.15)
	}
	blockW := mainW
	if subW > blockW {
		blockW = subW
	}
	blockH := mainH + gap + subH

	ox, oy := anchorOrigin(anchor, img.Bounds(), blockW, blockH, marginPx)
	ink, shadow := AutoContrastColor(img, image.Rect(ox, oy, ox+blockW, oy+blockH))
	ink = withAlpha(ink, opacity)
	shadow = withAlpha(shadow, opacity)

	y := oy
	if main != "" {
		drawLine(img, mf, main, ox, y+mm.Ascent.Round(), ink, shadow, anchor, blockW, mainW)
		y += mainH + gap
	}
	if sub != "" {
		drawLine(img, sf, sub, ox, y+sm.Ascent.Round(), ink, shadow, anchor, blockW, subW)
	}
}

// drawLine draws one line at baseline by, offsetting x for centered anchors and
// laying a 1px shadow under the ink.
func drawLine(img *image.RGBA, face font.Face, s string, ox, by int, ink, shadow color.RGBA, anchor Anchor, blockW, lineW int) {
	x := ox
	if anchor == AnchorLowerCenter || anchor == AnchorCenter {
		x = ox + (blockW-lineW)/2
	}
	DrawString(img, face, s, x+1, by+1, shadow)
	DrawString(img, face, s, x, by, ink)
}

// anchorOrigin returns the top-left corner of a blockW x blockH text block for
// anchor within bounds, inset by marginPx.
func anchorOrigin(anchor Anchor, bounds image.Rectangle, blockW, blockH, marginPx int) (int, int) {
	w, h := bounds.Dx(), bounds.Dy()
	switch anchor {
	case AnchorLowerLeft:
		return marginPx, h - marginPx - blockH
	case AnchorLowerCenter:
		return (w - blockW) / 2, h - marginPx - blockH
	case AnchorCenter:
		return (w - blockW) / 2, (h - blockH) / 2
	case AnchorUpperLeft:
		return marginPx, marginPx
	}
	return marginPx, h - marginPx - blockH
}

// withAlpha scales col's alpha by opacity in [0,1].
func withAlpha(col color.RGBA, opacity float64) color.RGBA {
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}
	col.A = uint8(float64(col.A) * opacity)
	return col
}
