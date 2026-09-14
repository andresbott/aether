// Command covergenlab is a local, browser-based tuning lab for the covergen
// package. It renders every style across a spread of random seeds and exposes
// each style's knobs as live sliders, so tuning cover art is a drag-and-watch
// loop instead of edit-rebuild-squint.
//
// It is a dev tool: it lives under libs/covergen/lab and is never imported by the server
// binary, so it ships in nothing. Edits to covergen's algorithms show up on
// restart; knob edits are live. Run it with:
//
//	go run ./libs/covergen/lab              # then open http://localhost:8099
//	go run ./libs/covergen/lab -addr :9000
package main

import (
	"flag"
	"html/template"
	"log"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/andresbott/aether/libs/covergen"
	"github.com/andresbott/aether/libs/covergen/allstyles"
	"github.com/andresbott/aether/libs/covergen/svg"
)

const (
	overviewPerStyle = 6   // thumbnails per style on the overview
	tuningGrid       = 12  // seeds shown on a style's tuning page
	thumbSize        = 200 // px; rendered fresh per request (no cache)
)

// gen is the Generator over covergen's full built-in style set, plus svg for
// tuning/preview: svg stays opt-in and out of the shipped allstyles set, but
// the lab (a dev-only tool, never imported by the server binary) knows about
// it directly so it can be curated and tuned here.
var gen = covergen.New(append(allstyles.All(covergen.DefaultPalette()), svg.Style)...)

// svgDir is the on-disk location of the svg style package (candidates/ + assets/),
// set from the -svgdir flag; the curation tool reads and writes there.
var svgDir = "libs/covergen/svg"

func candidatesDir() string { return filepath.Join(svgDir, "candidates") }
func assetsDir() string     { return filepath.Join(svgDir, "assets") }

