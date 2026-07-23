# Main Content View Layout — Uniform Guidance

Canonical rules for how a **main content view** (anything rendered into `PlayerLayout`'s
`<main>` via `RouterView`) is structured: its header, its scrolling body, its gutters and
its scrollbar. Follow this whenever you add or refactor a top-level route view so every
screen reads as one app.

The pattern originated in **Now Playing** (`QueueView`, `variant="full"`) and was
generalised into the reusable **`ContentScaffold`** during the library work. All main
content views — including Now Playing itself — now use the scaffold. New views should too.

**Reference implementations (copy these):**
- `webui/src/components/layout/ContentScaffold.vue` — the reusable header + body frame.
- `webui/src/views/LibraryView.vue` — scaffold + body switch + count summary.
- `webui/src/components/library/AlbumGrid.vue` — the self-scrolling grid body.
- `webui/src/components/library/AlbumListView.vue` — the list body + alphabet rail.
- `webui/src/views/RadioView.vue` — the minimal case (scaffold + one action + a grid).
- `webui/src/components/layout/QueueView.vue` — Now Playing; scaffold for the full
  variant, compact side-panel header for the sidebar variant, body in `QueueBody.vue`.
- `webui/src/components/layout/EditActionBar.vue` — the uniform edit affordance for
  editable detail views; see [`unified-edit-experience.md`](unified-edit-experience.md).
- `webui/src/components/layout/HeroActions.vue` — the uniform read-mode play/queue/star
  affordance rendered in the hero; see [`unified-play-experience.md`](unified-play-experience.md).

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

