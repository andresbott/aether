package svg

import (
	"os"
	"path/filepath"
	"testing"
)

const goodMotif = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40" fill="#FF00FF"/></svg>`

func TestValidateAcceptsGoodMotif(t *testing.T) {
	if err := Validate([]byte(goodMotif)); err != nil {
		t.Fatalf("Validate rejected a good motif: %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	cases := map[string]string{
		"no viewBox":  `<svg xmlns="http://www.w3.org/2000/svg"><circle cx="5" cy="5" r="4" fill="#FF00FF"/></svg>`,
		"no sentinel": `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40" fill="#123456"/></svg>`,
		"opaque bg":   `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><rect width="100" height="100" fill="#FF00FF"/></svg>`,
		"unparseable": `<svg viewBox="0 0 100 100"><circle`,
	}
	for name, src := range cases {
		if err := Validate([]byte(src)); err == nil {
			t.Errorf("%s: Validate accepted an invalid motif", name)
		}
	}
}

func TestValidateAcceptsEdgeBleedingMotif(t *testing.T) {
	// A bottom-anchored motif: full-width fill from y=60 to the bottom edge.
	// Bottom-left/right corners rasterize opaque, top-left/right stay
	// transparent — not all four corners are opaque, so it's a legitimate
	// edge-bleeding motif (e.g. a horizon/wave shape), not a full-canvas fill.
	const edgeBleed = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><rect x="0" y="60" width="100" height="40" fill="#FF00FF"/></svg>`
	if err := Validate([]byte(edgeBleed)); err != nil {
		t.Fatalf("Validate rejected a legitimate edge-bleeding motif: %v", err)
	}
}

func TestPromoteRejectDemote(t *testing.T) {
	dir := t.TempDir()
	cand := filepath.Join(dir, "candidates")
	assets := filepath.Join(dir, "assets")
	if err := os.MkdirAll(cand, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(d, n, body string) {
		if err := os.WriteFile(filepath.Join(d, n), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(cand, "good.svg", goodMotif)
	write(cand, "bad.svg", `<svg viewBox="0 0 100 100"><rect width="100" height="100" fill="#FF00FF"/></svg>`)

	if err := Promote(cand, assets, "good.svg"); err != nil {
		t.Fatalf("Promote(good): %v", err)
	}
	if _, err := os.Stat(filepath.Join(assets, "good.svg")); err != nil {
		t.Errorf("good.svg not in assets after promote: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cand, "good.svg")); !os.IsNotExist(err) {
		t.Errorf("good.svg still in candidates after promote")
	}
	if err := Promote(cand, assets, "bad.svg"); err == nil {
		t.Errorf("Promote(bad) should fail validation")
	}
	if err := Demote(assets, cand, "good.svg"); err != nil {
		t.Fatalf("Demote: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cand, "good.svg")); err != nil {
		t.Errorf("good.svg not back in candidates after demote: %v", err)
	}
	if err := Reject(cand, "good.svg"); err != nil {
		t.Fatalf("Reject: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cand, "good.svg")); !os.IsNotExist(err) {
		t.Errorf("good.svg still present after reject")
	}
	if _, err := safeName("../etc/passwd"); err == nil {
		t.Errorf("safeName accepted a traversal path")
	}
}

func TestEmbeddedAssetsValid(t *testing.T) {
	entries, err := os.ReadDir("assets")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if filepath.Ext(e.Name()) != ".svg" {
			continue
		}
		b, err := os.ReadFile(filepath.Join("assets", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := Validate(b); err != nil {
			t.Errorf("asset %s fails contract: %v", e.Name(), err)
		}
		n++
	}
	if n == 0 {
		t.Fatal("no embedded svg assets found")
	}
}
