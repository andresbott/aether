package covergen

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen/internal/paint"
	"github.com/andresbott/aether/libs/covergen/internal/text"
)

// DrawTextOverlay is the shared album-title overlay every style draws through. It
// reads the text.* knobs (see TextOverlayKnobs), varies the title size and opacity
// per seed, auto-places the block over the calmest region of the finished cover
// (text.LeastBusyAnchor, so type avoids busy, high-contrast detail), and inks it
// in auto black/white or a palette-derived tint. baseFrac is the style's title
// height as a fraction of the canvas width; anchors is the style's roam order —
// index 0 is the placement used when text.roam is 0, and a higher roam widens the
// search toward the later, more adventurous spots. cs is the cover's ColorSet (see
// Colored); a style that does not implement Colored passes the zero value and
// simply never tints. It draws on the same 2x canvas as Draw and consumes rng
// (size, opacity, saturation, ink — in that order), only on the non-empty-text path.
func DrawTextOverlay(img *image.RGBA, rng *rand.Rand, ks KnobSet, t Text, f Font, cs ColorSet, baseFrac float64, anchors []text.Anchor, opts ...TextOption) {
	var o textOptions
	for _, apply := range opts {
		apply(&o)
	}
	spread := ks.Float(TextSizeSpreadKnobName)
	base := float64(img.Bounds().Dx()) * baseFrac * ks.Float(TextScaleKnobName)
	px := sizeJitter(rng, spread, base)
	opacity := jitterOpacity(rng, ks.Float(TextOpacityKnobName), ks.Float(TextOpacitySpreadKnobName))
	sat := saturationJitter(rng, ks.Float(TextSaturationKnobName), ks.Float(TextSaturationSpreadKnobName))
	ink := pickInk(rng, ks.Float(TextTintKnobName), sat, cs)
	candidates := anchors[:poolSize(ks.Float(TextRoamKnobName), len(anchors))]

	// The outline halo extends beyond the text block, so its margin must clear the
	// halo or a thick border would clip at the canvas edge. marginFrac stays at the
	// shared default for every non-outline style, so their placement is unchanged.
	marginFrac := textMarginFrac
	if o.outline != nil {
		if hf := outlineHaloFrac(o.outline); hf > marginFrac {
			marginFrac = hf
		}
	}
	var anchor text.Anchor
	if o.colorFit {
		// Search size x anchor for a single-colour spot, shrinking to minTextPx
		// as a fallback (see text.FitPlacement). Consumes no rng, so draw order
		// is unchanged from the fixed-size path.
		anchor, px = text.FitPlacement(img, t.Main, t.Subtitle, f.Face, px, minTextPx(base, spread), marginFrac, colorFitThreshold, candidates)
	} else {
		anchor = text.LeastBusyAnchor(img, t.Main, t.Subtitle, f.Face, px, int(px*marginFrac), candidates)
	}
	margin := int(px * marginFrac)
	if o.outline != nil {
		text.BlockOutline(img, t.Main, t.Subtitle, f.Face, px, anchor, margin, opacity, buildOutline(o.outline, px, ink))
		return
	}
	text.Block(img, t.Main, t.Subtitle, f.Face, px, anchor, margin, opacity, ink)
}

// TextOption configures optional DrawTextOverlay behaviour. The zero set is the
// default fixed-size, least-busy-anchor placement, so a style that passes no
// option renders exactly as before.
type TextOption func(*textOptions)

type textOptions struct {
	colorFit bool
	outline  *outlineSpec
}

// WithColorFit enables shrink-or-relocate placement: when the title would land on
// a region with more than one colour, DrawTextOverlay shrinks it — down to the
// size floor implied by text.scale and text.sizeSpread (see minTextPx) — and/or
// relocates it to a single-colour spot (see text.FitPlacement). Opt-in per style;
// the other styles keep the fixed-size least-busy-anchor placement.
func WithColorFit() TextOption {
	return func(o *textOptions) { o.colorFit = true }
}

// outlineSpec is the resolved thick-border request from WithOutline: the width of the
// single white border as a fraction of the title px. DrawTextOverlay turns it into a
// text.Outline (see buildOutline).
type outlineSpec struct {
	width float64
}

