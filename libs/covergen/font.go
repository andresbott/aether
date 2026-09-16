package covergen

import (
	"image"
	"math/rand/v2"

	"golang.org/x/image/font"
)

// FontClass is the typographic classification a Font belongs to. Selection is
// randomized within a class, so a style asks for "a formal face" and gets a
// deterministic-per-seed pick.
type FontClass string

const (
	FontFormal  FontClass = "formal"  // elegant serif
	FontClean   FontClass = "clean"   // neutral grotesque sans
	FontDisplay FontClass = "display" // heavy poster/display
	FontMono    FontClass = "mono"    // monospace
	FontScript  FontClass = "script"  // handwritten/script
)

// Font is one embedded typeface: it knows its name, its classification, and how
// to produce a drawable face at a pixel height. The underlying font is parsed
// once at provider construction; callers must not assume a fresh face per call.
type Font interface {
	Name() string
	Class() FontClass
	Face(pxHeight float64) font.Face
}

// FontProvider exposes a classified font set to the render pipeline. Random
// deterministically picks one font of class from rng, falling back to any font
// when the class is empty; it is the seam the lab overrides to preview a pinned
// class or a single font.
type FontProvider interface {
	Random(rng *rand.Rand, class FontClass) Font
	ByClass(class FontClass) []Font
	ByName(name string) (Font, bool)
	All() []Font
	Classes() []FontClass
}

// Text is the optional overlay a style paints on top of its art: a primary line
// (Main) and a secondary line (Subtitle). Either may be empty; when both are
// empty the render is byte-identical to the textless pipeline.
type Text struct {
	Main     string
	Subtitle string
}

// Empty reports whether there is nothing to draw.
func (t Text) Empty() bool { return t.Main == "" && t.Subtitle == "" }

// TextDrawer is the optional capability a Style implements to paint a Text
// overlay. The render pipeline resolves a font of the style's TextClass and
// calls DrawText after Draw, only for non-empty text. DrawText draws onto the
// same 2x canvas as Draw and may consume rng — but only here, so the textless
// path is unaffected.
type TextDrawer interface {
	TextClass() FontClass
	DrawText(img *image.RGBA, rng *rand.Rand, ks KnobSet, text Text, f Font)
}
