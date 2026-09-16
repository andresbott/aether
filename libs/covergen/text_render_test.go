package covergen

import (
	"bytes"
	"image"
	"image/color"
	"math/rand/v2"
	"testing"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"

	"github.com/andresbott/aether/libs/covergen/internal/text"
)

// stubStyle is a plain non-text style: a solid two-tone fill so it clears the
// quality floors. It is NOT a TextDrawer.
type stubStyle struct{}

func (stubStyle) Name() string  { return "stub" }
func (stubStyle) Knobs() []Knob { return []Knob{GrainKnob(0)} }
func (stubStyle) Draw(img *image.RGBA, _ *rand.Rand, _ KnobSet) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if (x/16)%2 == 0 {
				img.SetRGBA(x, y, color.RGBA{20, 40, 200, 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{240, 220, 20, 255})
			}
		}
	}
}

// textStub is stubStyle plus a TextDrawer that always draws a black bar via the
// text package, so we can prove the text branch runs.
type textStub struct{ stubStyle }

func (textStub) TextClass() FontClass { return FontClean }
func (textStub) DrawText(img *image.RGBA, _ *rand.Rand, _ KnobSet, t Text, f Font) {
	text.Block(img, t.Main, t.Subtitle, f.Face, 24, text.AnchorCenter, 8, 1)
}

func TestGenerateWithTextEmptyIsByteIdentical(t *testing.T) {
	g := New(stubStyle{})
	a, err := g.GenerateWithKnobs("seed-1", 128, stubStyle{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := g.GenerateWithText("seed-1", 128, stubStyle{}, nil, Text{}, fakeFonts{})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("empty-text GenerateWithText differs from GenerateWithKnobs")
	}
}

func TestGenerateWithTextChangesOutput(t *testing.T) {
	g := New(textStub{})
	plain, _ := g.GenerateWithKnobs("seed-1", 128, textStub{}, nil)
	withText, _ := g.GenerateWithText("seed-1", 128, textStub{}, nil, Text{Main: "HELLO"}, fakeFonts{})
	if bytes.Equal(plain, withText) {
		t.Fatal("text overlay did not change output")
	}
	again, _ := g.GenerateWithText("seed-1", 128, textStub{}, nil, Text{Main: "HELLO"}, fakeFonts{})
	if !bytes.Equal(withText, again) {
		t.Fatal("text render is not deterministic")
	}
}

func TestNonTextDrawerIgnoresText(t *testing.T) {
	g := New(stubStyle{})
	plain, _ := g.GenerateWithKnobs("seed-1", 128, stubStyle{}, nil)
	withText, _ := g.GenerateWithText("seed-1", 128, stubStyle{}, nil, Text{Main: "X"}, fakeFonts{})
	if !bytes.Equal(plain, withText) {
		t.Fatal("non-TextDrawer style should ignore text")
	}
}

func TestNilFontProviderIgnoresText(t *testing.T) {
	g := New(textStub{})
	plain, _ := g.GenerateWithKnobs("seed-1", 128, textStub{}, nil)
	withText, _ := g.GenerateWithText("seed-1", 128, textStub{}, nil, Text{Main: "X"}, nil)
	if !bytes.Equal(plain, withText) {
		t.Fatal("nil FontProvider with non-empty text should be a no-op")
	}
}

type fakeFont struct{}

func (fakeFont) Name() string           { return "fake" }
func (fakeFont) Class() FontClass       { return FontClean }
func (fakeFont) Face(float64) font.Face { return basicfont.Face7x13 }

type fakeFonts struct{}

func (fakeFonts) Random(*rand.Rand, FontClass) Font { return fakeFont{} }
func (fakeFonts) ByClass(FontClass) []Font          { return []Font{fakeFont{}} }
func (fakeFonts) ByName(string) (Font, bool)        { return fakeFont{}, true }
func (fakeFonts) All() []Font                       { return []Font{fakeFont{}} }
func (fakeFonts) Classes() []FontClass              { return []FontClass{FontClean} }
