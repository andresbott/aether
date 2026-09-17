package covergen

import (
	"image/color"
	"math"
	"math/rand/v2"
	"testing"

	"github.com/andresbott/aether/libs/covergen/internal/paint"
)

func TestPoolSize(t *testing.T) {
	cases := []struct {
		frac float64
		n    int
		want int
	}{
		{-1, 6, 1}, {0, 6, 1}, {1, 6, 6}, {2, 6, 6},
		{0.5, 6, 4}, // 1 + round(0.5*5) = 1 + 3
		{0.5, 3, 2}, // 1 + round(0.5*2) = 1 + 1
	}
	for _, c := range cases {
		if got := poolSize(c.frac, c.n); got != c.want {
			t.Errorf("poolSize(%v, %d) = %d, want %d", c.frac, c.n, got, c.want)
		}
	}
}

func TestOutlineFillContrastsInnerBand(t *testing.T) {
	// A tint whose luminance sits on top of the inner band must be pushed away so the
	// letter core reads against its own border.
	inner := color.RGBA{120, 120, 120, 255}
	tint := color.NRGBA{128, 128, 128, 255} // near-identical luminance to inner
	got := outlineFill(&tint, inner)
	if gap := math.Abs(paint.Luminance(color.RGBA{got.R, got.G, got.B, 255}) - paint.Luminance(inner)); gap < outlineFillContrast-1 {
		t.Fatalf("outlineFill lum gap %.0f from inner band, want >= %d", gap, outlineFillContrast-1)
	}
	// A neutral fill over a light band already contrasts, so it is returned unchanged.
	if got := outlineFill(nil, color.RGBA{230, 230, 230, 255}); got != (color.NRGBA{20, 20, 20, 255}) {
		t.Errorf("nil ink over a light band = %v, want near-black neutral {20,20,20,255}", got)
	}
}

func TestTextInks(t *testing.T) {
	cs := ColorSet{
		Accent1: color.RGBA{200, 40, 40, 255}, // red
		Accent2: color.RGBA{40, 60, 200, 255}, // blue
	}
	inks := textInks(cs, 0.85)
	if len(inks) != 3 {
		t.Fatalf("len(inks) = %d, want 3", len(inks))
	}
	if inks[0] != nil {
		t.Errorf("inks[0] = %v, want nil (auto black/white)", inks[0])
	}
	if inks[1] == nil || inks[2] == nil || *inks[1] != *inks[2] {
		t.Fatalf("inks[1],[2] should both be the tint colour, got %v and %v", inks[1], inks[2])
	}
	spread := func(c *color.NRGBA) int {
		hi, lo := c.R, c.R
		for _, v := range []uint8{c.G, c.B} {
			if v > hi {
				hi = v
			}
			if v < lo {
				lo = v
			}
		}
		return int(hi) - int(lo)
	}
	// A high saturation gives an actual colour, not grey.
	if s := spread(inks[1]); s < 40 {
		t.Errorf("tint colour %v is too grey (channel spread %d)", *inks[1], s)
	}
	// Lower saturation greys it: strictly less channel spread.
	if low := spread(textInks(cs, 0.05)[1]); low >= spread(inks[1]) {
		t.Errorf("low-saturation tint (spread %d) should be greyer than the vivid one (spread %d)", low, spread(inks[1]))
	}
}

func TestSizeJitter(t *testing.T) {
	// spread 0 => exactly base, every seed.
	for i := 0; i < 20; i++ {
		rng := rand.New(rand.NewPCG(uint64(i), 1))
		if px := sizeJitter(rng, 0, 40); px != 40 {
			t.Errorf("seed %d: sizeJitter(spread 0) = %v, want 40", i, px)
		}
	}
	// spread 1 => varies across seeds and never below the 6px floor.
	sizes := map[float64]bool{}
	for i := 0; i < 50; i++ {
		rng := rand.New(rand.NewPCG(uint64(i), 2))
		px := sizeJitter(rng, 1, 40)
		sizes[px] = true
		if px < 6 {
			t.Fatalf("seed %d: px %v below the 6px floor", i, px)
		}
	}
	if len(sizes) < 2 {
		t.Error("spread 1 should vary px across seeds")
	}
}

