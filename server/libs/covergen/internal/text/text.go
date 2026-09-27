// Package text rasterizes short strings onto covergen's RGBA canvases. It is
// deliberately covergen-agnostic (it takes font faces and a face factory, not
// covergen types), so styles compose it however they place their overlay.
package text

import (
	"image"
	"image/color"
	"math"

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
	AnchorLowerRight
	AnchorUpperRight
)

// DrawString draws s in face and col with its baseline origin at (x, y), over
// img, returning the advance width. It wraps font.Drawer, which composites the
// glyph coverage mask with draw.Over — anti-aliased, and alpha-blended when
// col's alpha is translucent. col is typed as the color.Color interface (not
// color.RGBA) so callers can pass a non-premultiplied color.NRGBA, whose
// RGBA() method premultiplies correctly; a color.RGBA with RGB > A is already
// invalid premultiplied data and would composite wrong.
func DrawString(img *image.RGBA, face font.Face, s string, x, y int, col color.Color) int {
	d := font.Drawer{Dst: img, Src: image.NewUniform(col), Face: face, Dot: fixed.P(x, y)}
	d.DrawString(s)
	return (d.Dot.X - fixed.I(x)).Round()
}

// Measure returns the advance width and the face ascent/descent (px) of s.
func Measure(face font.Face, s string) (w, ascent, descent int) {
	m := face.Metrics()
	return font.MeasureString(face, s).Round(), m.Ascent.Round(), m.Descent.Round()
}

