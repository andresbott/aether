package covergen

import (
	"crypto/sha256"
	"errors"
	"image"
	"math/rand/v2"
)

// Style is a self-describing rendering algorithm: it knows its own name, the
// knobs it exposes for tuning, its post-processing film-grain amount, and how
// to paint itself onto an image given an RNG and a resolved KnobSet.
// Implementations live in their own per-style packages (see the covergen
// style-packages refactor); the Generator below dispatches across whichever
// set of Styles it is constructed with.
type Style interface {
	Name() string
	Knobs() []Knob
	Grain() int
	Draw(img *image.RGBA, rng *rand.Rand, ks KnobSet)
}

// Generator dispatches Style implementations by name and by seed-hash pick,
// mirroring the old package-level Generate/GenerateStyle free functions but
// over an explicit, caller-supplied set of styles instead of the fixed enum.
type Generator struct {
	styles []Style
	byName map[string]Style
}

// New builds a Generator over styles, in the given order. Styles with a
// duplicate Name() after the first are ignored.
func New(styles ...Style) *Generator {
	g := &Generator{byName: make(map[string]Style)}
	for _, s := range styles {
		if _, dup := g.byName[s.Name()]; dup {
			continue
		}
		g.byName[s.Name()] = s
		g.styles = append(g.styles, s)
	}
	return g
}

// Styles lists every style the Generator was constructed with, in order.
func (g *Generator) Styles() []Style { return g.styles }

// ByName looks up a style by its Name(), as returned by Styles.
func (g *Generator) ByName(name string) (Style, bool) { s, ok := g.byName[name]; return s, ok }

// StyleFor reports which style Generate will pick for seed. It returns nil if
// the Generator has no styles.
func (g *Generator) StyleFor(seed string) Style {
	if len(g.styles) == 0 {
		return nil
	}
	h := sha256.Sum256([]byte(seed))
	return g.styles[int(h[16])%len(g.styles)]
}

// Generate produces a deterministic abstract cover as PNG bytes, picking a
// style deterministically from the seed hash (see StyleFor).
func (g *Generator) Generate(seed string, size int) ([]byte, error) {
	s := g.StyleFor(seed)
	if s == nil {
		return nil, errors.New("covergen: generator has no styles")
	}
	h := sha256.Sum256([]byte(seed))
	return renderStyle(h, s, size, newKnobSet(s.Knobs(), nil))
}

// GenerateStyle produces a deterministic cover in the given style.
func (g *Generator) GenerateStyle(seed string, size int, s Style) ([]byte, error) {
	h := sha256.Sum256([]byte(seed))
	return renderStyle(h, s, size, newKnobSet(s.Knobs(), nil))
}

// GenerateWithKnobs is GenerateStyle with per-style knob overrides applied
// (see Style.Knobs). Overrides for keys the style does not declare are
// ignored, and nil overrides reproduce GenerateStyle exactly.
func (g *Generator) GenerateWithKnobs(seed string, size int, s Style, overrides map[string]float64) ([]byte, error) {
	h := sha256.Sum256([]byte(seed))
	return renderStyle(h, s, size, newKnobSet(s.Knobs(), overrides))
}
