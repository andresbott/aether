package main

import (
	"encoding/json"
	"image/color"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/rings"
	"github.com/andresbott/aether/libs/covergen/svg"
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

func TestHexRGBADropsAlpha(t *testing.T) {
	if got := hexRGBA(color.RGBA{R: 0x0a, G: 0xff, B: 0x00, A: 0x80}); got != "#0aff00" {
		t.Errorf("hexRGBA = %q, want #0aff00", got)
	}
}

func TestStyleWithPaletteResolves(t *testing.T) {
	pal := covergen.DefaultPalette()
	if _, ok := styleWithPalette("rings", pal); !ok {
		t.Error("rings should resolve")
	}
	if s, ok := styleWithPalette("svg", pal); !ok || s.Name() != "svg" {
		t.Error("svg should resolve (palette ignored)")
	}
	if _, ok := styleWithPalette("nope", pal); ok {
		t.Error("unknown style must not resolve")
	}
}

func TestIsPaletteStyle(t *testing.T) {
	rs, _ := styleWithPalette("rings", covergen.DefaultPalette())
	if !isPaletteStyle(rs) {
		t.Error("rings is palette-colored")
	}
	if isPaletteStyle(svg.Style) {
		t.Error("svg is not palette-colored")
	}
}

func TestHandleColorsReturnsHexRoles(t *testing.T) {
	rec := httptest.NewRecorder()
	handleColors(rec, httptest.NewRequest("GET", "/colors?style=rings&palette="+covergen.DefaultPalette().Name()+"&seed=abc", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, role := range []string{"background", "ink", "accent1", "accent2"} {
		if v := got[role]; len(v) != 7 || v[0] != '#' {
			t.Errorf("role %q = %q, want #rrggbb", role, v)
		}
	}
}

func TestHandleColorsDeterministic(t *testing.T) {
	const req = "/colors?style=rings&seed=abc"
	r1 := httptest.NewRecorder()
	handleColors(r1, httptest.NewRequest("GET", req, nil))
	r2 := httptest.NewRecorder()
	handleColors(r2, httptest.NewRequest("GET", req, nil))
	if r1.Body.String() != r2.Body.String() {
		t.Error("identical request must yield identical colors")
	}
}

func TestHandleColorsSaturationKnobShiftsColors(t *testing.T) {
	base := httptest.NewRecorder()
	handleColors(base, httptest.NewRequest("GET", "/colors?style=rings&seed=abc", nil))
	tuned := httptest.NewRecorder()
	handleColors(tuned, httptest.NewRequest("GET", "/colors?style=rings&seed=abc&palette.saturation=0", nil))
	if base.Body.String() == tuned.Body.String() {
		t.Error("palette.saturation override should change the previewed colors")
	}
}

func TestHandleColorsRejectsSvg(t *testing.T) {
	rec := httptest.NewRecorder()
	handleColors(rec, httptest.NewRequest("GET", "/colors?style=svg&seed=abc", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("svg colors status = %d, want 404", rec.Code)
	}
}

func TestHandleColorsUnknownStyle(t *testing.T) {
	rec := httptest.NewRecorder()
	handleColors(rec, httptest.NewRequest("GET", "/colors?style=nope&seed=abc", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown style status = %d, want 400", rec.Code)
	}
}

func TestStylePageShowsPalettePickerAndSwatches(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/style/rings", nil)
	req.SetPathValue("style", "rings")
	handleStyle(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, `id="palette"`) {
		t.Error("palette-colored style page should include the palette picker")
	}
	if !strings.Contains(body, "swatch-row") {
		t.Error("palette style page should include swatch rows")
	}
}

func TestStylePageHidesPalettePickerForSvg(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/style/svg", nil)
	req.SetPathValue("style", "svg")
	handleStyle(rec, req)
	if strings.Contains(rec.Body.String(), `id="palette"`) {
		t.Error("svg page must not include a palette picker")
	}
}

func TestStylePageListsAllPalettes(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/style/rings", nil)
	req.SetPathValue("style", "rings")
	handleStyle(rec, req)
	body := rec.Body.String()
	pals := covergen.Palettes()
	if len(pals) < 2 {
		t.Fatalf("expected multiple palettes, got %d", len(pals))
	}
	for _, p := range pals {
		if !strings.Contains(body, `value="`+p.Name()+`"`) {
			t.Errorf("palette picker missing option %q", p.Name())
		}
	}
}

func TestHandleColorsVariesByPalette(t *testing.T) {
	colorsFor := func(pal string) string {
		rec := httptest.NewRecorder()
		handleColors(rec, httptest.NewRequest("GET", "/colors?style=rings&seed=abc&palette="+pal, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("palette %s: status %d (%s)", pal, rec.Code, rec.Body.String())
		}
		return rec.Body.String()
	}
	if colorsFor("harmony") == colorsFor("neon") {
		t.Error("harmony and neon should produce different colors for the same seed")
	}
}

func TestSetupSnippetOnlyIncludesNonDefaultKnobs(t *testing.T) {
	style := rings.Style // harmony palette
	// rings.wobble default is 0.75; rings.spacing default is 1.0.
	values := map[string]float64{
		"rings.wobble":  1.2, // changed -> must appear
		"rings.spacing": 1.0, // == default -> must be omitted
	}
	got := setupSnippet(style, "harmony", values)
	if !strings.Contains(got, `"rings.wobble"`) {
		t.Errorf("snippet should include the changed knob rings.wobble:\n%s", got)
	}
	if strings.Contains(got, `"rings.spacing"`) {
		t.Errorf("snippet must omit the at-default knob rings.spacing:\n%s", got)
	}
}

func TestSetupSnippetUsesPaletteConstructor(t *testing.T) {
	style := rings.New(covergen.PaletteByName("neon"))
	got := setupSnippet(style, "neon", map[string]float64{"rings.wobble": 1.2})
	if !strings.Contains(got, `rings.New(covergen.PaletteByName("neon"))`) {
		t.Errorf("snippet should construct the style on the selected palette:\n%s", got)
	}
	if !strings.Contains(got, "GenerateWithKnobs(") {
		t.Errorf("snippet with overrides should call GenerateWithKnobs:\n%s", got)
	}
}

func TestSetupSnippetNoOverridesUsesGenerateStyle(t *testing.T) {
	style := rings.Style
	values := map[string]float64{} // every knob left at its default
	for _, k := range style.Knobs() {
		values[k.Name] = k.Default
	}
	got := setupSnippet(style, "harmony", values)
	if !strings.Contains(got, "GenerateStyle(") {
		t.Errorf("no-override snippet should call GenerateStyle:\n%s", got)
	}
	if strings.Contains(got, "GenerateWithKnobs(") || strings.Contains(got, "map[string]float64") {
		t.Errorf("no-override snippet must not build an overrides map:\n%s", got)
	}
}

func TestSetupSnippetKeepsKnobOrder(t *testing.T) {
	style := rings.Style
	// Both changed; the snippet must list them in Knobs() order (rings.spacing
	// precedes rings.wobble), not Go's random map iteration order.
	values := map[string]float64{
		"rings.wobble":  1.2,
		"rings.spacing": 2.0,
	}
	got := setupSnippet(style, "harmony", values)
	iSpacing := strings.Index(got, `"rings.spacing"`)
	iWobble := strings.Index(got, `"rings.wobble"`)
	if iSpacing < 0 || iWobble < 0 {
		t.Fatalf("both changed knobs should be present:\n%s", got)
	}
	if iSpacing > iWobble {
		t.Errorf("knobs should follow Knobs() order (spacing before wobble):\n%s", got)
	}
}

func TestSetupSnippetSvgUsesAssetConstructor(t *testing.T) {
	got := setupSnippet(svg.Style, "harmony", map[string]float64{})
	if !strings.Contains(got, "svg.New(") {
		t.Errorf("svg snippet should use the asset constructor svg.New:\n%s", got)
	}
	if strings.Contains(got, "PaletteByName") {
		t.Errorf("svg has no palette; snippet must not reference PaletteByName:\n%s", got)
	}
}

func TestHandleSetupReturnsSnippet(t *testing.T) {
	rec := httptest.NewRecorder()
	handleSetup(rec, httptest.NewRequest("GET", "/setup?style=rings&palette=neon&rings.wobble=1.2", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `rings.New(covergen.PaletteByName("neon"))`) {
		t.Errorf("snippet should construct rings on neon:\n%s", body)
	}
	if !strings.Contains(body, `"rings.wobble"`) {
		t.Errorf("snippet should carry the overridden knob:\n%s", body)
	}
}

func TestHandleSetupUnknownStyle(t *testing.T) {
	rec := httptest.NewRecorder()
	handleSetup(rec, httptest.NewRequest("GET", "/setup?style=nope", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown style status = %d, want 400", rec.Code)
	}
}

func TestStylePagePreselectsShippedPalette(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/style/rings", nil) // no ?palette=
	req.SetPathValue("style", "rings")
	handleStyle(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, `<option value="neon" selected>`) {
		t.Error("rings should default its palette picker to neon (its shipped palette)")
	}
	if strings.Contains(body, `<option value="harmony" selected>`) {
		t.Error("rings must not default to harmony")
	}
}

func TestStylePageClassicDefaultsToHarmony(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/style/classic", nil)
	req.SetPathValue("style", "classic")
	handleStyle(rec, req)
	if !strings.Contains(rec.Body.String(), `<option value="harmony" selected>`) {
		t.Error("classic ships on harmony and should default to it")
	}
}

func TestStylePaletteQueryOverridesShippedDefault(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/style/rings?palette=pastel", nil)
	req.SetPathValue("style", "rings")
	handleStyle(rec, req)
	if !strings.Contains(rec.Body.String(), `<option value="pastel" selected>`) {
		t.Error("an explicit ?palette= should win over the shipped default")
	}
}

func TestStylePageHasSetupButton(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/style/rings", nil)
	req.SetPathValue("style", "rings")
	handleStyle(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, `id="copy-setup"`) {
		t.Error("style page should have a Copy setup button")
	}
	if !strings.Contains(body, `id="setup"`) {
		t.Error("style page should have the setup dialog")
	}
}