func TestMinTextPx(t *testing.T) {
	approx := func(got, want float64) bool { d := got - want; return d < 1e-9 && d > -1e-9 }
	// The floor is the low end of sizeJitter's per-seed range: base*(1-0.25*spread).
	if got := minTextPx(100, 1.7); !approx(got, 57.5) {
		t.Errorf("minTextPx(100, 1.7) = %v, want ~57.5", got)
	}
	// spread 0 => no shrink headroom, the floor equals base.
	if got := minTextPx(80, 0); got != 80 {
		t.Errorf("minTextPx(80, 0) = %v, want 80", got)
	}
	// Clamped to the same 6px floor as sizeJitter when the formula drops low.
	if got := minTextPx(10, 4); got != 6 { // 10*(1-1) = 0 -> clamp
		t.Errorf("minTextPx(10, 4) = %v, want 6 (clamped)", got)
	}
}

func TestPickInk(t *testing.T) {
	cs := ColorSet{Accent1: color.RGBA{200, 40, 40, 255}, Accent2: color.RGBA{40, 60, 200, 255}}
	// tint 0 => always auto (nil).
	for i := 0; i < 20; i++ {
		rng := rand.New(rand.NewPCG(uint64(i), 3))
		if ink := pickInk(rng, 0, 0.85, cs); ink != nil {
			t.Errorf("seed %d: pickInk(tint 0) = %v, want nil (auto)", i, ink)
		}
	}
	// tint 1 => some seeds get a palette-derived colour.
	colored := false
	for i := 0; i < 50; i++ {
		rng := rand.New(rand.NewPCG(uint64(i), 4))
		if pickInk(rng, 1, 0.85, cs) != nil {
			colored = true
			break
		}
	}
	if !colored {
		t.Error("tint 1 should ink some seeds in a palette-derived colour")
	}
}

func TestJitterOpacity(t *testing.T) {
	// spread 0 => exactly the center, every seed.
	for i := 0; i < 20; i++ {
		rng := rand.New(rand.NewPCG(uint64(i), 5))
		if o := jitterOpacity(rng, 1, 0); o != 1 {
			t.Errorf("seed %d: jitterOpacity(center 1, spread 0) = %v, want 1", i, o)
		}
	}
	// spread 1 => varies across seeds and stays within [0,1].
	vals := map[float64]bool{}
	for i := 0; i < 50; i++ {
		rng := rand.New(rand.NewPCG(uint64(i), 6))
		o := jitterOpacity(rng, 0.8, 1)
		vals[o] = true
		if o < 0 || o > 1 {
			t.Fatalf("seed %d: opacity %v out of [0,1]", i, o)
		}
	}
	if len(vals) < 2 {
		t.Error("spread 1 should vary opacity across seeds")
	}
}

func TestSaturationJitter(t *testing.T) {
	// spread 0 => exactly the center, every seed.
	for i := 0; i < 20; i++ {
		rng := rand.New(rand.NewPCG(uint64(i), 7))
		if s := saturationJitter(rng, 0.85, 0); s != 0.85 {
			t.Errorf("seed %d: saturationJitter(center 0.85, spread 0) = %v, want 0.85", i, s)
		}
	}
	// spread 1 => varies across seeds and stays within [0,1].
	vals := map[float64]bool{}
	for i := 0; i < 50; i++ {
		rng := rand.New(rand.NewPCG(uint64(i), 8))
		s := saturationJitter(rng, 0.7, 1)
		vals[s] = true
		if s < 0 || s > 1 {
			t.Fatalf("seed %d: saturation %v out of [0,1]", i, s)
		}
	}
	if len(vals) < 2 {
		t.Error("spread 1 should vary saturation across seeds")
	}
}
