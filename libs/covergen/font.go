package covergen

import (
	"image"
	"math/rand/v2"

	"golang.org/x/image/font"
)

// FontClass is the typographic classification a Font belongs to. Selection is
// randomized within the class(es) a style declares, so a style asks for "a formal
// face" (or a small pool of classes) and gets a deterministic-per-seed pick.
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
// deterministically picks one font from rng, uniformly across the union of the
// given classes, falling back to any font when no class (or only an unknown one)
// is given; it is the seam the lab overrides to preview a pinned class-set or a
// single font.
type FontProvider interface {
	Random(rng *rand.Rand, classes ...FontClass) Font
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
// overlay. The render pipeline resolves a font from the union of the style's
// TextClasses and calls DrawText after Draw, only for non-empty text. A style
// returning more than one class gets a per-seed pick across all their fonts.
// DrawText draws onto the same 2x canvas as Draw and may consume rng — but only
// here, so the textless path is unaffected. cs is the cover's ColorSet (see
// Colored), so a style can ink type in palette roles; it is the zero value for
// styles that don't implement Colored.
type TextDrawer interface {
	TextClasses() []FontClass
	DrawText(img *image.RGBA, rng *rand.Rand, ks KnobSet, text Text, f Font, cs ColorSet)
}

// Colored is the optional capability a Style implements to expose its per-cover
// ColorSet so the text overlay can ink type in palette roles. renderOnce
// recomputes it from a fresh per-seed rng — matching Draw's first colour draw —
// and passes the result to DrawText.
type Colored interface {
	Colors(rng *rand.Rand, ks KnobSet) ColorSet
}
