# Main Content View Layout — Uniform Guidance

Canonical rules for how a **main content view** (anything rendered into `PlayerLayout`'s
`<main>` via `RouterView`) is structured: its header, its scrolling body, its gutters and
its scrollbar. Follow this whenever you add or refactor a top-level route view so every
screen reads as one app.

The pattern originated in **Now Playing** (`QueueView`, `variant="full"`) and was
generalised into the reusable **`ContentScaffold`** during the library work. The library
album/artist views and the radio view now follow it. New views should too.

**Reference implementations (copy these):**
- `webui/src/components/layout/ContentScaffold.vue` — the reusable header + body frame.
- `webui/src/views/LibraryView.vue` — scaffold + body switch + count summary.
- `webui/src/components/library/AlbumGrid.vue` — the self-scrolling grid body.
- `webui/src/components/library/AlbumListView.vue` — the list body + alphabet rail.
- `webui/src/views/RadioView.vue` — the minimal case (scaffold + one action + a grid).
- `webui/src/components/layout/QueueView.vue` — Now Playing; mirrors the header manually.

**Source specs/plans (the "why"):**
- `docs/superpowers/specs/2026-07-02-library-scaffold-and-artist-list-design.md`
- `docs/superpowers/specs/2026-06-19-merge-queue-nowplaying-design.md`
- `docs/superpowers/specs/2026-07-01-albums-table-list-view-design.md`

---

## 1. The layout shell (`PlayerLayout.vue`)

Every route view is mounted inside a fixed-height flex column:

```
app-container (100vh, overflow hidden)
 └ body-row (flex, min-height:0)
    └ content-area (flex, overflow hidden)
       ├ main.main-content  ← RouterView renders here
       └ QueueSidebar
```

- `.main-content` is `flex:1; overflow-y:auto; padding: 1rem 2rem` **by default**. Simple
  padded pages (settings, detail pages) rely on this default and scroll as one block.
- **Full-bleed routes** add the `main-content--flush` modifier, which drops the horizontal
  padding so the *view itself* owns its gutters and its scrollbar reaches the
  content-area's right edge. Flush is declared **per route via `meta.flush`** (co-located
  with the route, matching the existing `meta.layout` convention read in `App.vue`), and
  `PlayerLayout` reads it:

  ```ts
  // router/index.ts — on each full-bleed route:
  { path: '/radio', name: 'radio', component: …, meta: { flush: true } }

  // PlayerLayout.vue:
  :class="{ 'main-content--flush': route.meta.flush }"
  ```
  The `flush?: boolean` field is declared on `RouteMeta` in `App.vue`.

**When to make a route flush:** set `meta: { flush: true }` whenever the view uses
`ContentScaffold` (or otherwise manages its own gutter + self-scrolling body). Header-only
reuse without a self-scrolling body does **not** need flush. If you forget, the view gets a
double gutter and the scrollbar floats inside the padding instead of hugging the edge.

---

## 2. The content header (`ContentScaffold`)

`ContentScaffold` (`components/layout/ContentScaffold.vue`) is the **canonical content
header** — a general-purpose frame, not tied to any domain. Reuse it directly on any main
content view (library and radio do). Do not fork a second header component; extend this one
if a view needs more.

```vue
<ContentScaffold :title="title" :summary="summary">
    <template #actions> …toggles / buttons… </template>
    …body…            <!-- default slot -->
</ContentScaffold>
```

