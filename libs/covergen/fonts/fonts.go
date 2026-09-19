// Package fonts provides covergen's built-in, OFL-licensed FontProvider. It is
// the only package that embeds the typeface bytes; import it (as the lab does)
// to give the render pipeline real fonts. The style packages never import it —
// they receive a covergen.Font from the pipeline.
package fonts

import (
	"embed"
	"fmt"
	"math/rand/v2"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"

	"github.com/andresbott/aether/libs/covergen"
)

//go:embed assets/*.ttf
var assets embed.FS

// builtins maps each embedded face to its display name and class. Several faces
// per class widen that class's random pool (Random picks within the class); add
// rows (same class allowed) to extend it.
var builtins = []struct {
	file  string
	name  string
	class covergen.FontClass
}{
	// formal — elegant serif
	{"assets/PTSerif-Regular.ttf", "PT Serif", covergen.FontFormal},
	{"assets/DMSerifDisplay-Regular.ttf", "DM Serif Display", covergen.FontFormal},
	{"assets/CormorantGaramond-Regular.ttf", "Cormorant Garamond", covergen.FontFormal},
	// clean — neutral grotesque sans
	{"assets/PTSans-Regular.ttf", "PT Sans", covergen.FontClean},
	{"assets/Inter-Regular.ttf", "Inter", covergen.FontClean},
	{"assets/WorkSans-Regular.ttf", "Work Sans", covergen.FontClean},
	// display — heavy poster/display
	{"assets/ArchivoBlack-Regular.ttf", "Archivo Black", covergen.FontDisplay},
	{"assets/Anton-Regular.ttf", "Anton", covergen.FontDisplay},
	{"assets/BebasNeue-Regular.ttf", "Bebas Neue", covergen.FontDisplay},
	// mono — monospace
	{"assets/SpaceMono-Regular.ttf", "Space Mono", covergen.FontMono},
	{"assets/IBMPlexMono-Regular.ttf", "IBM Plex Mono", covergen.FontMono},
	{"assets/JetBrainsMono-Regular.ttf", "JetBrains Mono", covergen.FontMono},
	// script — handwritten/script
	{"assets/Pacifico-Regular.ttf", "Pacifico", covergen.FontScript},
	{"assets/Lobster-Regular.ttf", "Lobster", covergen.FontScript},
	{"assets/Caveat-Regular.ttf", "Caveat", covergen.FontScript},
}

// embeddedFont is one parsed typeface implementing covergen.Font.
type embeddedFont struct {
	name   string
	class  covergen.FontClass
	parsed *opentype.Font
}

func (f *embeddedFont) Name() string              { return f.name }
func (f *embeddedFont) Class() covergen.FontClass { return f.class }

// Face builds a face at pxHeight (Size in points == pixels at 72 DPI). On the
// (practically impossible) error path it falls back to a bitmap face so callers
// never get nil.
func (f *embeddedFont) Face(pxHeight float64) font.Face {
	if pxHeight < 1 {
		pxHeight = 1
	}
	face, err := opentype.NewFace(f.parsed, &opentype.FaceOptions{Size: pxHeight, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return basicfont.Face7x13
	}
	return face
}

// provider is the default FontProvider over the embedded builtins.
type provider struct {
	all     []covergen.Font
	byName  map[string]covergen.Font
	byClass map[covergen.FontClass][]covergen.Font
	classes []covergen.FontClass
}

// Default parses the embedded typefaces once and returns a provider over them.
// It panics if an embedded asset fails to parse — a build/asset bug, never a
// runtime condition.
func Default() covergen.FontProvider {
	p := &provider{byName: map[string]covergen.Font{}, byClass: map[covergen.FontClass][]covergen.Font{}}
	for _, b := range builtins {
		raw, err := assets.ReadFile(b.file)
		if err != nil {
			panic(fmt.Sprintf("covergen/fonts: read %s: %v", b.file, err))
		}
		parsed, err := opentype.Parse(raw)
		if err != nil {
			panic(fmt.Sprintf("covergen/fonts: parse %s: %v", b.file, err))
		}
		f := &embeddedFont{name: b.name, class: b.class, parsed: parsed}
		p.all = append(p.all, f)
		p.byName[b.name] = f
		if _, seen := p.byClass[b.class]; !seen {
			p.classes = append(p.classes, b.class)
		}
		p.byClass[b.class] = append(p.byClass[b.class], f)
	}
	return p
}

func (p *provider) All() []covergen.Font                         { return p.all }
func (p *provider) Classes() []covergen.FontClass                { return p.classes }
func (p *provider) ByClass(c covergen.FontClass) []covergen.Font { return p.byClass[c] }

func (p *provider) ByName(name string) (covergen.Font, bool) {
	f, ok := p.byName[name]
	return f, ok
}

// Random picks a deterministic-per-rng font uniformly across the union of the
// given classes, falling back to any font when no class (or only unknown ones)
// is given. It consumes one rng draw when the pool is non-empty. A single class
// reproduces the old per-class pick exactly (same pool, same draw).
func (p *provider) Random(rng *rand.Rand, classes ...covergen.FontClass) covergen.Font {
	var pool []covergen.Font
	for _, c := range classes {
		pool = append(pool, p.byClass[c]...)
	}
	if len(pool) == 0 {
		pool = p.all
	}
	if len(pool) == 0 {
		return nil
	}
	return pool[rng.IntN(len(pool))]
}
