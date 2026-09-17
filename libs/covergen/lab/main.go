// Command covergenlab is a local, browser-based tuning lab for the covergen
// package. It renders every style across a spread of random seeds and exposes
// each style's knobs as live sliders, so tuning cover art is a drag-and-watch
// loop instead of edit-rebuild-squint. Each palette-colored style also has a
// palette picker (decoupled from the style) exposing the selected palette's own
// knobs plus a live swatch strip of its four role colors; grain is a per-style
// knob like any other.
//
// It is a dev tool: it lives under libs/covergen/lab and is never imported by the server
// binary, so it ships in nothing. Edits to covergen's algorithms show up on
// restart; knob edits are live. Run it with:
//
//	go run ./libs/covergen/lab              # then open http://localhost:8099
//	go run ./libs/covergen/lab -addr :9000
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"image/color"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/allstyles"
	"github.com/andresbott/aether/libs/covergen/fonts"
	"github.com/andresbott/aether/libs/covergen/svg"
)

const (
	overviewPerStyle = 6   // thumbnails per style on the overview
	tuningGrid       = 24  // seeds shown on a style's tuning page
	thumbSize        = 200 // px; rendered fresh per request (no cache)
	swatchSeeds      = 4   // sample seeds shown as palette-role swatch rows
)

// placeholderMain and placeholderSub are the sample title/artist the lab overlays
// on thumbnails while the "Text" toggle is on, so text placement can be previewed
// without typing. The same pair is used on every thumbnail.
const (
	placeholderMain = "Midnight Drive"
	placeholderSub  = "The Wanderers"
)

// gen is the Generator over covergen's full built-in style set. The
// work-in-progress svg style is appended only when svg.Enabled (a temporary
// feature flag): it stays opt-in and out of the shipped allstyles set, and the
// lab (a dev-only tool, never imported by the server binary) surfaces it for
// curation and tuning only while that flag is on. Flip svg.Enabled to bring it
// back here and under /svg.
var gen = newGen()

func newGen() *covergen.Generator {
	styles := allstyles.All(covergen.DefaultPalette())
	if svg.Enabled {
		styles = append(styles, svg.Style)
	}
	return covergen.New(styles...)
}

// fontLib is the default provider the lab renders text with. Pinning happens via
// wrappers (see fontProviderFor).
var fontLib = fonts.Default()

// svgDir is the on-disk location of the svg style package (candidates/ + assets/),
// set from the -svgdir flag; the curation tool reads and writes there.
var svgDir = "libs/covergen/svg"

func candidatesDir() string { return filepath.Join(svgDir, "candidates") }
func assetsDir() string     { return filepath.Join(svgDir, "assets") }

// paletteNames lists the built-in palettes for the style page's picker.
func paletteNames() []string {
	ps := covergen.Palettes()
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Name()
	}
	return out
}

// styleWithPalette builds the named style colored by pal. svg colors itself from
// its own knobs and ignores the palette. Returns false for an unknown style.
func styleWithPalette(name string, pal covergen.Palette) (covergen.Style, bool) {
	if name == svg.Style.Name() {
		return svg.Style, true
	}
	for _, s := range allstyles.All(pal) {
		if s.Name() == name {
			return s, true
		}
	}
	return nil, false
}

// paletteFor resolves the palette for styleName from the request. An explicit
// ?palette= wins; with none, it falls back to the style's shipped palette (the
// allstyles pairing) so the lab previews and pre-selects each style the way it
// actually ships, instead of always defaulting to harmony. Unknown or
// palette-less styles (svg) fall through to harmony via PaletteByName("").
func paletteFor(styleName string, q url.Values) covergen.Palette {
	name := q.Get("palette")
	if name == "" {
		name = allstyles.DefaultPaletteName(styleName)
	}
	return covergen.PaletteByName(name)
}

// isPaletteStyle reports whether a style is colored by a covergen.Palette (i.e.
// declares palette.* knobs). svg is not, so it gets no picker or swatches.
func isPaletteStyle(s covergen.Style) bool {
	for _, k := range s.Knobs() {
		if strings.HasPrefix(k.Name, "palette.") {
			return true
		}
	}
	return false
}

// hexRGBA formats a colour as #rrggbb, dropping alpha (palette colours are opaque).
func hexRGBA(c color.RGBA) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func main() {
	addr := flag.String("addr", ":8099", "listen address")
	svgFlag := flag.String("svgdir", "libs/covergen/svg", "path to the svg style package (candidates/ + assets/)")
	flag.Parse()
	svgDir = *svgFlag

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handleOverview)
	mux.HandleFunc("GET /style/{style}", handleStyle)
	mux.HandleFunc("GET /img", handleImg)
	mux.HandleFunc("GET /colors", handleColors)
	mux.HandleFunc("GET /setup", handleSetup)
	mux.HandleFunc("GET /fonts", handleFonts)
	if svg.Enabled {
		mux.HandleFunc("GET /svg", handleSvg)
		mux.HandleFunc("GET /svg/img", handleSvgImg)
		mux.HandleFunc("POST /svg/promote", handleSvgPromote)
		mux.HandleFunc("POST /svg/reject", handleSvgReject)
		mux.HandleFunc("POST /svg/demote", handleSvgDemote)
	}

	log.Printf("covergenlab listening on http://localhost%s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil { //nolint:gosec // G114: dev-only localhost tuning tool, never linked into the server binary; request timeouts are unnecessary
		log.Fatal(err)
	}
}