func main() {
	addr := flag.String("addr", ":8099", "listen address")
	svgFlag := flag.String("svgdir", "libs/covergen/svg", "path to the svg style package (candidates/ + assets/)")
	flag.Parse()
	svgDir = *svgFlag

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", handleOverview)
	mux.HandleFunc("GET /style/{style}", handleStyle)
	mux.HandleFunc("GET /img", handleImg)
	mux.HandleFunc("GET /svg", handleSvg)
	mux.HandleFunc("GET /svg/img", handleSvgImg)
	mux.HandleFunc("POST /svg/promote", handleSvgPromote)
	mux.HandleFunc("POST /svg/reject", handleSvgReject)
	mux.HandleFunc("POST /svg/demote", handleSvgDemote)

	log.Printf("covergenlab listening on http://localhost%s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
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

// randomSeed returns an arbitrary high-entropy string. covergen hashes the
// seed, so any content works; this simply spreads samples across the space.
func randomSeed() string {
	return strconv.FormatUint(rand.Uint64(), 36) + strconv.FormatUint(rand.Uint64(), 36)
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
func handleImg(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	style, ok := gen.ByName(q.Get("style"))
	if !ok {
		http.Error(w, "unknown style", http.StatusBadRequest)
		return
	}
	size := thumbSize
	if s, err := strconv.Atoi(q.Get("size")); err == nil && s > 0 {
		size = s
	}
	png, err := gen.GenerateWithKnobs(q.Get("seed"), size, style, parseOverrides(style.Knobs(), q))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(png)
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
	}{Size: thumbSize}
	for _, s := range gen.Styles() {
		data.Rows = append(data.Rows, overviewRow{Name: s.Name(), Seeds: randomSeeds(overviewPerStyle)})
	}
	render(w, overviewTmpl, data)
}

func handleStyle(w http.ResponseWriter, r *http.Request) {
	style, ok := gen.ByName(r.PathValue("style"))
	if !ok {
		http.Error(w, "unknown style", http.StatusNotFound)
		return
	}
	overrides := parseOverrides(style.Knobs(), r.URL.Query())
	values := make(map[string]float64)
	for _, k := range style.Knobs() {
		if v, ok := overrides[k.Name]; ok {
			values[k.Name] = v
		} else {
			values[k.Name] = k.Default
		}
	}
	data := struct {
		Name   string
		Knobs  []covergen.Knob
		Values map[string]float64
		Seeds  []string
		Size   int
	}{
		Name:   style.Name(),
		Knobs:  style.Knobs(),
		Values: values,
		Seeds:  randomSeeds(tuningGrid),
		Size:   thumbSize,
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
  header { padding: 12px 20px; border-bottom: 1px solid #222; position: sticky; top: 0; background: #111; z-index: 2; }
  h1 { font-size: 16px; margin: 0; }
  h2 { font-size: 14px; margin: 0 0 8px; }
  main { padding: 20px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(140px, 1fr)); gap: 10px; }
  .grid img { width: 100%; aspect-ratio: 1; border-radius: 6px; background: #1a1a1a; display: block; }
  section { margin-bottom: 28px; }
`

var overviewTmpl = template.Must(template.New("overview").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>covergenlab</title><style>` + baseCSS + `</style></head>
<body>
<header><h1>covergenlab — cover-art styles</h1></header>
<main>
{{range .Rows}}{{$name := .Name}}
  <section>
    <h2><a href="/style/{{$name}}">{{$name}} →</a>{{if eq $name "svg"}} &nbsp;·&nbsp; <a href="/svg">svg curation →</a>{{end}}</h2>
    <div class="grid">
      {{range .Seeds}}<img src="/img?style={{$name}}&seed={{.}}&size={{$.Size}}" title="{{.}}">{{end}}
    </div>
  </section>
{{end}}
</main>
</body></html>`))

var styleTmpl = template.Must(template.New("style").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>{{.Name}} — covergenlab</title><style>` + baseCSS + `
  .layout { display: grid; grid-template-columns: 260px 1fr; gap: 20px; align-items: start; }
  .knobs { position: sticky; top: 60px; background: #161616; border: 1px solid #222; border-radius: 8px; padding: 14px; }
  .knob { margin-bottom: 14px; }
  .knob label { display: flex; justify-content: space-between; margin-bottom: 4px; }
  .knob output { color: #7db7ff; font-variant-numeric: tabular-nums; }
  .knob input { width: 100%; }
  .controls { display: flex; gap: 8px; margin-top: 8px; }
  button { background: #223; color: #cde; border: 1px solid #345; border-radius: 6px; padding: 6px 10px; cursor: pointer; }
  .empty { color: #666; }
</style></head>
<body>
<header><h1><a href="/">← styles</a> &nbsp;/&nbsp; {{.Name}}</h1></header>
<main>
  <div class="layout">
    <div class="knobs">
      {{if .Knobs}}
      {{range .Knobs}}
        <div class="knob">
          <label><span>{{.Label}}</span> <output>{{index $.Values .Name}}</output></label>
          <input type="range" data-knob="{{.Name}}" min="{{.Min}}" max="{{.Max}}" step="{{.Step}}" value="{{index $.Values .Name}}">
        </div>
      {{end}}
      {{else}}
        <p class="empty">This style has no knobs yet.</p>
      {{end}}
      <div class="controls">
        <button id="reroll" title="new random seeds">Reroll</button>
        <button id="reset" title="back to defaults">Reset</button>
      </div>
    </div>
    <div class="grid">
      {{range .Seeds}}<img class="cell" data-seed="{{.}}" title="{{.}}">{{end}}
    </div>
  </div>
</main>
<script>
  const STYLE = "{{.Name}}";
  const SIZE = "{{.Size}}";
  function update() {
    const knobs = new URLSearchParams();
    document.querySelectorAll('input[data-knob]').forEach(function (inp) {
      knobs.set(inp.dataset.knob, inp.value);
      const out = inp.closest('.knob').querySelector('output');
      if (out) out.textContent = inp.value;
    });
    document.querySelectorAll('img.cell').forEach(function (img) {
      const u = new URLSearchParams(knobs);
      u.set('style', STYLE);
      u.set('size', SIZE);
      u.set('seed', img.dataset.seed);
      img.src = '/img?' + u.toString();
    });
    const qs = knobs.toString();
    history.replaceState(null, '', location.pathname + (qs ? '?' + qs : ''));
  }
  document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('input[data-knob]').forEach(function (inp) {
      inp.addEventListener('input', update);
    });
    document.getElementById('reroll').addEventListener('click', function () { location.reload(); });
    document.getElementById('reset').addEventListener('click', function () { location.href = location.pathname; });
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