// WithOutline wraps the title in a single thick white border (see text.Outline) instead
// of a plain inked glyph — the retro / 1970s look the rings style uses — widthFrac*titlePx
// wide. Opt-in per style; a caller that wants plain text simply omits the option.
func WithOutline(widthFrac float64) TextOption {
	return func(o *textOptions) { o.outline = &outlineSpec{width: widthFrac} }
}

// buildOutline turns an outlineSpec into a text.Outline for a title at px height: a
// single white band widthFrac*px wide around a fill that is the chosen tint ink when the
// overlay picked one (darkened to read against the white) else near-black. Deterministic;
// consumes no rng.
func buildOutline(spec *outlineSpec, px float64, ink *color.NRGBA) text.Outline {
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	band := text.OutlineBand{Thickness: spec.width * px, Color: white}
	return text.Outline{Fill: outlineFill(ink, color.RGBA{R: 255, G: 255, B: 255, A: 255}), Bands: []text.OutlineBand{band}}
}

// outlineHaloFrac is how far the outline's halo extends beyond the text block as a
// fraction of the title px: the band width plus a small gap so the outer edge does not
// sit flush against the canvas.
func outlineHaloFrac(spec *outlineSpec) float64 {
	return spec.width + 0.05
}

// outlineFillContrast is the minimum luminance gap (0..255) the glyph fill must keep
// from the inner band, so the letter core never washes into its own border. It is
// larger than internal/text's minInkContrast (64) because the fill is fully surrounded
// by the band and so needs a stronger separation to read.
const outlineFillContrast = 90

// outlineFill picks the glyph-interior colour for an outlined title and guarantees it
// contrasts the innermost band by at least outlineFillContrast. It
// starts from the tint ink when the overlay chose one (keeping its hue) else a
// near-white/near-black neutral, then pushes the lightness away from the band until
// the gap is cleared — so a tint that happens to match the band's lightness still
// reads. Deterministic.
func outlineFill(ink *color.NRGBA, inner color.RGBA) color.NRGBA {
	innerLum := paint.Luminance(inner)
	base := color.NRGBA{R: 245, G: 245, B: 245, A: 255}
	switch {
	case ink != nil:
		base = *ink
	case innerLum >= 140:
		base = color.NRGBA{R: 20, G: 20, B: 20, A: 255}
	}
	return ensureContrast(base, innerLum, outlineFillContrast)
}

// ensureContrast keeps want's hue but blends its lightness toward white (over a dark
// reference) or black (over a light one) just enough to sit minGap luminance beyond
// refLum; a colour already that far from refLum is returned unchanged. It mirrors
// internal/text.contrastInk — the blend that reaches a target luminance is exact
// because luminance is linear in the channels.
func ensureContrast(want color.NRGBA, refLum, minGap float64) color.NRGBA {
	wl := paint.Luminance(color.RGBA{want.R, want.G, want.B, 255})
	if math.Abs(wl-refLum) >= minGap {
		return want
	}
	if refLum < 128 { // dark band -> lighten the fill toward white
		target := math.Min(refLum+minGap, 250)
		if wl >= target {
			return want
		}
		c := paint.LerpRGBA(color.RGBA{want.R, want.G, want.B, 255}, color.RGBA{255, 255, 255, 255}, (target-wl)/(255-wl))
		return color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255}
	}
	// light band -> darken the fill toward black
	target := math.Max(refLum-minGap, 5)
	if wl <= target {
		return want
	}
	c := paint.LerpRGBA(color.RGBA{want.R, want.G, want.B, 255}, color.RGBA{0, 0, 0, 255}, (wl-target)/wl)
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255}
}

// textMarginFrac is the block margin as a fraction of the title px. It is shared
// by the placement search and Block so a candidate is measured where it draws.
const textMarginFrac = 0.6

// colorFitThreshold is the mean per-channel colour spread (see text.colorSpread,
// 0..255) at or below which a region counts as effectively one colour for
// WithColorFit. Tuned on bauhaus's neon grid: a title within one cell (only a
// minor motif edge under it) stays put and full-size; one straddling two cells
// shrinks or relocates.
const colorFitThreshold = 32