// parseOverrides reads a style's declared knob values out of the query string,
// ignoring unknown or non-numeric params and clamping each to its [Min, Max].
//
// It takes the style's []covergen.Knob rather than the covergen.Style itself
// because []covergen.Knob is all this function needs; callers resolve the
// style through the Generator and pass style.Knobs() through.
func parseOverrides(knobs []covergen.Knob, q url.Values) map[string]float64 {
	out := make(map[string]float64)
	for _, k := range knobs {
		raw := q.Get(k.Name)
		if raw == "" {
			continue
		}
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			continue
		}
		if v < k.Min {
			v = k.Min
		}
		if v > k.Max {
			v = k.Max
		}
		out[k.Name] = v
	}
	return out
}

// parseText reads the main/subtitle overlay strings from the query.
func parseText(q url.Values) covergen.Text {
	return covergen.Text{Main: q.Get("main"), Subtitle: q.Get("sub")}
}

// styleTextClass returns a style's declared text class, or "" if it draws no text.
func styleTextClass(s covergen.Style) covergen.FontClass {
	if td, ok := s.(covergen.TextDrawer); ok {
		return td.TextClass()
	}
	return ""
}

// classPinned overrides the requested class with a fixed one (lab class picker).
type classPinned struct {
	covergen.FontProvider
	class covergen.FontClass
}

func (c classPinned) Random(rng *rand.Rand, _ covergen.FontClass) covergen.Font {
	return c.FontProvider.Random(rng, c.class)
}

// namePinned always returns one font (lab specific-font picker), still consuming
// one rng draw so placement matches the unpinned render.
type namePinned struct {
	covergen.FontProvider
	font covergen.Font
}

func (n namePinned) Random(rng *rand.Rand, _ covergen.FontClass) covergen.Font {
	if rng != nil {
		// Draw exactly one uint64, matching the cost of provider.Random's
		// pool[rng.IntN(len(pool))] (today's pools are len 1, i.e. IntN(1),
		// which itself draws one uint64 via IntN's power-of-two mask path) so
		// placement rng draws downstream stay aligned with an unpinned render.
		_ = rng.Uint64()
	}
	return n.font
}

// fontProviderFor resolves the provider for a request: a pinned font (?font=)
// wins, then a pinned class (?fontClass=), else the default library.
func fontProviderFor(q url.Values) covergen.FontProvider {
	if name := q.Get("font"); name != "" {
		if f, ok := fontLib.ByName(name); ok {
			return namePinned{FontProvider: fontLib, font: f}
		}
	}
	if class := q.Get("fontClass"); class != "" {
		return classPinned{FontProvider: fontLib, class: covergen.FontClass(class)}
	}
	return fontLib
}

// setupSnippet renders a paste-ready Go fragment that reconstructs style exactly
// as tuned in the lab. It constructs the style on paletteName, collects the knobs
// whose value differs from their Default into an overrides map (kept in Knobs()
// order so the output is deterministic), and calls the matching Generator method:
// GenerateWithKnobs when anything was tuned, the simpler GenerateStyle when every
// knob is at its default. Palette-less styles (svg) are asset-driven, so they get
// an svg.New(yourSVG) constructor instead of a covergen.PaletteByName one. When
// text carries a Main or Subtitle, the snippet instead declares a covergen.Text
// and switches to GenerateWithText against the real fonts.Default() provider —
// the lab's pinned-font/class wrappers are a preview-only seam, not something
// worth hardcoding into copy-paste setup code. An empty text reproduces today's
// textless output exactly.
func setupSnippet(style covergen.Style, paletteName string, values map[string]float64, text covergen.Text) string {
	name := style.Name()

	type override struct {
		name string
		val  float64
	}
	var overrides []override
	for _, k := range style.Knobs() {
		if v, ok := values[k.Name]; ok && v != k.Default {
			overrides = append(overrides, override{k.Name, v})
		}
	}

	palette := isPaletteStyle(style)
	ctorArg := "yourSVG"
	if palette {
		ctorArg = fmt.Sprintf("covergen.PaletteByName(%q)", paletteName)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "// covergenlab setup for style %q.\n", name)
	b.WriteString("// Imports: github.com/andresbott/aether/libs/covergen\n")
	fmt.Fprintf(&b, "//          github.com/andresbott/aether/libs/covergen/%s\n", name)
	if !text.Empty() {
		b.WriteString("//          github.com/andresbott/aether/libs/covergen/fonts\n")
	}
	if !palette {
		b.WriteString("// svg is asset-driven: replace yourSVG with your []byte SVG source.\n")
	}
	fmt.Fprintf(&b, "style := %s.New(%s)\n", name, ctorArg)

	if len(overrides) == 0 && text.Empty() {
		b.WriteString("// All knobs at their defaults; no overrides needed.\n")
		b.WriteString("png, err := covergen.New(style).GenerateStyle(seed, 512, style)\n")
		return b.String()
	}

	overridesExpr := "nil"
	if len(overrides) > 0 {
		b.WriteString("overrides := map[string]float64{\n")
		for _, o := range overrides {
			fmt.Fprintf(&b, "\t%q: %s,\n", o.name, formatKnob(o.val))
		}
		b.WriteString("}\n")
		overridesExpr = "overrides"
	} else {
		b.WriteString("// All knobs at their defaults; no overrides needed.\n")
	}

	if text.Empty() {
		b.WriteString("png, err := covergen.New(style).GenerateWithKnobs(seed, 512, style, overrides)\n")
		return b.String()
	}

	fmt.Fprintf(&b, "text := covergen.Text{Main: %q, Subtitle: %q}\n", text.Main, text.Subtitle)
	fmt.Fprintf(&b, "png, err := covergen.New(style).GenerateWithText(seed, 512, style, %s, text, fonts.Default())\n", overridesExpr)
	return b.String()
}

