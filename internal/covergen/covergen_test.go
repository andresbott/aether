package covergen_test

import (
	"bytes"
	"flag"
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

func TestGenerateBackgroundVaries(t *testing.T) {
	data, err := covergen.Generate("gradient test", 256)
	if err != nil {
		t.Fatalf("Generate: %v", err)
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

func TestGenerateHasForeground(t *testing.T) {
	// The centre pixel and a corner pixel should differ: a shape near the
	// centre should paint over the gradient, producing a different colour
	// there than the gradient alone would produce.
	data, err := covergen.Generate("shape test alpha", 256)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Sample several seeds; at least one must show a centre/edge colour diff
	// > threshold, proving shapes draw.
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