// minTextPx is the smallest title height WithColorFit will shrink to: the low end
// of sizeJitter's per-seed range, base*(1-0.25*spread), so the floor is never
// smaller than a size the jitter could already have drawn. base is
// width*baseFrac*text.scale and spread is text.sizeSpread; clamped to sizeJitter's
// same 6px floor.
func minTextPx(base, spread float64) float64 {
	m := base * (1 - 0.25*spread)
	if m < 6 {
		m = 6
	}
	return m
}

// poolSize maps a 0..1 fraction to an inclusive 1..n count (1 at <=0, n at >=1),
// so a knob at 0 pins the first option and higher values widen the pool. It sizes
// both the tint ink pool and the placement candidate set, so turning either
// variety knob (text.tint / text.roam) reliably changes the output — satisfying
// the knob-wiring guard.
func poolSize(frac float64, n int) int {
	switch {
	case frac <= 0:
		return 1
	case frac >= 1:
		return n
	}
	k := 1 + int(math.Round(frac*float64(n-1)))
	if k < 1 {
		k = 1
	}
	if k > n {
		k = n
	}
	return k
}

// sizeJitter scales basePx by a per-seed factor; text.sizeSpread 1 is about
// +/-25%, 0 is none. It consumes one rng draw regardless, keeping the draw order
// stable.
func sizeJitter(rng *rand.Rand, spread, basePx float64) float64 {
	px := basePx * (1 + (rng.Float64()-0.5)*0.5*spread)
	if px < 6 {
		px = 6
	}
	return px
}

// jitterOpacity varies the text opacity per seed around center by
// text.opacitySpread (1 is about +/-25%), clamped to [0,1]. It consumes one rng
// draw regardless, keeping the draw order stable.
func jitterOpacity(rng *rand.Rand, center, spread float64) float64 {
	return paint.ClampFloat(center*(1+(rng.Float64()-0.5)*0.5*spread), 0, 1)
}

// saturationJitter varies the tint colour's saturation per seed around center by
// text.saturationSpread (1 is about +/-25%), clamped to [0,1]. It consumes one
// rng draw regardless, keeping the draw order stable.
func saturationJitter(rng *rand.Rand, center, spread float64) float64 {
	return paint.ClampFloat(center*(1+(rng.Float64()-0.5)*0.5*spread), 0, 1)
}

// pickInk chooses the overlay ink from a tint-sized pool ([nil auto, tint, tint]);
// a higher text.tint mixes in more palette colour. sat is the tint colour's
// saturation (see saturationJitter). It consumes one rng draw.
func pickInk(rng *rand.Rand, tint, sat float64, cs ColorSet) *color.NRGBA {
	inks := textInks(cs, sat)[:poolSize(tint, 3)]
	return inks[rng.IntN(len(inks))]
}

// textInks is the ink pool in tint order: auto black/white first (nil), then the
// tint colour twice, so a higher text.tint colours more seeds. The tint colour is
// the COMPLEMENT of the palette's two accents (a distinct hue) at saturation sat,
// so it reads apart from accent-based artwork; internal/text.inkFor keeps that hue
// while adapting its lightness for local legibility.
func textInks(cs ColorSet, sat float64) []*color.NRGBA {
	c := contrastColor(cs, sat)
	return []*color.NRGBA{nil, c, c}
}

// contrastColor is an ink whose hue is the complement of the palette's two accents'
// average hue (so it stands apart from accent-based artwork), at saturation sat and
// mid lightness. Its lightness is then pushed for local contrast by
// internal/text.inkFor.
func contrastColor(cs ColorSet, sat float64) *color.NRGBA {
	h1, _, _ := paint.RgbToHsl(cs.Accent1)
	h2, _, _ := paint.RgbToHsl(cs.Accent2)
	c := paint.Hsl(averageHue(h1, h2)+180, sat, 0.5)
	return &color.NRGBA{R: c.R, G: c.G, B: c.B, A: 255}
}

// averageHue is the circular mean of two hues in degrees.
func averageHue(a, b float64) float64 {
	ar, br := a*math.Pi/180, b*math.Pi/180
	h := math.Atan2(math.Sin(ar)+math.Sin(br), math.Cos(ar)+math.Cos(br)) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return h
}
