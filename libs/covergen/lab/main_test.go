package main

import (
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andresbott/aether/libs/covergen/rings"
)

func TestParseOverridesReadsDeclaredKnob(t *testing.T) {
	q := url.Values{"rings.spacing": {"2.5"}}
	got := parseOverrides(rings.Style.Knobs(), q)
	if got["rings.spacing"] != 2.5 {
		t.Errorf("rings.spacing = %v, want 2.5", got["rings.spacing"])
	}
}

func TestParseOverridesIgnoresUnknownAndInvalid(t *testing.T) {
	q := url.Values{
		"rings.spacing": {"not-a-number"},
		"bogus.knob":    {"3"},
	}
	got := parseOverrides(rings.Style.Knobs(), q)
	if len(got) != 0 {
		t.Errorf("expected no overrides, got %v", got)
	}
}

func TestParseOverridesClampsToRange(t *testing.T) {
	var max float64
	for _, k := range rings.Style.Knobs() {
		if k.Name == "rings.spacing" {
			max = k.Max
		}
	}
	q := url.Values{"rings.spacing": {"99"}}
	got := parseOverrides(rings.Style.Knobs(), q)
	if got["rings.spacing"] != max {
		t.Errorf("clamped = %v, want %v (knob max)", got["rings.spacing"], max)
	}
}

func TestSvgPageListsCandidatesAndRotation(t *testing.T) {
	dir := t.TempDir()
	cand := filepath.Join(dir, "candidates")
	assets := filepath.Join(dir, "assets")
	_ = os.MkdirAll(cand, 0o755)
	_ = os.MkdirAll(assets, 0o755)
	_ = os.WriteFile(filepath.Join(cand, "cand-one.svg"), []byte(`<svg viewBox="0 0 100 100"/>`), 0o644)
	_ = os.WriteFile(filepath.Join(assets, "asset-one.svg"), []byte(`<svg viewBox="0 0 100 100"/>`), 0o644)
	svgDir = dir // package-level, set by the -svgdir flag in main

	rec := httptest.NewRecorder()
	handleSvg(rec, httptest.NewRequest("GET", "/svg", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "cand-one.svg") {
		t.Errorf("candidate not listed")
	}
	if !strings.Contains(body, "asset-one.svg") {
		t.Errorf("rotation asset not listed")
	}
}