// meanLuminance is the average perceptual luminance (0..255) of img under box.
func meanLuminance(img *image.RGBA, box image.Rectangle) float64 {
	box = box.Intersect(img.Bounds())
	var sum float64
	var n int
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			sum += paint.Luminance(img.RGBAAt(x, y))
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// AutoContrastColor samples the mean luminance of img under box and returns a
// near-black or near-white ink that reads against it, plus a translucent shadow
// of the opposite polarity. Both are color.NRGBA (non-premultiplied): the
// shadow's alpha < 255 must survive as a genuinely translucent composite, and
// color.RGBA's premultiplied convention (RGB must be <= A) can't represent
// that — {255,255,255,170} is invalid premultiplied data and would render
// opaque. Deterministic; consumes no rng.
func AutoContrastColor(img *image.RGBA, box image.Rectangle) (ink, shadow color.NRGBA) {
	if meanLuminance(img, box) < 128 {
		return color.NRGBA{245, 245, 245, 255}, color.NRGBA{0, 0, 0, 170}
	}
	return color.NRGBA{15, 15, 15, 255}, color.NRGBA{255, 255, 255, 170}
}

// minInkContrast is the smallest luminance gap (0..255) a caller-picked ink must
// have from the mean background to be used instead of the auto black/white pair.
const minInkContrast = 64

// inkFor returns the overlay ink + shadow. A nil want gives the auto black/white
// pair. A non-nil want keeps its HUE but has its lightness pushed toward white or
// black just enough to clear minInkContrast over box's mean luminance — so a
// palette colour renders colourfully AND legibly rather than washing out or
// falling back to grey. The shadow is the opposite polarity of the final ink, a
// legibility halo. Deterministic; consumes no rng.
func inkFor(img *image.RGBA, box image.Rectangle, want *color.NRGBA) (ink, shadow color.NRGBA) {
	if want == nil {
		return AutoContrastColor(img, box)
	}
	ink = contrastInk(*want, meanLuminance(img, box))
	if paint.Luminance(color.RGBA{ink.R, ink.G, ink.B, 255}) < 128 {
		return ink, color.NRGBA{255, 255, 255, 170}
	}
	return ink, color.NRGBA{0, 0, 0, 170}
}

// contrastInk keeps want's hue but blends it toward white (over a dark box) or
// black (over a light one) just enough to sit minInkContrast beyond the mean
// background luminance. Luminance is a linear function of the channels, so the
// blend factor that reaches a target luminance is exact. want already at or past
// the target is returned unchanged.
func contrastInk(want color.NRGBA, mean float64) color.NRGBA {
	wl := paint.Luminance(color.RGBA{want.R, want.G, want.B, 255})
	if mean < 128 { // dark background -> lighten toward white
		target := math.Min(mean+minInkContrast, 245)
		if wl >= target {
			return want
		}
		return lerpNRGBA(want, color.NRGBA{255, 255, 255, 255}, (target-wl)/(255-wl))
	}
	// light background -> darken toward black
	target := math.Max(mean-minInkContrast, 10)
	if wl <= target {
		return want
	}
	return lerpNRGBA(want, color.NRGBA{0, 0, 0, 255}, (wl-target)/wl)
}

// lerpNRGBA linearly blends two opaque colours.
func lerpNRGBA(a, b color.NRGBA, t float64) color.NRGBA {
	c := paint.LerpRGBA(color.RGBA{a.R, a.G, a.B, 255}, color.RGBA{b.R, b.G, b.B, 255}, t)
	return color.NRGBA{c.R, c.G, c.B, 255}
}

// blockLayout is the fitted geometry of a two-line overlay: the faces at the
// shrunk size plus the per-line and total dimensions. Block and LeastBusyAnchor
// share it so the box that gets measured is exactly the box that gets drawn.
type blockLayout struct {
	mf, sf           font.Face
	mAscent, sAscent int
	mainH, mainW     int
	subH, subW       int
	gap              int
	blockW, blockH   int
}

// layoutBlock shrinks mainPx until both lines fit maxW, then measures the block.
func layoutBlock(main, sub string, faceAt func(pxHeight float64) font.Face, mainPx float64, maxW int) blockLayout {
	if mainPx < 1 {
		mainPx = 1
	}
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

	lay := blockLayout{mf: mf, sf: sf, mAscent: mm.Ascent.Round(), sAscent: sm.Ascent.Round()}
	if main != "" {
		lay.mainH = (mm.Ascent + mm.Descent).Round()
		lay.mainW = font.MeasureString(mf, main).Round()
	}
	if sub != "" {
		lay.subH = (sm.Ascent + sm.Descent).Round()
		lay.subW = font.MeasureString(sf, sub).Round()
	}
	if main != "" && sub != "" {
		lay.gap = int(mainPx * 0.15)
	}
	lay.blockW = lay.mainW
	if lay.subW > lay.blockW {
		lay.blockW = lay.subW
	}
	lay.blockH = lay.mainH + lay.gap + lay.subH
	return lay
}

// Block draws a two-line overlay (main above a smaller subtitle) anchored within
// img. It shrinks mainPx until the wider line fits the canvas minus margins,
// inks the text in wantInk when it reads clearly (nil = auto black/white),
// pairs it with a contrasting shadow, and applies opacity. faceAt builds a face
// at a pixel height (a style passes its Font's Face method).
func Block(img *image.RGBA, main, sub string, faceAt func(pxHeight float64) font.Face, mainPx float64, anchor Anchor, marginPx int, opacity float64, wantInk *color.NRGBA) {
	if main == "" && sub == "" {
		return
	}
	lay := layoutBlock(main, sub, faceAt, mainPx, img.Bounds().Dx()-2*marginPx)

	ox, oy := anchorOrigin(anchor, img.Bounds(), lay.blockW, lay.blockH, marginPx)
	ink, shadow := inkFor(img, image.Rect(ox, oy, ox+lay.blockW, oy+lay.blockH), wantInk)
	ink = withAlpha(ink, opacity)
	shadow = withAlpha(shadow, opacity)

	y := oy
	if main != "" {
		drawLine(img, lay.mf, main, ox, y+lay.mAscent, ink, shadow, anchor, lay.blockW, lay.mainW)
		y += lay.mainH + lay.gap
	}
	if sub != "" {
		drawLine(img, lay.sf, sub, ox, y+lay.sAscent, ink, shadow, anchor, lay.blockW, lay.subW)
	}
}

// LeastBusyAnchor returns the candidate whose text box covers the flattest
// (lowest luminance-variance) area of img, so the overlay avoids busy, high-
// contrast regions. It lays the block out once — so every candidate is measured
// at the size Block will draw — and keeps the earliest candidate on ties. With no
// candidates it returns AnchorLowerLeft. Deterministic; consumes no rng.
func LeastBusyAnchor(img *image.RGBA, main, sub string, faceAt func(pxHeight float64) font.Face, mainPx float64, marginPx int, candidates []Anchor) Anchor {
	if len(candidates) == 0 {
		return AnchorLowerLeft
	}
	lay := layoutBlock(main, sub, faceAt, mainPx, img.Bounds().Dx()-2*marginPx)
	best := candidates[0]
	bestVar := math.Inf(1)
	for _, a := range candidates {
		ox, oy := anchorOrigin(a, img.Bounds(), lay.blockW, lay.blockH, marginPx)
		if v := luminanceVariance(img, image.Rect(ox, oy, ox+lay.blockW, oy+lay.blockH)); v < bestVar {
			bestVar = v
			best = a
		}
	}
	return best
}

// FitPlacement chooses both where and how large to draw the block so it lands on
// a single-colour region. It walks the main-text height from startPx down to
// minPx (a few steps) and, at each size, takes the candidate anchor whose box has
// the lowest colour spread (see colorSpread). It returns the largest size whose
// best anchor is at or below threshold — relocating first and shrinking only when
// no candidate is calm at the current size — and otherwise the lowest-spread
// (anchor, size) seen, so a busy cover still gets the least-bad placement. The
// margin is recomputed per size as int(px*marginFrac), matching Block. With no
// candidates it returns (AnchorLowerLeft, startPx). Deterministic; consumes no rng.
func FitPlacement(img *image.RGBA, main, sub string, faceAt func(pxHeight float64) font.Face, startPx, minPx, marginFrac, threshold float64, candidates []Anchor) (Anchor, float64) {
	if len(candidates) == 0 {
		return AnchorLowerLeft, startPx
	}
	spreadAt := func(px float64, a Anchor) float64 {
		marginPx := int(px * marginFrac)
		lay := layoutBlock(main, sub, faceAt, px, img.Bounds().Dx()-2*marginPx)
		ox, oy := anchorOrigin(a, img.Bounds(), lay.blockW, lay.blockH, marginPx)
		return colorSpread(img, image.Rect(ox, oy, ox+lay.blockW, oy+lay.blockH))
	}
	return pickSizeAnchor(sizeSteps(startPx, minPx, fitSizeSteps), candidates, spreadAt, threshold)
}

// fitSizeSteps is how many sizes FitPlacement tries between startPx and minPx
// (inclusive): a handful is enough to find a calm spot without re-measuring the
// cover many times.
const fitSizeSteps = 5

// sizeSteps returns n sizes from start down to min inclusive, largest first. It
// collapses to a single size when n <= 1 or start <= min.
func sizeSteps(start, min float64, n int) []float64 {
	if n <= 1 || start <= min {
		return []float64{start}
	}
	out := make([]float64, n)
	for i := range out {
		t := float64(i) / float64(n-1)
		out[i] = start + (min-start)*t
	}
	return out
}

// pickSizeAnchor scans sizes in preference order (largest first) and, at each
// size, picks the candidate anchor with the lowest spreadAt. It returns the first
// (largest) size whose best anchor is at or below threshold; if none qualifies it
// returns the lowest-spread (anchor, size) seen across every size. candidates must
// be non-empty and sizes must have at least one entry.
func pickSizeAnchor(sizes []float64, candidates []Anchor, spreadAt func(px float64, a Anchor) float64, threshold float64) (Anchor, float64) {
	bestA, bestPx := candidates[0], sizes[0]
	bestSpread := math.Inf(1)
	for _, px := range sizes {
		aBest, aSpread := candidates[0], math.Inf(1)
		for _, a := range candidates {
			if s := spreadAt(px, a); s < aSpread {
				aSpread, aBest = s, a
			}
		}
		if aSpread <= threshold {
			return aBest, px
		}
		if aSpread < bestSpread {
			bestSpread, bestA, bestPx = aSpread, aBest, px
		}
	}
	return bestA, bestPx
}

// colorSpread is the mean of the three per-channel population standard deviations
// (0..255) of img under box — ~0 over a flat single colour and large where the
// box straddles distinct colours. Unlike luminanceVariance it moves for a pure
// hue change (two colours of equal luminance), so it answers "more than one
// colour here?" for the placement search.
func colorSpread(img *image.RGBA, box image.Rectangle) float64 {
	box = box.Intersect(img.Bounds())
	var sr, sg, sb, sr2, sg2, sb2 float64
	var n int
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			p := img.RGBAAt(x, y)
			r, g, b := float64(p.R), float64(p.G), float64(p.B)
			sr += r
			sg += g
			sb += b
			sr2 += r * r
			sg2 += g * g
			sb2 += b * b
			n++
		}
	}
	if n == 0 {
		return 0
	}
	std := func(sum, sumsq float64) float64 {
		v := sumsq/float64(n) - (sum/float64(n))*(sum/float64(n))
		if v < 0 {
			v = 0 // guard tiny negative from float rounding
		}
		return math.Sqrt(v)
	}
	return (std(sr, sr2) + std(sg, sg2) + std(sb, sb2)) / 3
}

