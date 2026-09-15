// Package allstyles bundles covergen's six built-in styles for callers that want
// the full set. Importing it links all six; import the individual style packages
// instead to link only what you use.
package allstyles

import (
	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/bauhaus"
	"github.com/andresbott/aether/libs/covergen/classic"
	"github.com/andresbott/aether/libs/covergen/poster"
	"github.com/andresbott/aether/libs/covergen/remix"
	"github.com/andresbott/aether/libs/covergen/rings"
	"github.com/andresbott/aether/libs/covergen/waves"
)

// All returns the six styles in canonical order (matching the pre-refactor Style
// iota) so Generator.Generate's hash-pick is byte-for-byte unchanged, each
// colored by pal.
func All(pal covergen.Palette) []covergen.Style {
	return []covergen.Style{
		classic.New(pal), bauhaus.New(pal), rings.New(pal),
		waves.New(pal), poster.New(pal), remix.New(pal),
	}
}

// New returns a Generator over the six built-in styles, each paired with the
// palette it was tuned for in the lab (bauhaus and rings on neon, waves on
// triadic, poster and remix on pastel, classic on the default harmony). Style
// order matches All so Generate's seed-hash style pick is byte-for-byte
// unchanged. Each style's own palette-knob tweaks live in its package (see its
// *PaletteDefaults), so those apply on top of whichever palette is paired here.
func New() *covergen.Generator {
	return covergen.New(
		classic.New(covergen.PaletteByName("harmony")),
		bauhaus.New(covergen.PaletteByName("neon")),
		rings.New(covergen.PaletteByName("neon")),
		waves.New(covergen.PaletteByName("triadic")),
		poster.New(covergen.PaletteByName("pastel")),
		remix.New(covergen.PaletteByName("pastel")),
	)
}
