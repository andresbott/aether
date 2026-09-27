package covergen_test

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/allstyles"
)

// TestPalettesRenderThroughStyles ensures every built-in palette produces
// renderable covers: each palette is injected into every style and rendered at a
// small size, and the result must be a valid PNG. This guards against a palette
// returning colours the render pipeline (quality floors, downsample, grain)
// cannot handle. It complements the colour-level invariants in palette_test.go.
func TestPalettesRenderThroughStyles(t *testing.T) {
	for _, pal := range covergen.Palettes() {
		for _, s := range allstyles.All(pal) {
			g := covergen.New(s)
			data, err := g.GenerateStyle("palette-render-seed", 48, s)
			if err != nil {
				t.Errorf("palette %s / style %s: %v", pal.Name(), s.Name(), err)
				continue
			}
			if _, err := png.Decode(bytes.NewReader(data)); err != nil {
				t.Errorf("palette %s / style %s: invalid PNG: %v", pal.Name(), s.Name(), err)
			}
		}
	}
}
