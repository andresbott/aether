// Package lowpoly implements the covergen "lowpoly" style: a jittered triangle
// lattice whose facets are flat-filled from an accent gradient with per-facet
// jitter, for a crystalline low-poly look.
package lowpoly

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/internal/paint"
	"github.com/andresbott/aether/libs/covergen/internal/text"
)

// New returns a lowpoly style that colors itself from pal; nil uses
// covergen.DefaultPalette().
func New(pal covergen.Palette) covergen.Style {
	if pal == nil {
		pal = covergen.DefaultPalette()
	}
	return style{pal: pal}
}

// Style is the lowpoly cover-art style with the default palette.
var Style covergen.Style = New(nil)

type style struct{ pal covergen.Palette }

func (s style) Name() string { return "lowpoly" }
func (s style) Knobs() []covergen.Knob {
	out := append([]covergen.Knob(nil), lowpolyKnobs...)
	for _, k := range s.pal.Knobs() {
		if d, ok := lowpolyPaletteDefaults[k.Name]; ok {
			k.Default = d
		}
		out = append(out, k)
	}
	out = append(out, covergen.GrainKnob(7))
	// lowpoly prefers its own defaults for several shared text-overlay knobs (a small,
	// monochrome title pinned to its home anchor — baked from a lab URL), without
	// affecting the other styles.
	for _, k := range covergen.TextOverlayKnobs() {
		if d, ok := lowpolyTextDefaults[k.Name]; ok {
			k.Default = d
		}
		out = append(out, k)
	}
	return out
}

// lowpolyPaletteDefaults tunes the injected palette's knob defaults for lowpoly.
// It ships on triadic (see allstyles.New) for a bold, well-separated facet
// gradient. Keys the injected palette does not declare are ignored.
var lowpolyPaletteDefaults = map[string]float64{
	"palette.saturation":       0.95,
	"palette.saturationSpread": 3.15,
	"palette.spread":           1.7,
	"palette.spreadJitter":     0.55,
}

// lowpolyTextDefaults overrides the shared text-overlay knob defaults for lowpoly only
// (baked from a lab URL, 2026-09-17). Keys not present keep the shared house-style
// defaults (see covergen.TextOverlayKnobs).
var lowpolyTextDefaults = map[string]float64{
	covergen.TextScaleKnobName:         0.9,
	covergen.TextSizeSpreadKnobName:    1,
	covergen.TextOpacityKnobName:       1,
	covergen.TextOpacitySpreadKnobName: 1,
	covergen.TextRoamKnobName:          0,
	covergen.TextTintKnobName:          0.25,
	covergen.TextSaturationKnobName:    0,
}

// lowpolyKnobs tune the triangle lattice. Multipliers/spreads applied after a
// per-seed random draw: density sets the grid resolution, jitter how far interior
// vertices stray from the regular grid.
var lowpolyKnobs = []covergen.Knob{
	{Name: "lowpoly.density", Label: "Facet density", Min: 0.4, Max: 3, Step: 0.05, Default: 0.5},
	{Name: "lowpoly.densitySpread", Label: "Facet density spread", Min: 0, Max: 10, Step: 0.05, Default: 4.15},
	{Name: "lowpoly.jitter", Label: "Vertex jitter", Min: 0, Max: 0.7, Step: 0.05, Default: 0.6},
}

const (
	maxGrid  = 30   // hard cap on grid resolution, bounding the triangle count
	colorJit = 0.34 // per-facet gradient jitter that keeps neighbouring facets distinct
)