// luminanceVariance is the population variance of perceptual luminance under box
// — near 0 over a flat fill, larger across edges and busy detail.
func luminanceVariance(img *image.RGBA, box image.Rectangle) float64 {
	box = box.Intersect(img.Bounds())
	var sum, sumSq float64
	var n int
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			l := paint.Luminance(img.RGBAAt(x, y))
			sum += l
			sumSq += l * l
			n++
		}
	}
	if n == 0 {
		return 0
	}
	mean := sum / float64(n)
	return sumSq/float64(n) - mean*mean
}

// drawLine draws one line at baseline by, offsetting x for centered anchors and
// laying a 1px shadow under the ink.
func drawLine(img *image.RGBA, face font.Face, s string, ox, by int, ink, shadow color.NRGBA, anchor Anchor, blockW, lineW int) {
	x := alignX(anchor, ox, blockW, lineW)
	DrawString(img, face, s, x+1, by+1, shadow)
	DrawString(img, face, s, x, by, ink)
}

// alignX offsets a lineW-wide line within a blockW-wide block: centered for the
// center anchors, right-aligned for the right anchors, left otherwise.
func alignX(anchor Anchor, ox, blockW, lineW int) int {
	switch anchor {
	case AnchorLowerCenter, AnchorCenter:
		return ox + (blockW-lineW)/2
	case AnchorLowerRight, AnchorUpperRight:
		return ox + (blockW - lineW)
	}
	return ox
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
	case AnchorLowerRight:
		return w - marginPx - blockW, h - marginPx - blockH
	case AnchorUpperRight:
		return w - marginPx - blockW, marginPx
	}
	return marginPx, h - marginPx - blockH
}

