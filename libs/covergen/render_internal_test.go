package covergen

import (
	"bytes"
	"crypto/sha256"
	"image"
	"image/color"
	"math/rand/v2"
	"strconv"
	"testing"
)

// biasedStub paints either a solid fill (variation 0) or a black/white split
// (variation ~127), chosen by its first rng draw. Because the render pipeline
// reseeds the rng on each attempt, about half of all attempts come out flat and
// half varied — deterministically per seed. It lets the tests find seeds whose
// first render is flat (to prove reseeding happens) or already varied (to prove
// the good render is kept).
type biasedStub struct{}

func (biasedStub) Name() string  { return "biased" }
func (biasedStub) Knobs() []Knob { return nil }
func (biasedStub) Draw(img *image.RGBA, rng *rand.Rand, _ KnobSet) {
	if rng.Uint64()%2 == 0 {
		fillRGBA(img, color.RGBA{R: 30, G: 30, B: 30, A: 255})
		return
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.RGBA{A: 255}
			if x >= b.Dx()/2 {
				c = color.RGBA{R: 255, G: 255, B: 255, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
}

// flatStub always paints a single solid colour: no seed ever clears the floor.
type flatStub struct{}

func (flatStub) Name() string  { return "flat" }
func (flatStub) Knobs() []Knob { return nil }
func (flatStub) Draw(img *image.RGBA, _ *rand.Rand, _ KnobSet) {
	fillRGBA(img, color.RGBA{R: 10, G: 20, B: 30, A: 255})
}

const testFloor = 8.0

// scoreVar scores by colour spread alone, so the loop tests drive renderBest
// with the same ">=1 means good" contract that production coverScore uses,
// without depending on the edge-prominence floor.
func scoreVar(img *image.RGBA) float64 { return variation(img) / testFloor }

// firstRenderFlatSeed returns the hash of the first "clear-N" seed whose single
// render falls below floor, so the caller can prove the retry loop clears it.
func firstRenderFlatSeed(t *testing.T, size int, ks KnobSet) [32]byte {
	t.Helper()
	for i := 0; i < 2000; i++ {
		h := sha256.Sum256([]byte("clear-" + strconv.Itoa(i)))
		if variation(renderOnce(h, biasedStub{}, size, ks, Text{}, nil)) < testFloor {
			return h
		}
	}
	t.Fatal("no flat-start seed found for biasedStub")
	return [32]byte{}
}

func TestRenderBestReseedsPastFlatStart(t *testing.T) {
	const size = 64
	ks := newKnobSet(biasedStub{}.Knobs(), nil)
	h := firstRenderFlatSeed(t, size, ks)
	// Sanity: this seed really does start flat.
	if v := variation(renderOnce(h, biasedStub{}, size, ks, Text{}, nil)); v >= testFloor {
		t.Fatalf("precondition: first render should be flat, got variation %v", v)
	}
	got := renderBest(h, biasedStub{}, size, ks, Text{}, nil, scoreVar, 64)
	if v := variation(got); v < testFloor {
		t.Fatalf("renderBest left output below floor: got %v, want >= %v", v, testFloor)
	}
}

func TestRenderBestGivesUpOnAlwaysFlat(t *testing.T) {
	const size = 64
	ks := newKnobSet(flatStub{}.Knobs(), nil)
	h := sha256.Sum256([]byte("flat-seed"))
	got := renderBest(h, flatStub{}, size, ks, Text{}, nil, scoreVar, 8)
	if got == nil {
		t.Fatal("renderBest returned nil image")
	}
	if b := got.Bounds(); b.Dx() != size || b.Dy() != size {
		t.Fatalf("renderBest returned %dx%d, want %dx%d", b.Dx(), b.Dy(), size, size)
	}
	// Best-so-far of identical flat renders is still flat; the contract is that
	// it terminates and returns a usable image, never nil and never a panic.
	if v := variation(got); v != 0 {
		t.Fatalf("always-flat best-so-far variation = %v, want 0", v)
	}
}

func TestRenderBestKeepsGoodFirstRender(t *testing.T) {
	const size = 64
	ks := newKnobSet(biasedStub{}.Knobs(), nil)
	// Find a seed whose first render already clears the floor.
	var h [32]byte
	for i := 0; ; i++ {
		hh := sha256.Sum256([]byte("good-" + strconv.Itoa(i)))
		if variation(renderOnce(hh, biasedStub{}, size, ks, Text{}, nil)) >= testFloor {
			h = hh
			break
		}
	}
	first := renderOnce(h, biasedStub{}, size, ks, Text{}, nil)
	best := renderBest(h, biasedStub{}, size, ks, Text{}, nil, scoreVar, 8)
	if !bytes.Equal(first.Pix, best.Pix) {
		t.Fatal("renderBest changed a good first render; the fast path must return it unchanged")
	}
}

func TestRenderStyleDeterministicThroughRetry(t *testing.T) {
	const size = 64
	ks := newKnobSet(biasedStub{}.Knobs(), nil)
	// A flat-start seed forces renderStyle's retry loop (prod floor > 0) to run.
	h := firstRenderFlatSeed(t, size, ks)
	a, err := renderStyle(h, biasedStub{}, size, ks, Text{}, nil)
	if err != nil {
		t.Fatalf("renderStyle a: %v", err)
	}
	b, err := renderStyle(h, biasedStub{}, size, ks, Text{}, nil)
	if err != nil {
		t.Fatalf("renderStyle b: %v", err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("renderStyle through the retry path is not deterministic")
	}
}

// grainStub paints a solid mid-grey and declares the shared grain knob, so the
// render pipeline's post-process grain step can be exercised in isolation: a flat
// fill has zero variation, so any variation in the output is grain.
type grainStub struct{}

func (grainStub) Name() string  { return "grainy" }
func (grainStub) Knobs() []Knob { return []Knob{GrainKnob(0)} }
func (grainStub) Draw(img *image.RGBA, _ *rand.Rand, _ KnobSet) {
	fillRGBA(img, color.RGBA{R: 128, G: 128, B: 128, A: 255})
}

func TestGrainKnobControlsNoise(t *testing.T) {
	const size = 64
	h := sha256.Sum256([]byte("grain-seed"))

	none := renderOnce(h, grainStub{}, size, newKnobSet(grainStub{}.Knobs(), nil), Text{}, nil)
	if v := variation(none); v != 0 {
		t.Fatalf("grain=0 should leave a flat fill untouched, got variation %v", v)
	}

	grainy := renderOnce(h, grainStub{}, size, newKnobSet(grainStub{}.Knobs(), map[string]float64{GrainKnobName: 8}), Text{}, nil)
	if v := variation(grainy); v == 0 {
		t.Fatal("grain=8 should add noise, but the output stayed flat")
	}
}

func TestGrainRenderDeterministic(t *testing.T) {
	const size = 64
	h := sha256.Sum256([]byte("grain-seed"))
	ks := newKnobSet(grainStub{}.Knobs(), map[string]float64{GrainKnobName: 6})
	a := renderOnce(h, grainStub{}, size, ks, Text{}, nil)
	b := renderOnce(h, grainStub{}, size, ks, Text{}, nil)
	if !bytes.Equal(a.Pix, b.Pix) {
		t.Fatal("grain render is not deterministic for a fixed seed")
	}
}