// formatKnob renders a knob value with the fewest digits that round-trips it,
// so 4 stays "4" and 1.2 stays "1.2" rather than "4.000000".
func formatKnob(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// randomSeed returns an arbitrary high-entropy string. covergen hashes the
// seed, so any content works; this simply spreads samples across the space.
func randomSeed() string {
	return strconv.FormatUint(rand.Uint64(), 36) + strconv.FormatUint(rand.Uint64(), 36) //nolint:gosec // G404: spreads lab sample seeds across the space; not security-sensitive
}

func randomSeeds(n int) []string {
	s := make([]string, n)
	for i := range s {
		s[i] = randomSeed()
	}
	return s
}

// handleImg renders one cover as PNG, fresh per request (no caching) so both
// code edits (after a restart) and knob edits (live) are reflected immediately.
// Text and font resolution mirror the live query: an empty Text or the default
// (unpinned) FontProvider reproduce the textless render exactly (see
// GenerateWithText).
func handleImg(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pal := paletteFor(q.Get("style"), q)
	style, ok := styleWithPalette(q.Get("style"), pal)
	if !ok {
		http.Error(w, "unknown style", http.StatusBadRequest)
		return
	}
	size := thumbSize
	if s, err := strconv.Atoi(q.Get("size")); err == nil && s > 0 {
		size = s
	}
	png, err := covergen.New(style).GenerateWithText(
		q.Get("seed"), size, style, parseOverrides(style.Knobs(), q),
		parseText(q), fontProviderFor(q))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
}

// handleColors returns the palette's four role colours for one seed under the
// current knobs, as JSON hex, so the style page can show live swatches next to
// the palette picker. It mirrors handleImg's palette/style/knob resolution.
// Styles without a palette (svg) have no role colours and return 404.
func handleColors(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pal := paletteFor(q.Get("style"), q)
	style, ok := styleWithPalette(q.Get("style"), pal)
	if !ok {
		http.Error(w, "unknown style", http.StatusBadRequest)
		return
	}
	if !isPaletteStyle(style) {
		http.Error(w, "style has no palette", http.StatusNotFound)
		return
	}
	cs := covergen.Colors(pal, q.Get("seed"), style.Knobs(), parseOverrides(style.Knobs(), q))
	writeJSON(w, map[string]string{
		"background": hexRGBA(cs.Background),
		"ink":        hexRGBA(cs.Ink),
		"accent1":    hexRGBA(cs.Accent1),
		"accent2":    hexRGBA(cs.Accent2),
	})
}

// handleSetup returns a paste-ready Go snippet reconstructing the current style
// under its live knob values (see setupSnippet). It mirrors handleImg's palette/
// style/knob resolution so the snippet reproduces exactly what the grid shows.
func handleSetup(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pal := paletteFor(q.Get("style"), q)
	style, ok := styleWithPalette(q.Get("style"), pal)
	if !ok {
		http.Error(w, "unknown style", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = fmt.Fprint(w, setupSnippet(style, pal.Name(), parseOverrides(style.Knobs(), q), parseText(q)))
}

// handleFonts lists font names for ?class= (all fonts when class is empty), as
// JSON, so the style page can repopulate its specific-font picker.
func handleFonts(w http.ResponseWriter, r *http.Request) {
	pool := fontLib.All()
	if c := r.URL.Query().Get("class"); c != "" {
		pool = fontLib.ByClass(covergen.FontClass(c))
	}
	names := make([]string, len(pool))
	for i, f := range pool {
		names[i] = f.Name()
	}
	writeJSON(w, names)
}

func listSVGs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".svg" {
			out = append(out, e.Name())
		}
	}
	return out
}

func handleSvg(w http.ResponseWriter, _ *http.Request) {
	data := struct {
		Candidates []string
		Rotation   []string
		Seeds      []string
		Size       int
	}{
		Candidates: listSVGs(candidatesDir()),
		Rotation:   listSVGs(assetsDir()),
		Seeds:      randomSeeds(overviewPerStyle),
		Size:       thumbSize,
	}
	render(w, svgTmpl, data)
}

// handleSvgImg renders one candidate/asset as a real cover via a single-asset
// svg.Style, so contenders are judged parametrized, not as raw SVG.
func handleSvgImg(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dir := assetsDir()
	if q.Get("dir") == "candidates" {
		dir = candidatesDir()
	}
	name := filepath.Base(q.Get("file"))
	src, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // G304: name is filepath.Base of a query param (traversal stripped), and dir is always one of the two hardcoded lab-local directories, never a raw request path
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	size := thumbSize
	if s, err := strconv.Atoi(q.Get("size")); err == nil && s > 0 {
		size = s
	}
	st := svg.New(src)
	png, err := covergen.New(st).GenerateWithKnobs(q.Get("seed"), size, st, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png) //nolint:gosec // G705: response is a freshly generated PNG image with an explicit image/png Content-Type, never HTML or user text
}

func handleSvgPromote(w http.ResponseWriter, r *http.Request) {
	if err := svg.Promote(candidatesDir(), assetsDir(), r.FormValue("file")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/svg", http.StatusSeeOther)
}

func handleSvgReject(w http.ResponseWriter, r *http.Request) {
	if err := svg.Reject(candidatesDir(), r.FormValue("file")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/svg", http.StatusSeeOther)
}

func handleSvgDemote(w http.ResponseWriter, r *http.Request) {
	if err := svg.Demote(assetsDir(), candidatesDir(), r.FormValue("file")); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/svg", http.StatusSeeOther)
}

type overviewRow struct {
	Name  string
	Seeds []string
}

func handleOverview(w http.ResponseWriter, _ *http.Request) {
	data := struct {
		Rows []overviewRow
		Size int
		Main string
		Sub  string
	}{Size: thumbSize, Main: placeholderMain, Sub: placeholderSub}
	for _, s := range gen.Styles() {
		data.Rows = append(data.Rows, overviewRow{Name: s.Name(), Seeds: randomSeeds(overviewPerStyle)})
	}
	render(w, overviewTmpl, data)
}

func handleStyle(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	pal := paletteFor(r.PathValue("style"), q)
	style, ok := styleWithPalette(r.PathValue("style"), pal)
	if !ok {
		http.Error(w, "unknown style", http.StatusNotFound)
		return
	}
	overrides := parseOverrides(style.Knobs(), q)

	// Split the style's knobs into its own controls and the injected palette
	// controls, so the page can group the palette picker, its knobs, and the
	// colour swatches together (see styleTmpl). Grain is a plain style knob.
	values := make(map[string]float64)
	var styleKnobs, paletteKnobs []covergen.Knob
	for _, k := range style.Knobs() {
		if v, ok := overrides[k.Name]; ok {
			values[k.Name] = v
		} else {
			values[k.Name] = k.Default
		}
		if strings.HasPrefix(k.Name, "palette.") {
			paletteKnobs = append(paletteKnobs, k)
		} else {
			styleKnobs = append(styleKnobs, k)
		}
	}

	hasPalette := len(paletteKnobs) > 0
	selPalette := ""
	if hasPalette {
		selPalette = pal.Name()
	}
	// Swatch rows reuse the first few tuning seeds so the previewed colours line
	// up with the first thumbnails in the grid.
	seeds := randomSeeds(tuningGrid)
	swatch := seeds
	if len(swatch) > swatchSeeds {
		swatch = swatch[:swatchSeeds]
	}

	// The font-class picker preselects the style's own declared class, unless an
	// explicit ?fontClass= names another one (mirroring paletteFor). The
	// specific-font picker then lists that class's fonts, falling back to every
	// font when the style draws no text (class ""), matching handleFonts.
	selClass := covergen.FontClass(q.Get("fontClass"))
	if selClass == "" {
		selClass = styleTextClass(style)
	}
	fontPool := fontLib.All()
	if selClass != "" {
		fontPool = fontLib.ByClass(selClass)
	}
	fontNames := make([]string, len(fontPool))
	for i, f := range fontPool {
		fontNames[i] = f.Name()
	}

	data := struct {
		Name            string
		StyleKnobs      []covergen.Knob
		PaletteKnobs    []covergen.Knob
		Values          map[string]float64
		Palettes        []string
		Palette         string
		HasPalette      bool
		Seeds           []string
		SwatchSeeds     []string
		Size            int
		TextClass       string
		FontClasses     []covergen.FontClass
		Fonts           []string
		Font            string
		Main            string
		Sub             string
		PlaceholderMain string
		PlaceholderSub  string
	}{
		Name:            style.Name(),
		StyleKnobs:      styleKnobs,
		PaletteKnobs:    paletteKnobs,
		Values:          values,
		Palettes:        paletteNames(),
		Palette:         selPalette,
		HasPalette:      hasPalette,
		Seeds:           seeds,
		SwatchSeeds:     swatch,
		Size:            thumbSize,
		TextClass:       string(selClass),
		FontClasses:     fontLib.Classes(),
		Fonts:           fontNames,
		Font:            q.Get("font"),
		Main:            q.Get("main"),
		Sub:             q.Get("sub"),
		PlaceholderMain: placeholderMain,
		PlaceholderSub:  placeholderSub,
	}
	render(w, styleTmpl, data)
}

func render(w http.ResponseWriter, t *template.Template, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.Execute(w, data); err != nil {
		log.Printf("covergenlab: template: %v", err)
	}
}

var baseCSS = `
  :root { color-scheme: dark; }
  body { margin: 0; background: #111; color: #ddd; font: 14px/1.4 system-ui, sans-serif; }
  a { color: #7db7ff; text-decoration: none; }
  header { padding: 12px 20px; border-bottom: 1px solid #222; position: sticky; top: 0; background: #111; z-index: 2; display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .text-toggle { background: #223; color: #cde; border: 1px solid #345; border-radius: 6px; padding: 6px 10px; cursor: pointer; font: inherit; white-space: nowrap; }
  .text-toggle[aria-pressed="true"] { background: #1e5631; color: #d7ffe6; border-color: #2b7a4b; }
  h1 { font-size: 16px; margin: 0; }
  h2 { font-size: 14px; margin: 0 0 8px; }
  main { padding: 20px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 10px; }
  .grid img { width: 100%; aspect-ratio: 1; border-radius: 6px; background: #1a1a1a; display: block; }
  section { margin-bottom: 28px; }
`

var overviewTmpl = template.Must(template.New("overview").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>covergenlab</title><style>` + baseCSS + `
  .styles { display: grid; grid-template-columns: repeat(auto-fill, minmax(340px, 1fr)); gap: 16px; align-items: start; }
  .style-card { background: #161616; border: 1px solid #222; border-radius: 8px; padding: 12px; margin: 0; }
  .style-card h2 { margin: 0 0 10px; }
  .style-card .grid { grid-template-columns: repeat(auto-fill, minmax(92px, 1fr)); gap: 8px; }
  @media (min-width: 1100px) {
    .styles { grid-template-columns: repeat(2, 1fr); }
    .style-card .grid { grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); }
  }
</style></head>
<body>
<header>
  <h1>covergenlab — cover-art styles</h1>
  <button type="button" class="text-toggle" id="text-toggle" aria-pressed="true">Text: on</button>
</header>
<main>
<div class="styles">
{{range .Rows}}{{$name := .Name}}
  <section class="style-card">
    <h2><a href="/style/{{$name}}">{{$name}} →</a>{{if eq $name "svg"}} &nbsp;·&nbsp; <a href="/svg">svg curation →</a>{{end}}</h2>
    <div class="grid">
      {{range .Seeds}}<img class="cell" data-style="{{$name}}" data-seed="{{.}}" title="{{.}}">{{end}}
    </div>
  </section>
{{end}}
</div>
</main>
<script>
  var TEXT_ON = true;
  var PH_MAIN = "{{.Main}}", PH_SUB = "{{.Sub}}", SIZE = "{{.Size}}";
  function renderCells() {
    document.querySelectorAll('img.cell').forEach(function (img) {
      var u = new URLSearchParams();
      u.set('style', img.dataset.style);
      u.set('seed', img.dataset.seed);
      u.set('size', SIZE);
      if (TEXT_ON) { u.set('main', PH_MAIN); u.set('sub', PH_SUB); }
      img.src = '/img?' + u.toString();
    });
  }
  document.addEventListener('DOMContentLoaded', function () {
    var t = document.getElementById('text-toggle');
    t.addEventListener('click', function () {
      TEXT_ON = !TEXT_ON;
      t.setAttribute('aria-pressed', TEXT_ON ? 'true' : 'false');
      t.textContent = TEXT_ON ? 'Text: on' : 'Text: off';
      renderCells();
    });
    renderCells();
  });
</script>
</body></html>`))

var styleTmpl = template.Must(template.New("style").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>{{.Name}} — covergenlab</title><style>` + baseCSS + `
  .layout { display: grid; grid-template-columns: minmax(500px, 560px) minmax(0, 1fr); gap: 20px; align-items: start; }
  .grid { grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); }
  .knobs { position: sticky; top: 60px; background: #161616; border: 1px solid #222; border-radius: 8px; padding: 14px; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px 18px; align-items: start; }
  .knob { margin-bottom: 14px; }
  .knob label { display: flex; justify-content: space-between; margin-bottom: 4px; }
  .knob output { color: #7db7ff; font-variant-numeric: tabular-nums; }
  .knob input { width: 100%; }
  .controls { grid-column: 1 / -1; display: flex; gap: 8px; margin-top: 4px; }
  button { background: #223; color: #cde; border: 1px solid #345; border-radius: 6px; padding: 6px 10px; cursor: pointer; }
  .empty { color: #666; }
  .group { min-width: 0; }
  .group h3 { font-size: 12px; text-transform: uppercase; letter-spacing: .06em; color: #9aa; margin: 0 0 8px; }
  select#palette, select#fontClass, select#font { width: 100%; margin-bottom: 12px; background: #101010; color: #ddd; border: 1px solid #345; border-radius: 6px; padding: 6px; }
  .knob input[type="text"] { width: 100%; box-sizing: border-box; background: #101010; color: #ddd; border: 1px solid #345; border-radius: 6px; padding: 6px; }
  .swatches { display: flex; flex-direction: column; gap: 6px; margin-top: 10px; }
  .swatch-row { display: flex; gap: 4px; }
  .swatch { flex: 1; height: 22px; border-radius: 4px; background: #000; border: 1px solid rgba(0,0,0,.4); }
  dialog#setup { width: min(680px, 92vw); padding: 0; background: #161616; color: #ddd; border: 1px solid #345; border-radius: 8px; }
  dialog#setup::backdrop { background: rgba(0,0,0,.6); }
  dialog#setup pre { margin: 0; padding: 14px; max-height: 62vh; overflow: auto; background: #0d0d0d; border-radius: 8px 8px 0 0; font: 12px/1.5 ui-monospace, SFMono-Regular, Menlo, monospace; white-space: pre; color: #cde; }
  dialog#setup .dlg-actions { display: flex; gap: 8px; justify-content: flex-end; padding: 12px 14px; }
</style></head>
<body>
<header>
  <h1><a href="/">← styles</a> &nbsp;/&nbsp; {{.Name}}</h1>
  <button type="button" class="text-toggle" id="text-toggle" aria-pressed="true">Text: on</button>
</header>
<main>
  <div class="layout">
    <div class="knobs">
      <div class="group">
        <h3>Style</h3>
        {{range .StyleKnobs}}
        <div class="knob">
          <label><span>{{.Label}}</span> <output>{{index $.Values .Name}}</output></label>
          <input type="range" data-knob="{{.Name}}" min="{{.Min}}" max="{{.Max}}" step="{{.Step}}" value="{{index $.Values .Name}}">
        </div>
        {{end}}
      </div>
      {{if .HasPalette}}
      <div class="group">
        <h3>Palette</h3>
        <select id="palette">
          {{range .Palettes}}<option value="{{.}}"{{if eq . $.Palette}} selected{{end}}>{{.}}</option>{{end}}
        </select>
        {{range .PaletteKnobs}}
        <div class="knob">
          <label><span>{{.Label}}</span> <output>{{index $.Values .Name}}</output></label>
          <input type="range" data-knob="{{.Name}}" min="{{.Min}}" max="{{.Max}}" step="{{.Step}}" value="{{index $.Values .Name}}">
        </div>
        {{end}}
        <div class="swatches">
          {{range .SwatchSeeds}}<div class="swatch-row" data-seed="{{.}}" title="{{.}}"><span class="swatch"></span><span class="swatch"></span><span class="swatch"></span><span class="swatch"></span></div>{{end}}
        </div>
      </div>
      {{end}}
      <div class="group">
        <h3>Text</h3>
        <div class="knob">
          <label><span>Main</span></label>
          <input type="text" id="text-main" placeholder="{{.PlaceholderMain}}" value="{{.Main}}">
        </div>
        <div class="knob">
          <label><span>Subtitle</span></label>
          <input type="text" id="text-sub" placeholder="{{.PlaceholderSub}}" value="{{.Sub}}">
        </div>
        <select id="fontClass">
          {{range .FontClasses}}<option value="{{.}}"{{if eq . $.TextClass}} selected{{end}}>{{.}}</option>{{end}}
        </select>
        <select id="font">
          <option value="">(random)</option>
          {{range .Fonts}}<option value="{{.}}"{{if eq . $.Font}} selected{{end}}>{{.}}</option>{{end}}
        </select>
      </div>
      <div class="controls">
        <button id="reroll" title="new random seeds">Reroll</button>
        <button id="reset" title="back to defaults">Reset</button>
        <button id="copy-setup" title="Go code that reproduces this tuning">Copy setup</button>
      </div>
    </div>
    <div class="grid">
      {{range .Seeds}}<img class="cell" data-seed="{{.}}" title="{{.}}">{{end}}
    </div>
  </div>
  <dialog id="setup">
    <pre id="setup-code"></pre>
    <div class="dlg-actions">
      <button id="copy-clip">Copy to clipboard</button>
      <button id="close-setup">Close</button>
    </div>
  </dialog>
</main>
<script>
  const STYLE = "{{.Name}}";
  const SIZE = "{{.Size}}";
  const PALETTE = "{{.Palette}}"; // "" when the style has no palette (svg)
  let FONT_CLASS = "{{.TextClass}}"; // "" when the style draws no text (svg); changes live, not via reload
  const ROLES = ['background', 'ink', 'accent1', 'accent2'];
  let swatchToken = 0;
  let TEXT_ON = true; // "Text" toggle; when on, empty Main/Sub fall back to the placeholder
  const PH_MAIN = "{{.PlaceholderMain}}", PH_SUB = "{{.PlaceholderSub}}";

  function readKnobs() {
    const knobs = new URLSearchParams();
    document.querySelectorAll('input[data-knob]').forEach(function (inp) {
      knobs.set(inp.dataset.knob, inp.value);
      const out = inp.closest('.knob').querySelector('output');
      if (out) out.textContent = inp.value;
    });
    return knobs;
  }

  // updateSwatches repaints each palette-role swatch row from /colors, live with
  // the current knobs. The token guards against out-of-order fetch responses.
  function updateSwatches(knobs) {
    if (!PALETTE) return;
    const rows = document.querySelectorAll('.swatch-row');
    if (!rows.length) return;
    const token = ++swatchToken;
    rows.forEach(function (row) {
      const u = new URLSearchParams(knobs);
      u.set('style', STYLE);
      u.set('palette', PALETTE);
      u.set('seed', row.dataset.seed);
      fetch('/colors?' + u.toString()).then(function (r) {
        return r.ok ? r.json() : null;
      }).then(function (c) {
        if (!c || token !== swatchToken) return;
        row.querySelectorAll('.swatch').forEach(function (el, i) {
          const hex = c[ROLES[i]] || '#000';
          el.style.background = hex;
          el.title = ROLES[i] + ' ' + hex;
        });
      }).catch(function () {});
    });
  }

  // textAndFontParams reads the live text/font controls (main, sub, the pinned
  // font, and the pinned class) onto knobs, the same way readKnobs() reads the
  // sliders — so update() ends up with one query string covering all of it.
  function textAndFontParams(knobs) {
    const mainEl = document.getElementById('text-main');
    const subEl = document.getElementById('text-sub');
    const fontEl = document.getElementById('font');
    // With the Text toggle on, an empty field falls back to the placeholder so
    // covers preview text; a typed value overrides it. Toggle off = no overlay.
    if (TEXT_ON) {
      const main = (mainEl && mainEl.value) || PH_MAIN;
      const sub = (subEl && subEl.value) || PH_SUB;
      if (main) knobs.set('main', main);
      if (sub) knobs.set('sub', sub);
    }
    if (fontEl && fontEl.value) knobs.set('font', fontEl.value);
    if (FONT_CLASS) knobs.set('fontClass', FONT_CLASS);
    return knobs;
  }

  function update() {
    const knobs = textAndFontParams(readKnobs());
    document.querySelectorAll('img.cell').forEach(function (img) {
      const u = new URLSearchParams(knobs);
      u.set('style', STYLE);
      if (PALETTE) u.set('palette', PALETTE);
      u.set('size', SIZE);
      u.set('seed', img.dataset.seed);
      img.src = '/img?' + u.toString();
    });
    updateSwatches(knobs);
    const qs = new URLSearchParams(knobs);
    if (PALETTE) qs.set('palette', PALETTE);
    const s = qs.toString();
    history.replaceState(null, '', location.pathname + (s ? '?' + s : ''));
  }

  document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('input[data-knob]').forEach(function (inp) {
      inp.addEventListener('input', update);
    });
    const palSel = document.getElementById('palette');
    if (palSel) {
      palSel.addEventListener('change', function () {
        // Reload with the chosen palette; drop palette.* overrides so it starts
        // from the new palette's own defaults, but keep the style knobs.
        const q = new URLSearchParams();
        readKnobs().forEach(function (v, k) { if (k.indexOf('palette.') !== 0) q.set(k, v); });
        q.set('palette', palSel.value);
        location.href = location.pathname + '?' + q.toString();
      });
    }
    document.getElementById('reroll').addEventListener('click', function () { location.reload(); });
    document.getElementById('reset').addEventListener('click', function () {
      location.href = location.pathname + (PALETTE ? '?palette=' + encodeURIComponent(PALETTE) : '');
    });

    // Text inputs and the specific-font picker update live, like the knobs.
    const mainInput = document.getElementById('text-main');
    const subInput = document.getElementById('text-sub');
    const fontSel = document.getElementById('font');
    if (mainInput) mainInput.addEventListener('input', update);
    if (subInput) subInput.addEventListener('input', update);
    if (fontSel) fontSel.addEventListener('change', update);

    // The "Text" toggle enables/disables the overlay (placeholder-or-typed vs none).
    const textToggle = document.getElementById('text-toggle');
    if (textToggle) {
      textToggle.addEventListener('click', function () {
        TEXT_ON = !TEXT_ON;
        textToggle.setAttribute('aria-pressed', TEXT_ON ? 'true' : 'false');
        textToggle.textContent = TEXT_ON ? 'Text: on' : 'Text: off';
        update();
      });
    }

    // The font-class picker fetches /fonts for the newly chosen class and
    // repopulates the specific-font picker (keeping the pinned font only if it
    // still belongs to the new class), then updates the grid live — no reload.
    const fontClassSel = document.getElementById('fontClass');
    if (fontClassSel) {
      fontClassSel.addEventListener('change', function () {
        FONT_CLASS = fontClassSel.value;
        fetch('/fonts?class=' + encodeURIComponent(FONT_CLASS)).then(function (r) {
          return r.ok ? r.json() : [];
        }).then(function (names) {
          if (!fontSel) return;
          const prev = fontSel.value;
          fontSel.innerHTML = '';
          const optRandom = document.createElement('option');
          optRandom.value = '';
          optRandom.textContent = '(random)';
          fontSel.appendChild(optRandom);
          names.forEach(function (name) {
            const opt = document.createElement('option');
            opt.value = name;
            opt.textContent = name;
            fontSel.appendChild(opt);
          });
          fontSel.value = names.indexOf(prev) !== -1 ? prev : '';
        }).catch(function () {}).finally(update);
      });
    }

    // Copy setup: ask /setup for the Go snippet that reproduces the live knobs,
    // then show it in a modal with a clipboard button.
    const setupDlg = document.getElementById('setup');
    const setupCode = document.getElementById('setup-code');
    document.getElementById('copy-setup').addEventListener('click', function () {
      const u = readKnobs();
      u.set('style', STYLE);
      if (PALETTE) u.set('palette', PALETTE);
      if (mainInput && mainInput.value) u.set('main', mainInput.value);
      if (subInput && subInput.value) u.set('sub', subInput.value);
      fetch('/setup?' + u.toString()).then(function (r) {
        return r.ok ? r.text() : Promise.reject(new Error('setup ' + r.status));
      }).then(function (code) {
        setupCode.textContent = code;
      }).catch(function () {
        setupCode.textContent = '// failed to generate setup';
      }).finally(function () {
        if (typeof setupDlg.showModal === 'function') setupDlg.showModal();
      });
    });
    document.getElementById('copy-clip').addEventListener('click', function () {
      if (navigator.clipboard) navigator.clipboard.writeText(setupCode.textContent);
    });
    document.getElementById('close-setup').addEventListener('click', function () {
      setupDlg.close();
    });

    update();
  });
</script>
</body></html>`))

var svgTmpl = template.Must(template.New("svg").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>svg curation — covergenlab</title><style>` + baseCSS + `
  form.act { display: inline; }
  .item { margin-bottom: 8px; }
</style></head>
<body>
<header><h1><a href="/">← styles</a> &nbsp;/&nbsp; svg curation</h1></header>
<main>
  <section>
    <h2>Candidates ({{len .Candidates}})</h2>
    {{if .Candidates}}{{range .Candidates}}
      <div class="item">
        <div><strong>{{.}}</strong>
          <form class="act" method="post" action="/svg/promote"><input type="hidden" name="file" value="{{.}}"><button>Promote</button></form>
          <form class="act" method="post" action="/svg/reject"><input type="hidden" name="file" value="{{.}}"><button>Reject</button></form>
        </div>
        <div class="grid">{{$f := .}}{{range $.Seeds}}<img src="/svg/img?dir=candidates&file={{$f}}&seed={{.}}&size={{$.Size}}">{{end}}</div>
      </div>
    {{end}}{{else}}<p class="empty">No candidates in candidates/.</p>{{end}}
  </section>
  <section>
    <h2>Rotation ({{len .Rotation}})</h2>
    {{if .Rotation}}{{range .Rotation}}
      <div class="item">
        <div><strong>{{.}}</strong>
          <form class="act" method="post" action="/svg/demote"><input type="hidden" name="file" value="{{.}}"><button>Remove</button></form>
        </div>
        <div class="grid">{{$f := .}}{{range $.Seeds}}<img src="/svg/img?dir=assets&file={{$f}}&seed={{.}}&size={{$.Size}}">{{end}}</div>
      </div>
    {{end}}{{else}}<p class="empty">No assets in the rotation yet.</p>{{end}}
  </section>
</main>
</body></html>`))