// withAlpha scales col's alpha by opacity in [0,1]. col is color.NRGBA
// (non-premultiplied), so scaling .A alone and leaving RGB untouched is
// correct — premultiplication happens later, in col.RGBA().
func withAlpha(col color.NRGBA, opacity float64) color.NRGBA {
	if opacity < 0 {
		opacity = 0
	}
	if opacity > 1 {
		opacity = 1
	}
	col.A = uint8(float64(col.A) * opacity)
	return col
}

// Outline configures a thick multi-border "echo" treatment for a text block: each
// glyph is drawn as a solid Fill wrapped in one or more concentric colour bands of
// increasing radius (a retro / 1970s look) instead of the plain inked glyph with a
// thin drop shadow that Block draws. Bands are ordered innermost first — Bands[0]
// hugs the glyph — and each band's Thickness is its own width in px at the main line
// size (BlockOutline scales it down for the smaller subtitle line). An empty Bands
// draws nothing.
type Outline struct {
	Fill  color.NRGBA
	Bands []OutlineBand
}

// OutlineBand is one concentric border of an Outline: a colour and its width in px
// (measured outward from the previous band, at the main line size).
type OutlineBand struct {
	Thickness float64
	Color     color.NRGBA
}

// BlockOutline draws the same two-line, anchored, size-fitted block as Block but in
// the Outline "thick border" treatment (see Outline): each glyph gets a solid fill
// wrapped in concentric colour bands — the 70s look the rings style uses. It shares
// Block's layout (layoutBlock / anchorOrigin / alignX), so a block measures and
// places identically whichever treatment draws it. opacity scales every band and the
// fill uniformly. Deterministic; consumes no rng.
func BlockOutline(img *image.RGBA, main, sub string, faceAt func(pxHeight float64) font.Face, mainPx float64, anchor Anchor, marginPx int, opacity float64, o Outline) {
	if (main == "" && sub == "") || len(o.Bands) == 0 {
		return
	}
	lay := layoutBlock(main, sub, faceAt, mainPx, img.Bounds().Dx()-2*marginPx)
	ox, oy := anchorOrigin(anchor, img.Bounds(), lay.blockW, lay.blockH, marginPx)

	y := oy
	if main != "" {
		lineX := alignX(anchor, ox, lay.blockW, lay.mainW)
		drawOutlineLine(img, lay.mf, main, lineX, y+lay.mAscent, lay.mAscent, lay.mainH, lay.mainW, o, 1, opacity)
		y += lay.mainH + lay.gap
	}
	if sub != "" {
		lineX := alignX(anchor, ox, lay.blockW, lay.subW)
		drawOutlineLine(img, lay.sf, sub, lineX, y+lay.sAscent, lay.sAscent, lay.subH, lay.subW, o, subRatio, opacity)
	}
}

