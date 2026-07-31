package covergen_test

import (
	"bytes"
	"flag"
	"fmt"
	"image/png"
	"os"
	"testing"

	"github.com/andresbott/aether/internal/covergen"
)

func TestGenerateReturnsValidPNG(t *testing.T) {
	data, err := covergen.Generate("adele|19", 256)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Generate returned empty bytes")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 256 || b.Dy() != 256 {
		t.Errorf("expected 256x256 image, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestGenerateDeterministic(t *testing.T) {
	a, err := covergen.Generate("some seed", 256)
	if err != nil {
		t.Fatalf("Generate a: %v", err)
	}
	b, err := covergen.Generate("some seed", 256)
	if err != nil {
		t.Fatalf("Generate b: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("same seed+size produced different bytes")
	}
}

func TestGenerateSeedSensitive(t *testing.T) {
	a, err := covergen.Generate("seed one", 256)
	if err != nil {
		t.Fatalf("Generate a: %v", err)
	}
	b, err := covergen.Generate("seed two", 256)
	if err != nil {
		t.Fatalf("Generate b: %v", err)
	}
	if bytes.Equal(a, b) {
		t.Fatal("different seeds produced identical bytes")
	}
}

func TestGenerateRejectsNonPositiveSize(t *testing.T) {
	for _, size := range []int{0, -1, -256} {
		if _, err := covergen.Generate("seed", size); err == nil {
			t.Errorf("Generate(seed, %d) returned nil error; want error", size)
		}
	}
}

func TestGenerateUsesStyleFromSeedHash(t *testing.T) {
	// Generate must equal GenerateStyle with the style StyleFor reports,
	// proving the auto-pick is pure seed-hash dispatch.
	for _, seed := range []string{"adele|19", "miles davis|kind of blue", "x", ""} {
		auto, err := covergen.Generate(seed, 128)
		if err != nil {
			t.Fatalf("Generate(%q): %v", seed, err)
		}
		style := covergen.StyleFor(seed)
		explicit, err := covergen.GenerateStyle(seed, 128, style)
		if err != nil {
			t.Fatalf("GenerateStyle(%q, %v): %v", seed, style, err)
		}
		if !bytes.Equal(auto, explicit) {
			t.Errorf("seed %q: Generate != GenerateStyle(%v)", seed, style)
		}
	}
}

func TestStyleForCoversAllStyles(t *testing.T) {
	// With enough seeds every style must be reachable; also sanity-check
	// the distribution isn't collapsed onto one style.
	got := map[covergen.Style]int{}
	for i := 0; i < 256; i++ {
		got[covergen.StyleFor(fmt.Sprintf("seed-%d", i))]++
	}
	for _, s := range covergen.Styles() {
		if got[s] == 0 {
			t.Errorf("style %v never selected across 256 seeds", s)
		}
	}
}

func TestGenerateStyleAllStylesRenderAndDiffer(t *testing.T) {
	const seed = "style matrix seed"
	rendered := map[string][]byte{}
	for _, s := range covergen.Styles() {
		data, err := covergen.GenerateStyle(seed, 128, s)
		if err != nil {
			t.Fatalf("GenerateStyle(%v): %v", s, err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("style %v: png.Decode: %v", s, err)
		}
		if b := img.Bounds(); b.Dx() != 128 || b.Dy() != 128 {
			t.Errorf("style %v: expected 128x128, got %dx%d", s, b.Dx(), b.Dy())
		}
		for name, other := range rendered {
			if bytes.Equal(data, other) {
				t.Errorf("styles %v and %s produced identical bytes", s, name)
			}
		}
		rendered[s.String()] = data
	}
}

func TestGenerateStyleDeterministic(t *testing.T) {
	for _, s := range covergen.Styles() {
		a, err := covergen.GenerateStyle("det seed", 128, s)
		if err != nil {
			t.Fatalf("GenerateStyle(%v) a: %v", s, err)
		}
		b, err := covergen.GenerateStyle("det seed", 128, s)
		if err != nil {
			t.Fatalf("GenerateStyle(%v) b: %v", s, err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("style %v: same seed+size produced different bytes", s)
		}
	}
}

func TestGenerateStyleRejectsUnknownStyle(t *testing.T) {
	for _, s := range []covergen.Style{-1, covergen.Style(len(covergen.Styles()))} {
		if _, err := covergen.GenerateStyle("seed", 128, s); err == nil {
			t.Errorf("GenerateStyle(seed, 128, %d) returned nil error; want error", int(s))
		}
	}
}

func TestClassicBackgroundVaries(t *testing.T) {
	data, err := covergen.GenerateStyle("gradient test", 256, covergen.StyleClassic)
	if err != nil {
		t.Fatalf("GenerateStyle: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	// Corner colours must differ — confirms a gradient was drawn, not a flat fill.
	tl := img.At(0, 0)
	br := img.At(255, 255)
	if tl == br {
		t.Errorf("top-left and bottom-right are identical (%v); expected gradient", tl)
	}
}

func TestClassicHasForeground(t *testing.T) {
	// The centre pixel and a corner pixel should differ: a shape near the
	// centre should paint over the gradient, producing a different colour
	// there than the gradient alone would produce.
	data, err := covergen.GenerateStyle("shape test alpha", 256, covergen.StyleClassic)
	if err != nil {
		t.Fatalf("GenerateStyle: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	centre := img.At(128, 128)
	edge := img.At(10, 10)
	if colorDist(centre, edge) < 5 {
		t.Errorf("centre and edge nearly identical (%v vs %v); expected foreground shape", centre, edge)
	}
}

func colorDist(a, b interface{ RGBA() (r, g, b, a uint32) }) int {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	d := func(x, y uint32) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return (d(ar, br) + d(ag, bg) + d(ab, bb)) >> 8
}

var updateGolden = flag.Bool("update", false, "write golden sample PNGs to testdata/")

func TestGolden(t *testing.T) {
	if !*updateGolden {
		t.Skip("run with -update to regenerate golden samples in testdata/")
	}
	seeds := []string{
		"adele|19",
		"various artists|coyote ugly",
		"miles davis|kind of blue",
		"daft punk|one more time",
		"boards of canada|music has the right to children",
		"metallica|the memory remains",
		"dimmu borgir|stormblast",
		"est|behind the yashmak",
		"steve lacy|jazz adv",
		"clutch|the regulator",
		"air|la femme d'argent",
		"raised fist|get this right",
	}
	for _, seed := range seeds {
		data, err := covergen.Generate(seed, 512)
		if err != nil {
			t.Fatalf("Generate(%q): %v", seed, err)
		}
		filename := "testdata/" + sanitize(seed) + ".png"
		if err := os.WriteFile(filename, data, 0644); err != nil {
			t.Fatalf("write %s: %v", filename, err)
		}
	}
}

func sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+32)
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}