- **Props:** `title: string`, `summary?: string`, `showBack?: boolean`.
- **Emits:** `@back` (when the back button is clicked).
- **Slots:** `#actions` (right-aligned header controls), `#title-actions` (inline actions beside the title), default (the body).
- **Structure & exact styling** (match these values if you ever hand-roll the header):
  - Header is `flex-shrink:0`, full-width with
    `padding-right: calc(var(--app-rail-clearance) + 2 * var(--sb-w, 0px))` (Recipe A: compensates for the body scroller's scrollbar twice) and a bottom border.
  - Inner `.scaffold-header-inner.content-col` centers on the shared content column with `0.75rem` block padding (`.content-col` supplies the inline gutter + max-width).
  - Optional back button (`showBack` prop): `Button` with `pi-arrow-left` icon, rendered before the title.
  - Title `<h1>`: `font-size: 1.5rem; font-weight: 700`.
  - Summary span: `font-size: 0.85rem; font-weight: 400; color: var(--app-text-secondary)`,
    sitting beside the title. **Rendered only when non-empty.**
  - `#actions` sits at the far right (`margin-left:auto` via the title's `flex:1`).
  - Body is `flex:1; min-height:0` with **no** left gutter — bodies own their full width and center content per the recipes (§4). This keeps the scrollbar and alphabet rail flush right.

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
frames it. Content alignment is driven by four CSS custom properties and three recipes that
handle scrollbar compensation at different depths.

**Tokens** (defined in `_variables.scss`, `--sb-w` set by `PlayerLayout` on `.content-area`):

| Token | Value | Meaning |
|---|---|---|
| `--app-content-max-width` | `1320px` | Inner content-box width of the column |
| `--app-content-gutter` | `1rem` | Horizontal padding inside the column |
| `--app-rail-clearance` | `2.75rem` | Alphabet-rail slot: rail 1.75rem + 1rem gap |
| `--sb-w` | measured px | Native scrollbar width |

**Global utility** (defined in `_main.scss`):

```css
.content-col {
    width: 100%;
    max-width: calc(var(--app-content-max-width) + 2 * var(--app-content-gutter));
    margin-inline: auto;
    padding-inline: var(--app-content-gutter);
    box-sizing: border-box;
}
```

**Three recipes** for centering body content. Elements at different depths need different
scrollbar compensation (the multipliers differ):

- **Recipe A — fixed (non-scrolling) frame** (scaffold header, fixed list headers, fixed heroes).
  The element spans the full pane (no scrollbar of its own), so it compensates for the body
  scroller's scrollbar *twice* (once for the scrollbar itself, once for the clearance the
  scroller's content adds on top):
  `padding-right: calc(var(--app-rail-clearance) + 2 * var(--sb-w, 0px))` on the frame, then
  a centered column inside (`.content-col` or manual centering).

- **Recipe B — plain scrolling body** (detail pages, playlists, search results, Now Playing).
  The scroll container's content box already excludes the scrollbar, so:
  `padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px))` on the scroll
  container (or its content wrapper), then `.content-col` blocks inside.

- **Recipe C — VirtualScroller body** (album/artist/genre/radio lists, card grids). Padding
  must go on PrimeVue's content element, and rows carry their own centering:
  `:deep(.p-virtualscroller-content) { box-sizing: border-box; padding-left: var(--app-content-gutter); padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px) + var(--app-content-gutter)); }`
  and each row `max-width: var(--app-content-max-width); margin-inline: auto;`.
  Fixed column-headers above such a scroller use Recipe A **plus** the gutter: frame
  `padding-left: var(--app-content-gutter); padding-right: calc(var(--app-rail-clearance) + 2 * var(--sb-w, 0px) + var(--app-content-gutter))`,
  inner row `max-width: var(--app-content-max-width); margin-inline: auto`.

**Alignment invariant:** the **inner edge** of `.content-col` (= column edge + gutter) is
where the title `h1`, every row *box*, every card, and every hero starts. Recipes A/B/C all
resolve to the same inner content box:
`min(1320px, pane − rail-clearance − 2·sb − 2·gutter)` centered left of the rail slot.

**Rules for any body:**
- Fill the frame with `height: 100%` and do the scrolling here (`overflow-y: auto`).
- Set `scrollbar-gutter: stable` so reserving the track causes no reflow when content
  changes, and the native scrollbar stays flush right. **Never restyle the scrollbar** —
  we keep the OS/browser default app-wide.
- Apply one of the three recipes to center content on the same shared column as the header.
  The width lives in one token, `--app-content-max-width` in `_variables.scss` — never
  hardcode `1320px`.
- Keep per-body loading / empty / error states inside the body. An empty state must not
  double as the error state: if the query can fail, render a distinct error branch (some
  current bodies still conflate the two — don't copy that).

---

## 5. List views: alphabet rail vs. scrollbar

For long alphabetically-indexed lists, the rail is a thin overlay **immediately left of the
native scrollbar** — the scrollbar remains the outermost element.

- `--sb-w` is now set once by `PlayerLayout` on `.content-area` — **views must not re-measure**;
  inherit the var (it's `0` on overlay-scrollbar systems).
- Set `scrollbar-gutter: stable` on the scroller.
- Rail: `position:absolute; top:0; bottom:0; right: var(--sb-w, 0px); width:1.75rem`. The
  `--app-rail-clearance` token (2.75rem) includes this rail width + 1rem gap; recipes
  reserve that clearance on the right so content never sits under the rail.
- Rail `@select(offset)` → `scrollToIndex(offset)` (for lazily-windowed lists, `ensureRange`
  before scrolling — see `AlbumListView`).

`AlphabetRail.vue` (`:letters`, `@select`) is reusable as-is.

---

## 6. Now Playing (`QueueView`) — the origin, and its two variants

`QueueView` originated the pattern and now composes `ContentScaffold` like every other
main content view. It splits per variant:

- `variant="full"` (Now Playing, route `/`): renders `ContentScaffold` with the queue
  actions (edit/save/clear, shared `QueueHeaderActions.vue`) in `#actions` and the
  scrolling queue in the default slot (`QueueBody.vue`). The scrolling content now sits on
  the **shared content column** (same as every other main view); the title aligns
  pixel-exact with the track rows.
- `variant="sidebar"` (queue panel): a compact hand-rolled header (`1rem/600` title, pill
  count badge, small buttons) — this variant is **not** governed by this guidance; it's
  side-panel chrome, and the scaffold's `h1`/wide paddings don't fit it.

**Guidance:** for a plain view (title + count + a few actions + a scrolling body), **import
`ContentScaffold`** — do not reimplement. Hand-rolled headers are reserved for non-view
chrome like the queue sidebar.

---

## 7. Conformance status

| View | Route | Conforms? |
| --- | --- | --- |
| Now Playing (`QueueView` full) | `/` | ✅ `ContentScaffold` (origin of the pattern) |
| Library (album/artist × list/grid) | `/library` | ✅ `ContentScaffold` |
| Search | `/search` | ✅ `ContentScaffold` |
| Radio | `/radio` | ✅ `ContentScaffold` |
| Album detail (`AlbumView`) | `/album/:id` | ✅ `ContentScaffold` (cover in body hero) |
| Artist detail (`ArtistView`) | `/artist/:id` | ✅ `ContentScaffold` |
| Playlists (`PlaylistsView`) | `/playlists` | ✅ `ContentScaffold` |
| Playlist detail (`PlaylistDetailView`) | `/playlist/:id` | ✅ `ContentScaffold` |
| Genres (`GenresView`) | `/genres` | ✅ `ContentScaffold` (stub body) |
| Podcasts (`PodcastsView`) | `/podcasts` | ✅ `ContentScaffold` |
| Podcast channel (`PodcastChannelView`) | `/podcast/:id` | ✅ `ContentScaffold` (cover in body hero) |
| Settings sub-views | `/settings/*` | ❌ separate `SettingsLayout`, own headers |

Settings sub-views intentionally use `SettingsLayout`, not this pattern. All other main
content views now conform; keep new ones on `ContentScaffold`.

---

## 8. Checklist for a new main content view

1. Wrap the page in `ContentScaffold` with a `title` and (if it has a countable body) a
   `summary` computed that pluralises and returns `''` at zero.
2. Put header controls in `#actions`.
3. Make the body a self-scrolling child: `height:100%; overflow-y:auto;
   scrollbar-gutter:stable`, content centered via `.content-col` / the recipes (§4). Reserve
   the uniform rail clearance on the right even without a rail, so the content column sits
   at the same x on every view.
4. Add `meta: { flush: true }` to the route in `router/index.ts`.
5. Long indexed list? Reuse `AlphabetRail` per §5 (never re-measure `--sb-w`; it's already
   set by `PlayerLayout`).
6. Test (Vitest): title renders, summary reflects the count (singular + plural) and is
   absent at zero, actions live in the header, empty/loading states show. See
   `RadioView.spec.ts` / `LibraryView.spec.ts` for the shape.
