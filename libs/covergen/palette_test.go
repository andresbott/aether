package covergen

import (
	"crypto/sha256"
	"image/color"
	"math"
	"strconv"
	"testing"

	"github.com/andresbott/aether/libs/covergen/internal/paint"
)

// The invariants below hold for every registered palette (harmony included, since
// it is in Palettes()): deterministic output, four opaque colours, a guaranteed
// Background/Ink contrast, and a live palette.saturation knob.

func TestAllPalettesDeterministic(t *testing.T) {
	h := sha256.Sum256([]byte("seed"))
	for _, p := range Palettes() {
		ks := newKnobSet(p.Knobs(), nil)
		a := p.Colors(rngFromHash(h), ks)
		b := p.Colors(rngFromHash(h), ks)
		if a != b {
			t.Errorf("%s: Colors not deterministic: %v vs %v", p.Name(), a, b)
		}
	}
}

func TestAllPalettesFourOpaqueColors(t *testing.T) {
	for _, p := range Palettes() {
		ks := newKnobSet(p.Knobs(), nil)
		for i := 0; i < 200; i++ {
			h := sha256.Sum256([]byte("seed-" + strconv.Itoa(i)))
			cs := p.Colors(rngFromHash(h), ks)
			for _, c := range []color.RGBA{cs.Background, cs.Ink, cs.Accent1, cs.Accent2} {
				if c.A != 255 {
					t.Fatalf("%s seed %d: non-opaque color %v", p.Name(), i, c)
				}
			}
		}
	}
}

func TestAllPalettesBackgroundInkContrast(t *testing.T) {
	for _, p := range Palettes() {
		ks := newKnobSet(p.Knobs(), nil)
		for i := 0; i < 200; i++ {
			h := sha256.Sum256([]byte("contrast-" + strconv.Itoa(i)))
			cs := p.Colors(rngFromHash(h), ks)
			if d := math.Abs(paint.Luminance(cs.Background) - paint.Luminance(cs.Ink)); d < 50 {
				t.Fatalf("%s seed %d: Background/Ink luminance gap %.1f < 50 (bg=%v ink=%v)", p.Name(), i, d, cs.Background, cs.Ink)
			}
		}
	}
}

func TestAllPalettesSaturationKnobChangesColors(t *testing.T) {
	h := sha256.Sum256([]byte("knob"))
	for _, p := range Palettes() {
		base := p.Colors(rngFromHash(h), newKnobSet(p.Knobs(), nil))
		tuned := p.Colors(rngFromHash(h), newKnobSet(p.Knobs(), map[string]float64{"palette.saturation": 0}))
		if base == tuned {
			t.Errorf("%s: palette.saturation override had no effect on colors", p.Name())
		}
	}
}

// Colors is the exported helper the lab uses to preview a palette's role colors
// without reaching into the package's unexported seed/knob resolution. It must
// reproduce exactly what the render path feeds a style's Draw.
func TestColorsMirrorsRenderPath(t *testing.T) {
	pal := DefaultPalette()
	ks := pal.Knobs()
	const seed = "mirror-seed"
	got := Colors(pal, seed, ks, nil)
	h := sha256.Sum256([]byte(seed))
	want := pal.Colors(rngFromHash(h), newKnobSet(ks, nil))
	if got != want {
		t.Fatalf("Colors = %v, want %v (must match seed->rng + newKnobSet)", got, want)
	}
}

func TestColorsDeterministic(t *testing.T) {
	pal := DefaultPalette()
	if a, b := Colors(pal, "s", pal.Knobs(), nil), Colors(pal, "s", pal.Knobs(), nil); a != b {
		t.Fatalf("Colors not deterministic: %v vs %v", a, b)
	}
}

func TestColorsHonorsOverrides(t *testing.T) {
	pal := DefaultPalette()
	base := Colors(pal, "s", pal.Knobs(), nil)
	tuned := Colors(pal, "s", pal.Knobs(), map[string]float64{"palette.saturation": 0})
	if base == tuned {
		t.Fatal("palette.saturation override had no effect through Colors")
	}
}

// Palettes is the lab's picker source; every entry must be usable as a labelled,
// distinct option.
func TestPalettesAreNamedAndUnique(t *testing.T) {
	ps := Palettes()
	if len(ps) == 0 {
		t.Fatal("Palettes() returned none")
	}
	seen := map[string]bool{}
	for _, p := range ps {
		n := p.Name()
		if n == "" {
			t.Errorf("palette %T has an empty Name()", p)
		}
		if seen[n] {
			t.Errorf("duplicate palette name %q", n)
		}
		seen[n] = true
	}
	if DefaultPalette().Name() == "" {
		t.Error("DefaultPalette has an empty Name()")
	}
}

// PaletteByName backs the shipped per-style palette pairing (allstyles) and the
// lab's picker: a known name returns that palette; anything else falls back to
// the default so a stray or renamed value can never break rendering.
func TestPaletteByName(t *testing.T) {
	def := DefaultPalette().Name()
	for _, p := range Palettes() {
		if got := PaletteByName(p.Name()).Name(); got != p.Name() {
			t.Errorf("PaletteByName(%q) = %q, want %q", p.Name(), got, p.Name())
		}
	}
	for _, name := range []string{"", "does-not-exist"} {
		if got := PaletteByName(name).Name(); got != def {
			t.Errorf("PaletteByName(%q) = %q, want default %q", name, got, def)
		}
	}
}
