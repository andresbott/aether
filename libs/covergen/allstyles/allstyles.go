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

// New returns a Generator over All() with the default palette.
func New() *covergen.Generator { return covergen.New(All(covergen.DefaultPalette())...) }