// Draw builds a jittered grid of vertices (edges pinned to the canvas border so
// facets tile it flush), splits each cell into two triangles, and flat-fills each
// from an accent gradient sampled at its centroid plus a per-facet jitter — the
// jitter is what makes neighbouring facets differ, giving crisp crystalline edges
// rather than a smooth gradient.
func (s style) Draw(img *image.RGBA, rng *rand.Rand, ks covergen.KnobSet) {
	sz := img.Bounds().Dx()
	fs := float64(sz)
	densityMul := ks.Float("lowpoly.density")
	densitySpread := ks.Float("lowpoly.densitySpread")
	jitterAmt := ks.Float("lowpoly.jitter")

	cs := s.pal.Colors(rng, ks)

	rd := rng.Float64()
	m := int(densityMul*(5+rd*5) + (densitySpread-1)*3*(rd-0.5) + 0.5)
	if m < 3 {
		m = 3
	}
	if m > maxGrid {
		m = maxGrid
	}
	cell := fs / float64(m)

	// Vertex grid (m+1 x m+1). Interior vertices jitter; border vertices stay on
	// the canvas edge so the triangles cover it with no gaps.
	type pt struct{ x, y float64 }
	pts := make([][]pt, m+1)
	for i := 0; i <= m; i++ {
		pts[i] = make([]pt, m+1)
		for j := 0; j <= m; j++ {
			x, y := float64(j)*cell, float64(i)*cell
			if i > 0 && i < m && j > 0 && j < m {
				x += (rng.Float64() - 0.5) * 2 * jitterAmt * cell
				y += (rng.Float64() - 0.5) * 2 * jitterAmt * cell
			}
			pts[i][j] = pt{x, y}
		}
	}

	// Gradient direction and its projection range over the canvas, so t normalises
	// to [0,1] across the image.
	ang := rng.Float64() * 2 * math.Pi
	gx, gy := math.Cos(ang), math.Sin(ang)
	proj := func(x, y float64) float64 { return x*gx + y*gy }
	pmin := math.Min(math.Min(proj(0, 0), proj(fs, 0)), math.Min(proj(0, fs), proj(fs, fs)))
	pmax := math.Max(math.Max(proj(0, 0), proj(fs, 0)), math.Max(proj(0, fs), proj(fs, fs)))
	span := pmax - pmin
	if span == 0 {
		span = 1
	}

	facet := func(ax, ay, bx, by, cx, cy float64) {
		gxc, gyc := (ax+bx+cx)/3, (ay+by+cy)/3
		t := (proj(gxc, gyc) - pmin) / span
		t = paint.ClampFloat(t+(rng.Float64()-0.5)*colorJit, 0, 1)
		col := paint.LerpRGBA(cs.Accent1, cs.Accent2, t)
		switch r := rng.Float64(); {
		case r < 0.14:
			col = paint.LerpRGBA(col, cs.Ink, 0.28) // occasional highlight/shadow facet
		case r > 0.86:
			col = paint.LerpRGBA(col, cs.Background, 0.30)
		}
		fillTriangle(img, ax, ay, bx, by, cx, cy, col)
	}

	for i := 0; i < m; i++ {
		for j := 0; j < m; j++ {
			a, b := pts[i][j], pts[i][j+1]
			c, d := pts[i+1][j], pts[i+1][j+1]
			if (i+j)%2 == 0 { // alternate the split diagonal for a less regular lattice
				facet(a.x, a.y, b.x, b.y, c.x, c.y)
				facet(b.x, b.y, d.x, d.y, c.x, c.y)
			} else {
				facet(a.x, a.y, b.x, b.y, d.x, d.y)
				facet(a.x, a.y, d.x, d.y, c.x, c.y)
			}
		}
	}
}

// textClasses are the classification(s) lowpoly renders its overlay in; the
// pipeline picks a font from their union per seed.
var textClasses = []covergen.FontClass{covergen.FontMono}

func (s style) TextClasses() []covergen.FontClass { return textClasses }

// Colors exposes the per-cover ColorSet (see covergen.Colored) so the shared text
// overlay can tint the title in the palette's accent complement.
func (s style) Colors(rng *rand.Rand, ks covergen.KnobSet) covergen.ColorSet {
	return s.pal.Colors(rng, ks)
}

// DrawText paints the album title + subtitle in a clean sans face via the shared
// overlay (see covergen.DrawTextOverlay); lowpolyAnchors sets its roam order.
func (s style) DrawText(img *image.RGBA, rng *rand.Rand, ks covergen.KnobSet, t covergen.Text, f covergen.Font, cs covergen.ColorSet) {
	covergen.DrawTextOverlay(img, rng, ks, t, f, cs, lowpolyTextFrac, lowpolyAnchors)
}

const lowpolyTextFrac = 0.085

// lowpolyAnchors is lowpoly's roam order: index 0 (lower-left) is the placement
// used at text.roam 0; higher roam widens the pool.
var lowpolyAnchors = []text.Anchor{
	text.AnchorLowerLeft, text.AnchorLowerCenter, text.AnchorCenter,
	text.AnchorUpperLeft, text.AnchorLowerRight, text.AnchorUpperRight,
}

// edge returns twice the signed area of triangle (ax,ay)-(bx,by)-(cx,cy); its
// sign tells which side of edge AB point C lies on.
func edge(ax, ay, bx, by, cx, cy float64) float64 {
	return (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
}

// fillTriangle flat-fills the triangle with col using an inclusive edge-function
// test over its bounding box, so shared edges are covered by both neighbours (no
// seams) at the cost of a 1px opaque overlap.
func fillTriangle(img *image.RGBA, ax, ay, bx, by, cx, cy float64, col color.RGBA) {
	area := edge(ax, ay, bx, by, cx, cy)
	if area == 0 {
		return
	}
	sz := img.Bounds().Dx()
	minX := max(int(math.Floor(min(ax, bx, cx))), 0)
	maxX := min(int(math.Ceil(max(ax, bx, cx))), sz-1)
	minY := max(int(math.Floor(min(ay, by, cy))), 0)
	maxY := min(int(math.Ceil(max(ay, by, cy))), sz-1)
	for py := minY; py <= maxY; py++ {
		fy := float64(py) + 0.5
		for px := minX; px <= maxX; px++ {
			fx := float64(px) + 0.5
			w0 := edge(bx, by, cx, cy, fx, fy)
			w1 := edge(cx, cy, ax, ay, fx, fy)
			w2 := edge(ax, ay, bx, by, fx, fy)
			if (w0 >= 0 && w1 >= 0 && w2 >= 0) || (w0 <= 0 && w1 <= 0 && w2 <= 0) {
				img.SetRGBA(px, py, col)
			}
		}
	}
}