// drawOutlineLine renders one Outline line: it stamps the glyph run into an offscreen
// coverage mask once, computes each pixel's distance to the nearest glyph pixel, then
// paints the fill (on the glyph) and each band (out to its cumulative radius) onto img
// at opacity. scale shrinks the band widths for the subtitle line. Cost is O(mask
// area) — independent of band thickness — rather than re-stamping the glyph per pixel.
func drawOutlineLine(img *image.RGBA, face font.Face, s string, lineX, baselineY, ascent, lineH, lineW int, o Outline, scale, opacity float64) {
	if s == "" || lineW <= 0 || lineH <= 0 {
		return
	}
	a := uint8(math.Round(paint.ClampFloat(opacity, 0, 1) * 255))
	if a == 0 {
		return
	}

	// Cumulative outer radii per band (scaled x3 to match the chamfer metric below).
	thr := make([]int32, len(o.Bands))
	var cum float64
	for i, band := range o.Bands {
		cum += band.Thickness * scale
		thr[i] = int32(math.Round(cum * 3))
	}
	pad := int(math.Ceil(cum)) + 2

	mw, mh := lineW+2*pad, lineH+2*pad
	mask := image.NewAlpha(image.Rect(0, 0, mw, mh))
	d := font.Drawer{Dst: mask, Src: image.NewUniform(color.Alpha{A: 255}), Face: face, Dot: fixed.P(pad, pad+ascent)}
	d.DrawString(s)
	dist := chamferDist(mask)

	fill := color.RGBA{o.Fill.R, o.Fill.G, o.Fill.B, a}
	bands := make([]color.RGBA, len(o.Bands))
	for i, band := range o.Bands {
		bands[i] = color.RGBA{band.Color.R, band.Color.G, band.Color.B, a}
	}

	imgX0, imgY0 := lineX-pad, baselineY-ascent-pad
	bnds := img.Bounds()
	for my := 0; my < mh; my++ {
		iy := imgY0 + my
		if iy < bnds.Min.Y || iy >= bnds.Max.Y {
			continue
		}
		for mx := 0; mx < mw; mx++ {
			g := dist[my*mw+mx]
			c := fill
			if g != 0 { // not on the glyph: find the innermost band that reaches this pixel
				idx := -1
				for i, t := range thr {
					if g <= t {
						idx = i
						break
					}
				}
				if idx < 0 {
					continue // beyond the outermost band
				}
				c = bands[idx]
			}
			ix := imgX0 + mx
			if ix < bnds.Min.X || ix >= bnds.Max.X {
				continue
			}
			paint.BlendPixel(img, ix, iy, c)
		}
	}
}

// chamferDist returns, for every pixel of mask, an approximate Euclidean distance
// (scaled x3, integer) to the nearest covered pixel (alpha >= 128); covered pixels
// are 0. It is a two-pass 3-4 chamfer, so a value v means a distance of about v/3 px
// — accurate enough to band thick decorative borders and O(area) rather than a
// per-pixel search.
func chamferDist(mask *image.Alpha) []int32 {
	b := mask.Bounds()
	w, h := b.Dx(), b.Dy()
	const inf = int32(1) << 28
	d := make([]int32, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if mask.AlphaAt(b.Min.X+x, b.Min.Y+y).A >= 128 {
				d[y*w+x] = 0
			} else {
				d[y*w+x] = inf
			}
		}
	}
	min32 := func(a, b int32) int32 {
		if a < b {
			return a
		}
		return b
	}
	for y := 0; y < h; y++ { // forward pass
		for x := 0; x < w; x++ {
			v := d[y*w+x]
			if x > 0 {
				v = min32(v, d[y*w+x-1]+3)
			}
			if y > 0 {
				v = min32(v, d[(y-1)*w+x]+3)
				if x > 0 {
					v = min32(v, d[(y-1)*w+x-1]+4)
				}
				if x < w-1 {
					v = min32(v, d[(y-1)*w+x+1]+4)
				}
			}
			d[y*w+x] = v
		}
	}
	for y := h - 1; y >= 0; y-- { // backward pass
		for x := w - 1; x >= 0; x-- {
			v := d[y*w+x]
			if x < w-1 {
				v = min32(v, d[y*w+x+1]+3)
			}
			if y < h-1 {
				v = min32(v, d[(y+1)*w+x]+3)
				if x < w-1 {
					v = min32(v, d[(y+1)*w+x+1]+4)
				}
				if x > 0 {
					v = min32(v, d[(y+1)*w+x-1]+4)
				}
			}
			d[y*w+x] = v
		}
	}
	return d
}
