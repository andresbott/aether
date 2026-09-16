// Package allstyles bundles covergen's ten built-in styles for callers that want
// the full set. Importing it links all ten; import the individual style packages
// instead to link only what you use.
//
// The work-in-progress svg style is deliberately left out of this bundle while
// its feature flag (svg.Enabled) is false: it stays in the tree but unwired, so
// nothing that imports allstyles — the server included — links or renders it. Add
// it back here once that flag is flipped.
package allstyles

import (
	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/bauhaus"
	"github.com/andresbott/aether/libs/covergen/classic"
	"github.com/andresbott/aether/libs/covergen/halftone"
	"github.com/andresbott/aether/libs/covergen/liquid"
	"github.com/andresbott/aether/libs/covergen/lowpoly"
	"github.com/andresbott/aether/libs/covergen/mosaic"
	"github.com/andresbott/aether/libs/covergen/poster"
	"github.com/andresbott/aether/libs/covergen/remix"
	"github.com/andresbott/aether/libs/covergen/rings"
	"github.com/andresbott/aether/libs/covergen/waves"
)

// All returns the built-in styles, each colored by pal. The original six lead in
// their canonical order (matching the pre-refactor Style iota); the four newer
// styles (mosaic, halftone, liquid, lowpoly) follow. Generator.Generate picks a
// style by seed hash modulo the style count, so appending the newer four kept the
// order stable but shifted which style a given seed resolves to.
func All(pal covergen.Palette) []covergen.Style {
	return []covergen.Style{
		classic.New(pal), bauhaus.New(pal), rings.New(pal),
		waves.New(pal), poster.New(pal), remix.New(pal),
		mosaic.New(pal), halftone.New(pal), liquid.New(pal), lowpoly.New(pal),
	}
}

// defaultPalettes pairs each built-in style with the palette it was tuned for in
// the lab. It is the single source of truth for the pairing: New() ships each
// style on its palette, and DefaultPaletteName exposes it so the lab defaults its
// palette picker (and previews) to it instead of always to harmony.
var defaultPalettes = map[string]string{
	"classic":  "harmony",
	"bauhaus":  "neon",
	"rings":    "neon",
	"waves":    "triadic",
	"poster":   "pastel",
	"remix":    "pastel",
	"mosaic":   "neon",
	"halftone": "mono",
	"liquid":   "triadic",
	"lowpoly":  "triadic",
}

// DefaultPaletteName returns the palette a built-in style ships on (see New), or
// "" for a style with no pairing (e.g. svg) — which callers resolve to the
// default harmony palette via covergen.PaletteByName.
func DefaultPaletteName(styleName string) string { return defaultPalettes[styleName] }

// New returns a Generator over the ten built-in styles, each paired with the
// palette it was tuned for in the lab (bauhaus, rings, and mosaic on neon; waves,
// liquid, and lowpoly on triadic; poster and remix on pastel; classic on harmony;
// halftone on mono; see defaultPalettes). Style order matches All. Each style's
// own palette-knob tweaks live in its package (see its *PaletteDefaults), so those
// apply on top of whichever palette is paired here.
func New() *covergen.Generator {
	pal := func(style string) covergen.Palette { return covergen.PaletteByName(defaultPalettes[style]) }
	return covergen.New(
		classic.New(pal("classic")),
		bauhaus.New(pal("bauhaus")),
		rings.New(pal("rings")),
		waves.New(pal("waves")),
		poster.New(pal("poster")),
		remix.New(pal("remix")),
		mosaic.New(pal("mosaic")),
		halftone.New(pal("halftone")),
		liquid.New(pal("liquid")),
		lowpoly.New(pal("lowpoly")),
	)
}
