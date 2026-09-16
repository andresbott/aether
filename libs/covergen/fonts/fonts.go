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

// builtins maps each embedded face to its display name and class. One face per
// class today; add rows (same class allowed) to widen a class's random pool.
var builtins = []struct {
	file  string
	name  string
	class covergen.FontClass
}{
	{"assets/PTSerif-Regular.ttf", "PT Serif", covergen.FontFormal},
	{"assets/PTSans-Regular.ttf", "PT Sans", covergen.FontClean},
	{"assets/ArchivoBlack-Regular.ttf", "Archivo Black", covergen.FontDisplay},
	{"assets/SpaceMono-Regular.ttf", "Space Mono", covergen.FontMono},
	{"assets/Pacifico-Regular.ttf", "Pacifico", covergen.FontScript},
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

// Random picks a deterministic-per-rng font of class, falling back to any font
// when the class is empty. It consumes one rng draw when the pool is non-empty.
func (p *provider) Random(rng *rand.Rand, class covergen.FontClass) covergen.Font {
	pool := p.byClass[class]
	if len(pool) == 0 {
		pool = p.all
	}
	if len(pool) == 0 {
		return nil
	}
	return pool[rng.IntN(len(pool))]
}
