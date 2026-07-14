# Dense Pro — theme port notes

Reference mockup: [`dense-pro.html`](./dense-pro.html) (open in a browser; tab bar
switches Now Playing / Library / Album / Artist / Song; top-right toggle switches
light/dark). This file lists the changes needed to bring that look into the real
Vue app. Nothing here is implemented yet — it's the working spec for the next
iteration.

The mockup is a **static, self-contained** approximation: its class names and
token names are its own, **not** the app's. Treat the values below as the source
of truth; map them onto the real components.

## The idea in one line

Compact, information-dense "pro/audiophile" layout — tight rows, small radii, a
**dark slate navigation + player** framing a light content canvas, with a
**cyan** accent (indigo → cyan). Contrast in both themes comes from layered
surfaces, not a single flat tone.

---

## 1. Color tokens — `src/assets/scss/_variables.scss`

Current tokens and their new Dense Pro values. **Keep the token names**; just
change values (and add the few new ones noted below).

| Token | Light (now → new) | Dark (now → new) |
|---|---|---|
| `--app-background` | `#f8f9fa` → **`#e7eaee`** | `#10141b` → **`#0d1117`** |
| `--app-surface` | `#ffffff` → **`#f4f6f8`** | `#1a202c` → **`#141b24`** |
| `--app-border` | `#e5e7eb` → **`#dde2e8`** | `#333a47` → **`#232d3a`** |
| `--app-text-primary` | `#1f2937` → **`#1b2430`** | `#e2e8f0` → **`#d7dee6`** |
| `--app-text-secondary` | `#6b7280` → **`#67727f`** | `#94a3b8` → **`#7b8794`** |
| `--app-accent` | `#6366f1` → **`#0e9bb5`** | `#818cf8` → **`#2fd3ef`** |
| `--app-accent-hover` | `#4f46e5` → **`#0b8299`** | `#a5b4fc` → **`#57e0f7`** |
| `--app-accent-soft` | `#eef2ff` → **`rgba(14,155,181,.12)`** | `#272c35` → **`rgba(47,211,239,.16)`** |
| `--app-accent-soft-hover` | `#e0e7ff` → **`rgba(14,155,181,.2)`** | `#333a47` → **`rgba(47,211,239,.24)`** |
| `--app-hover` | `rgba(0,0,0,.04)` → **`rgba(20,120,150,.07)`** | `rgba(255,255,255,.06)` → **`rgba(47,211,239,.09)`** |
| `--app-player-bg` | `#1a2942` → **`#141b24`** | (same) → **`#080b10`** |
| `--app-player-text` | `#e2e8f0` → **`#d3dae2`** | (same) → **`#d7dee6`** |