- **Props:** `title: string`, `summary?: string`.
- **Slots:** `#actions` (right-aligned header controls), default (the body).
- **Structure & exact styling** (match these values if you ever hand-roll the header):
  - Header is `flex-shrink:0`, `align-items: baseline`, centered on the shared content
    column (`max-width: var(--app-content-max-width); margin-inline:auto`),
    `padding: 0.75rem 2rem`. The `2rem` is the side gutter that indents the title over the
    body content.
  - Title `<h1>`: `font-size: 1.5rem; font-weight: 700`.
  - Summary span: `font-size: 0.85rem; font-weight: 400; color: var(--app-text-secondary)`,
    sitting beside the title. **Rendered only when non-empty.**
  - `#actions` sits at the far right (`margin-left:auto` via the title's `flex:1`).
  - Body is `flex:1; min-height:0; padding-left: 2rem` — matches the header's left gutter so
    rows line up under the title, while the right edge stays unpadded so the body's
    scrollbar sits flush right.

The scaffold does **not** impose `overflow` — the body slot owns its own scroll (see §4).

---

## 3. The count summary convention

The summary is a short secondary-text count shown beside the title. Build it as a single
computed string, pluralise the unit, and return **`''` (empty) when the count is zero** so
the scaffold omits the element entirely.

```ts
const summary = computed(() => {
    const n = items.value?.length ?? 0
    if (n === 0) return ''
    return `${n} ${n === 1 ? 'station' : 'stations'}`
})
```

- Library: `"1240 albums"` / `"87 artists"`; Radio: `"5 stations"`; Now Playing:
  `"27 tracks • 1 hr 34 min"` (an extra `• duration` segment, omitted when duration is 0).
- Pre-build multi-part summaries as one string (no stray whitespace between number and
  unit) so `.text()` assertions stay reliable.
- For large/windowed lists, source the count from the **index total**, not the number of
  rows currently rendered (the album grid shows 50 but reports the full library total).

---

## 4. The scrolling body

The body is a child that **scrolls itself** and centers its content — the scaffold only
frames it. Two shapes:

**Grid body** (see `AlbumGrid.vue`):

```css
.grid-scroll { height: 100%; overflow-y: auto; scrollbar-gutter: stable; }
.album-grid  { max-width: var(--app-content-max-width); margin: 0 auto; padding: 1rem;
               display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 2rem; }
```

**List body** (see `AlbumListView.vue`): a `VirtualScroller` at `height:100%` inside a
`position:relative` container, content centered at a max-width with gutters.

Rules for any body:
- Fill the frame with `height: 100%` and do the scrolling here (`overflow-y: auto`).
- Set `scrollbar-gutter: stable` so reserving the track causes no reflow when content
  changes, and the native scrollbar stays flush right. **Never restyle the scrollbar** —
  we keep the OS/browser default app-wide.
- Center content on the same shared column as the header
  (`max-width: var(--app-content-max-width); margin:0 auto`) with its own inner `padding`
  (`1rem` for grids). The width lives in one token, `--app-content-max-width` in
  `_variables.scss` — never hardcode `1400px`.
- Keep per-body loading / empty / error states inside the body. An empty state must not
  double as the error state: if the query can fail, render a distinct error branch (some
  current bodies still conflate the two — don't copy that).

---

## 5. List views: alphabet rail vs. scrollbar

For long alphabetically-indexed lists, the rail is a thin overlay **immediately left of the
native scrollbar** — the scrollbar remains the outermost element. No magic offsets:

- Measure the OS scrollbar width once with the reusable **`useScrollbarWidth`** composable
  (`webui/src/composables/useScrollbarWidth.ts`) — returns `0` on overlay-scrollbar systems.
- Set `scrollbar-gutter: stable` on the scroller and bind the measured width to a CSS var
  on the root: `:style="{ '--sb-w': scrollbarWidth + 'px' }"`.
- Rail: `position:absolute; top:0; bottom:0; right: var(--sb-w); width:<rail-w>`. Rows get
  `padding-right: calc(<rail-w> + var(--sb-w))` so content never sits under the rail.
- Rail `@select(offset)` → `scrollToIndex(offset)` (for lazily-windowed lists, `ensureRange`
  before scrolling — see `AlbumListView`).

`AlphabetRail.vue` (`:letters`, `@select`) is reusable as-is.

---

## 6. Now Playing (`QueueView`) — the origin, and why it's special

`QueueView` predates `ContentScaffold` and **mirrors the header manually** rather than
importing it, because it carries edit-mode chrome, drag-drop, and a `variant` prop that the
scaffold does not model. Keep it visually in lock-step with the scaffold:

- `variant="full"` (Now Playing, route `/`): header is `.queue-view--full .queue-view-header`
  with the **same** values as the scaffold — shared content column
  (`var(--app-content-max-width)`), `padding: 0.75rem 2rem`, title `1.5rem/700`, summary
  `0.85rem` secondary. The scrolling *content* below it sits on a narrower **1100px**
  centered column, so the title deliberately sits slightly left of it.
- `variant="sidebar"` (queue panel): a compact header (`1rem/600` title, smaller paddings)
  — this variant is **not** governed by this guidance; it's side-panel chrome.

**Guidance:** for a plain view (title + count + a few actions + a scrolling body), **import
`ContentScaffold`** — do not reimplement. Only hand-roll the header (matching §2's values)
when the view needs structure the scaffold can't express, as `QueueView` does. This
duplication is tolerated at one site; if a *third* hand-rolled header appears, extract the
shared header row into a small presentational component both compose.

---

## 7. Conformance status

| View | Route | Conforms? |
| --- | --- | --- |
| Now Playing (`QueueView` full) | `/` | ✅ origin of the pattern (manual header) |
| Library (album/artist × list/grid) | `/library` | ✅ `ContentScaffold` |
| Radio | `/radio` | ✅ `ContentScaffold` |
| Album detail (`AlbumView`) | `/album/:id` | ❌ predates it — `max-width:1200px`, own header, not flush |
| Artist detail (`ArtistView`) | `/artist/:id` | ✅ `ContentScaffold` |
| Playlists / Genres / Podcasts / Settings | various | ❌ default padded `main-content`, own headers |

The ❌ views are not broken — they use the default padded shell. Migrate them to this
pattern opportunistically when you're already working in them; don't do a blanket sweep.

---

## 8. Checklist for a new main content view

1. Wrap the page in `ContentScaffold` with a `title` and (if it has a countable body) a
   `summary` computed that pluralises and returns `''` at zero.
2. Put header controls in `#actions`.
3. Make the body a self-scrolling child: `height:100%; overflow-y:auto;
   scrollbar-gutter:stable`, content centered at
   `max-width: var(--app-content-max-width); margin:0 auto` with its own padding.
4. Add `meta: { flush: true }` to the route in `router/index.ts`.
5. Long indexed list? Use `useScrollbarWidth` + `AlphabetRail` per §5.
6. Test (Vitest): title renders, summary reflects the count (singular + plural) and is
   absent at zero, actions live in the header, empty/loading states show. See
   `RadioView.spec.ts` / `LibraryView.spec.ts` for the shape.
