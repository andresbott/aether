package fonts_test

import (
	"math/rand/v2"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/fonts"
)

func TestDefaultCoversEveryClass(t *testing.T) {
	p := fonts.Default()
	for _, c := range []covergen.FontClass{
		covergen.FontFormal, covergen.FontClean, covergen.FontDisplay,
		covergen.FontMono, covergen.FontScript,
	} {
		if len(p.ByClass(c)) == 0 {
			t.Fatalf("class %q has no font", c)
		}
	}
	if len(p.All()) != 15 {
		t.Fatalf("All() = %d fonts, want 15", len(p.All()))
	}
	if len(p.Classes()) != 5 {
		t.Fatalf("Classes() = %d, want 5", len(p.Classes()))
	}
}

func TestRandomDeterministic(t *testing.T) {
	p := fonts.Default()
	r1 := rand.New(rand.NewPCG(1, 2))
	r2 := rand.New(rand.NewPCG(1, 2))
	f1 := p.Random(r1, covergen.FontFormal)
	f2 := p.Random(r2, covergen.FontFormal)
	if f1 == nil || f1.Name() != f2.Name() {
		t.Fatalf("Random not deterministic: %v vs %v", f1, f2)
	}
	if f1.Class() != covergen.FontFormal {
		t.Fatalf("Random(formal).Class() = %q", f1.Class())
	}
}

func TestRandomUnionOfClasses(t *testing.T) {
	p := fonts.Default()
	// Across two classes, Random must draw from BOTH over many seeds.
	got := map[covergen.FontClass]bool{}
	for i := 0; i < 200; i++ {
		f := p.Random(rand.New(rand.NewPCG(uint64(i), 9)), covergen.FontClean, covergen.FontDisplay)
		if f == nil {
			t.Fatalf("seed %d: Random(clean, display) = nil", i)
		}
		got[f.Class()] = true
	}
	if !got[covergen.FontClean] || !got[covergen.FontDisplay] {
		t.Fatalf("union pick didn't cover both classes: got %v", got)
	}
	// No classes -> falls back to any font (non-nil).
	if p.Random(rand.New(rand.NewPCG(1, 1))) == nil {
		t.Fatal("Random() with no classes should fall back to a font")
	}
}

func TestByNameAndFace(t *testing.T) {
	p := fonts.Default()
	f, ok := p.ByName("Space Mono")
	if !ok {
		t.Fatal("Space Mono not found by name")
	}
	if f.Face(24) == nil {
		t.Fatal("Face(24) returned nil")
	}
}
