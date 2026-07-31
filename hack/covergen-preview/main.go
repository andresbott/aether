// Command covergen-preview renders a set of seed strings with every covergen
// style plus the deterministic auto-pick, writing PNGs and an index.html
// contact sheet to /tmp/covergen-preview for visual comparison.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andresbott/aether/internal/covergen"
)

const (
	outDir = "/tmp/covergen-preview"
	size   = 512
)

var seeds = []string{
	"Nina Simone",
	"Miles Davis - Kind of Blue",
	"Radiohead - OK Computer",
	"Aphex Twin",
	"Daft Punk - Discovery",
	"Billie Eilish",
	"Boards of Canada - Music Has the Right to Children",
	"Johann Sebastian Bach",
	"Kendrick Lamar - To Pimp a Butterfly",
	"Fleetwood Mac - Rumours",
	"Tame Impala",
	"Portishead - Dummy",
	"The Beatles - Abbey Road",
	"Björk - Homogenic",
	"Massive Attack - Mezzanine",
	"Miles Davis",
	"Sufjan Stevens - Illinois",
	"Deftones - White Pony",
	"Khruangbin",
	"Arvo Pärt - Tabula Rasa",
}

func main() {
	styles := covergen.Styles()
	cols := make([]string, 0, len(styles)+1)
	cols = append(cols, "auto")
	for _, s := range styles {
		cols = append(cols, s.String())
	}

	if err := os.RemoveAll(outDir); err != nil {
		fatal(err)
	}
	for _, c := range cols {
		if err := os.MkdirAll(filepath.Join(outDir, c), 0o750); err != nil {
			fatal(err)
		}
	}

	for si, seed := range seeds {
		name := fmt.Sprintf("%02d-%s.png", si+1, slug(seed))

		data, err := covergen.Generate(seed, size)
		if err != nil {
			fatal(fmt.Errorf("auto / %q: %w", seed, err))
		}
		if err := os.WriteFile(filepath.Join(outDir, "auto", name), data, 0o600); err != nil {
			fatal(err)
		}

		for _, st := range styles {
			data, err := covergen.GenerateStyle(seed, size, st)
			if err != nil {
				fatal(fmt.Errorf("%s / %q: %w", st, seed, err))
			}
			if err := os.WriteFile(filepath.Join(outDir, st.String(), name), data, 0o600); err != nil {
				fatal(err)
			}
		}
		fmt.Printf("rendered %2d/%d %s (auto=%s)\n", si+1, len(seeds), seed, covergen.StyleFor(seed))
	}

	if err := writeIndex(cols); err != nil {
		fatal(err)
	}
	fmt.Println("open", filepath.Join(outDir, "index.html"))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func slug(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}

func writeIndex(cols []string) error {
	var b strings.Builder
	b.WriteString(`<!doctype html><meta charset="utf-8"><title>covergen preview</title>
<style>
body{background:#141418;color:#ddd;font:14px/1.4 sans-serif;margin:20px}
table{border-collapse:collapse}
th,td{padding:6px;text-align:center;vertical-align:top}
th{position:sticky;top:0;background:#141418;font-size:15px}
td.seed{text-align:right;max-width:160px;font-size:12px;color:#aaa}
img{width:190px;height:190px;border-radius:8px;display:block}
</style><table><tr><th></th>`)
	for _, c := range cols {
		fmt.Fprintf(&b, "<th>%s</th>", c)
	}
	b.WriteString("</tr>\n")
	for si, seed := range seeds {
		fmt.Fprintf(&b, "<tr><td class=seed>%s</td>", seed)
		for _, c := range cols {
			fmt.Fprintf(&b, `<td><img loading=lazy src="%s/%02d-%s.png"></td>`, c, si+1, slug(seed))
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</table>\n")
	return os.WriteFile(filepath.Join(outDir, "index.html"), []byte(b.String()), 0o600)
}