**New tokens to add** (the mockup uses layers the current theme doesn't have):

- `--app-surface-2` — elevated cards / raised panels. Light **`#ffffff`**, dark **`#1a222e`**.
  Today the app only has background + surface; Dense Pro is a 3-layer system
  (canvas `background` → panel `surface` → card `surface-2`).
- `--app-nav-bg` — **dark slate in both themes** (see §3). Light **`#1b2430`**, dark **`#0a0e13`**.
- `--app-nav-text` — light **`#d3dae2`**, dark **`#d7dee6`**.
- `--app-nav-text-dim` — light **`#7b8794`**, dark **`#6a7681`**.
- `--app-nav-brand` — logo/brand accent, **`#2fd3ef`** both themes.
- `--app-player-dim` — muted controls in the player bar. Light **`#6f7b88`**, dark **`#6a7681`**.
- `--app-player-track` — scrubber/volume rail. **`rgba(255,255,255,.18)`** (light) / `.14` (dark).

Accent-on-white legibility: `#0e9bb5` is fine for icons/large text and button
fills (white text). For small cyan text on white, prefer `--app-accent-hover`
(`#0b8299`).

## 2. Dimensions — `_variables.scss`

- `--app-content-max-width` `1400px` → **`1320px`** (denser column).
- `--app-player-height` `64px` → **`116px`** (taller bar so the controls breathe).
- Add a small radius scale token if convenient, e.g. `--app-radius: 8px` (cards/
  inputs) — Dense Pro uses **6–10px**, tighter than today.
- Sidebar widths can stay (`280` / `70`), or narrow the expanded rail toward
  `240px` to match the mock's tighter feel.

---

## 3. Component changes

### `src/components/layout/AppSidebar.vue` — biggest visual change
- Sidebar background becomes **dark slate in BOTH themes** (`--app-nav-bg`),
  not `--app-surface`. This dark rail against the light content canvas is the
  main source of "contrasting areas" in light mode — the whole point of the look.
- Text uses `--app-nav-text` / `--app-nav-text-dim`; brand/logo uses
  `--app-nav-brand` (cyan).
- **Active/selected items are full-width bars, not rounded pills.** Remove the
  nav list's horizontal padding and set the item `border-radius: 0` so the item
  (its hover background and active background) spans the entire sidebar width,
  edge to edge. The item keeps only its internal left padding for the icon/label.
- Active item: cyan text on `--app-accent-soft` full-width background, with the
  accent highlight as a **right-edge bar** (`box-shadow: inset -3px 0 0 var(--app-accent)`),
  sitting flush against the sidebar's right border (the content-facing edge). The
  current app already uses `inset -3px 0 0`; the change is dropping the pill radius
  and side gutter so it reaches full width. Apply the same to the collapsed rail.
- Tighten item padding/font for density (~`0.55rem 1rem`, `0.85rem` font).

### `src/components/layout/PlayerControls.vue`
- Height `116px` with **vertical padding (~24px)** so the play/pause + scrubber
  cluster sits with clear breathing room from the top and bottom edges (the bar
  was previously cramped with zero vertical padding). Background `--app-player-bg`
  (`#141b24` / `#080b10`).
- Give the center column (transport row + scrubber row) a slightly larger vertical
  gap (~11px) so the two rows aren't cramped together.
- Play button: solid cyan (`--app-accent`) circle, white glyph.
- Muted transport/volume icons use `--app-player-dim`; rails use
  `--app-player-track`. Time labels tabular-nums.

### Track lists (`src/components/library/AlbumTrackRow.vue` + album/artist track headers)
- **Compact rows ~40px** tall.
- Grid columns tighten to roughly `38px  minmax(0,2.4fr)  minmax(0,1.4fr)  34px  62px`
  (index · title · artist · star · duration).
- Header row shows a small **clock icon** for the duration column; durations use
  `font-variant-numeric: tabular-nums`.
- Current/playing row: `--app-accent-soft` background, cyan title + cyan index
  (volume icon in place of the number). Queue current row gets an inset cyan bar.

### Library card grid (`src/components/library/VirtualCardGrid.vue` + card component)
- Smaller cards: min width **~142–160px**, gap **12px**, radius **8px**, padding **8px**.
- Card = `--app-surface-2` with a hairline `--app-border` and a very subtle
  shadow (`0 1px 3px rgba(0,0,0,.06)`), lifting to `0 6px 16px rgba(0,0,0,.12)`
  on hover. Cover art radius ~5px.

### `ContentScaffold` header + `AlbumView.vue` hero
- Smaller title (`~22px`), tighter header spacing; summary line in
  `--app-text-secondary`.
- Album/artist/song hero can stay on `--app-surface-2` (Dense Pro keeps heroes
  flat, not gradient — that was the separate "mix" experiment we dropped).

### Queue (`src/components/layout/QueueSidebar.vue`, `QueueRow.vue`)
- Compact rows. Match the sidebar treatment: **full-width rows** (remove the
  queue list's horizontal padding and set row `border-radius: 0`) so the current
  item's background spans edge to edge.
- Current item: `--app-accent-soft` full-width background with the accent
  highlight as a **left-edge bar** (`box-shadow: inset 3px 0 0 var(--app-accent)`) —
  the left is the queue's content-facing edge (mirror of the nav, whose bar is on
  its right/content-facing edge).
- Count badge uses accent-soft + cyan text.
- Queue panel divider (`border-left`) uses the **soft neutral `--app-border`**, a
  low-contrast line — not any dark nav-edge color. Keep it subtle.

---

## 4. Open questions to resolve while iterating
- **Dark sidebar in light theme** — confirm we want the sidebar (and player)
  permanently dark. It's the defining contrast of this look, but it's a real
  departure from the current all-light chrome.
- **Third surface layer** — confirm adding `--app-surface-2` vs. reusing
  `--app-surface` as the card color and darkening the canvas.
- **Accent** — cyan `#0e9bb5`/`#2fd3ef` is the proposed accent; confirm before we
  touch every accent usage across the app.
- **Density everywhere vs. just lists** — decide whether the compact spacing
  applies globally or mainly to tracklists/queue/cards.
