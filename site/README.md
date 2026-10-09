# Aether website

The public site — landing page and user documentation — built with
[Hugo](https://gohugo.io) and the [hugo-book](https://github.com/alex-shpak/hugo-book)
theme, published to GitHub Pages by `.github/workflows/site.yml` on every
stable release.

Internal and agent docs stay in `/docs`; this directory is only what users read.

## Run it locally

Needs Hugo **extended** ≥ 0.146 and Go (Hugo fetches the theme as a module).
From this directory (`make help` lists everything):

```sh
make serve                          # live reload on http://localhost:1313/aether/
make check                          # does it build? catches links to missing pages
make build                          # what CI runs, output in public/
make clean                          # drop public/ and Hugo's resource cache
make new PAGE=docs/guides/my-page.md
```

From the repo root, `make site-serve`, `make site-build` and `make site-check`
do the same.

## Layout

| Path | |
|---|---|
| `content/_index.md` | the landing page: hero copy and screenshot in the front matter, the rest in markdown |
| `content/docs/` | the documentation; its tree is the sidebar |
| `hugo.toml` | site config, theme params, landing and sidebar menus |
| `assets/_custom.scss` | the `aether` theme: the web UI's palette on hugo-book, plus the landing page and screenshot styles |
| `assets/screenshots/` | screenshots of the web UI, see [Screenshots](#screenshots) |
| `assets/icons/` | the Material Symbols glyphs the landing page's nav and highlights use, see [Icons](#icons) |
| `layouts/landing.html` | the landing page, standalone (not the theme's layout) |
| `layouts/_partials/`, `layouts/_shortcodes/`, `layouts/_markup/` | theme overrides (menu as the nav rail, wordmark, alert titles) and the site's shortcodes |
| `static/fonts/` | Inter, the web UI's typeface, self-hosted |
| `archetypes/docs.md` | front matter for new pages (`make new`); named `docs` to override the theme's own |

The look is the web UI's: `assets/_custom.scss` uses the same `--app-*` token
names and values as `webui/src/assets/scss/_variables.scss` — change a colour
there, change it here. The brand mark and favicon are mounted from `webui/`
(see `hugo.toml`), so they never drift.

## Screenshots

Pages show screenshots with the `screenshot` shortcode:

```markdown
{{< screenshot name="settings-tasks" title="Settings → Tasks, where the catalog scan runs" >}}
```

Until `assets/screenshots/settings-tasks.png` exists, a placeholder stands in:
an outline of the app with the title and the file it expects. To replace it,
save the capture under that name (`.png`, `.webp` or `.jpg`) — no markdown
change. An optional `settings-tasks-dark.png` next to it is shown to readers in
dark mode. `make serve` only notices a newly added file after a restart.

| Parameter | |
|---|---|
| `name` | the file name under `assets/screenshots/`, without extension |
| `title` | caption and alt text |
| `kind` | the placeholder's shape: `desktop` (16:10, default), `phone` (9:19.5), `panel` (4:3, a dialog or section), `rail` (1:2, a crop of the sidebar) |
| `ratio` | overrides the placeholder's aspect ratio, e.g. `16 / 7` |
| `width` | caps the figure's width, e.g. `17rem` |

Captures expected so far (`grep -rn 'screenshot name=' content`; the landing
page's hero is `heroShot` in its front matter):

| File | Where |
|---|---|
| `library` | landing page hero: album grid, play queue, player bar (desktop) |
| `settings-tasks` | Quickstart: Settings → Tasks |
| `settings-scan-folders` | Configuration: the read-only scan folders panel on Settings → Libraries |
| `library-dialog` | Libraries: the library dialog with its filters |
| `sidebar-libraries` | Libraries: the sidebar with the main library and a library per view |
| `connected-apps` | Connecting apps: User settings → Connected apps |
| `sign-in` | Authentication: the sign-in page |
| `settings-users` | Authentication: Settings → Users |

Capture at 1440×900 (desktop) or 390×844 (phone) with a scanned library and
something queued, so views are not empty. The landing hero (`library`) is the
exception, at 1680×1050: at 1440 the album grid beside the open queue shows only
two columns.

## Icons

The landing page's nav (`params.icon` on the `home` menu in `hugo.toml`) and
highlights (`feature icon=`) name a file in `assets/icons/`, inlined by
`layouts/_partials/icon.html`. They are the web UI's glyphs — Material Symbols,
its filled Rounded style — copied from the web UI's icon package. To add one,
from `webui/` (with `npm install` done):

```sh
node -e 'const s = require("@iconify-json/material-symbols/icons.json"), n = process.argv[1];
  const i = s.icons[n + "-rounded"] ?? s.icons[s.aliases[n + "-rounded"]?.parent];
  require("fs").writeFileSync(`../site/assets/icons/${n}.svg`,
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" aria-hidden="true">${i.body}</svg>\n`)' menu-book
```

Names are the app's, kebab-cased: `ms-menu-book` in the app is `menu-book` here.

The one exception is `github.svg`, GitHub's mark (`mark-github` from
[Octicons](https://github.com/primer/octicons), MIT), which Material Symbols
does not have. Its viewBox is padded from `0 0 16 16` to `-2 -2 20 20` so it
sits at the same optical size as the Material glyphs, whose 24-unit box leaves
about 2 units of air on each side.

## Writing pages

- **Link to pages by file**: `[Libraries](/docs/guides/libraries.md)`, anchors
  included (`…/configuration.md#scan-folders`). The theme resolves them to the
  right URL and **fails the build on a missing page** (`BookPortableLinks`).
  Do not hard-code `/aether/…` URLs.
- **Callouts** are GitHub alerts, so they render on GitHub too:

  ```markdown
  > [!WARNING]
  > Removing a scan folder removes its tracks.
  ```

  `NOTE`, `TIP`, `IMPORTANT`, `WARNING` and `CAUTION` are available.
- **Ordering**: `weight` in the front matter, lower first. Sections are folders
  with an `_index.md`.
- Useful front matter: `bookToc: false` hides the right-hand table of contents,
  `bookHidden: true` hides a page from the sidebar, `bookCollapseSection: true`
  collapses a section, `bookFlatSection: true` renders a section's children at
  the top level.
- Shortcodes: the site's own `screenshot` (above), `screens` (screenshots side
  by side), and for the landing page `features` + `feature` (the highlights
  list, `icon=` names a file in `assets/icons/`). The theme's `tabs`, `details`, `steps`, `mermaid` and others work
  too — see the [theme demo](https://hugo-book-demo.netlify.app).

Describe what a user sees and does, not how it is implemented; the server's
behaviour is documented for contributors in `/docs/agents`.

## Updating the theme

```sh
make theme-update TAG=v14   # any tag from https://github.com/alex-shpak/hugo-book/tags
```

It pins `go.mod` to the commit that tag points to. The theme's tags are not
Go-module versions, so `@v13` does not resolve — and **never run
`hugo mod get -u`**: Go only sees the theme's ancient `v0.15.0` tag as a real
release and would "upgrade" to it. After updating, check the theme's changelog
for renamed params and look at the site with `make serve`.
