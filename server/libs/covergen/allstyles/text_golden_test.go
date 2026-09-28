package allstyles_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/allstyles"
	"github.com/andresbott/aether/libs/covergen/fonts"
)

// updateTextGolden blesses the local text-overlay goldens (see TestTextGoldens).
var updateTextGolden = flag.Bool("update", false, "write text-overlay golden PNGs to testdata/text/")

// TestTextGoldens locks the text-overlay appearance of every style. With -update
// it (re)writes local golden PNGs under testdata/text/ (which is gitignored).
// Without it, if those goldens exist it re-renders each sample and asserts byte
// equality; if they are absent (fresh checkout / CI) it skips. This is a
// local visual guard, not a CI gate — font rasterization bytes can differ across
// environments, which is why the goldens are local-only like the other covergen
// goldens.
func TestTextGoldens(t *testing.T) {
	fp := fonts.Default()
	g := allstyles.New()
	txt := covergen.Text{Main: "Midnight Drive", Subtitle: "The Wanderers"}
	for _, s := range g.Styles() {
		got, err := g.GenerateWithText("golden-seed", 320, s, nil, txt, fp)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join("testdata", "text", s.Name()+".png")
		if *updateTextGolden {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			t.Skipf("golden %s absent (testdata/text/*.png is gitignored, local-only); run `go test ./libs/covergen/allstyles -run TestTextGoldens -update` to generate the baseline", path)
		}
		if err != nil {
			t.Fatalf("read golden %s: %v", path, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s: text cover changed from golden (got %d bytes, want %d) — only regenerate with -update if intentional", s.Name(), len(got), len(want))
		}
	}
}
