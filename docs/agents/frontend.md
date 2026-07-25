# Frontend — structure, conventions, the three registries

`webui/` is a Vue 3 + TypeScript SPA (Vite, PrimeVue 4, Pinia, TanStack
vue-query, vue-router). Built output is copied into `app/spa/files/ui` and
embedded in the Go binary (`make package-ui`); in the binary the SPA is the
catch-all route behind `/api/v1` and `/rest`.

## The three convention registries (read before UI work)

These live in `docs/architecture/` and are **maintained, current documents**
— unlike the historical specs in `docs/superpowers/`. CLAUDE.md makes the
first one mandatory:

1. **`docs/architecture/main-content-view-layout.md`** — every top-level
   route view rendered into `PlayerLayout` uses the `ContentScaffold` header
   (title + count summary + `#actions`), a self-scrolling body centered on
   `--app-content-max-width`, and `meta: { flush: true }` on its route. Has a
   conformance table and a new-view checklist. `QueueView` is the one
   tolerated hand-rolled header — do not create a third.
2. **`docs/architecture/unified-edit-experience.md`** — editable detail views
   use `EditActionBar` (pencil → Delete/Save/Cancel, confirm-first dialog
   ordering via one global SCSS rule, Esc semantics, `dirty` prop).
3. **`docs/architecture/unified-play-experience.md`** — read-mode
   Play/Queue/Star live in `HeroActions` inside `HeroHeader`, never in the
   scaffold action bar; hidden in edit mode.

When a view diverges from these registries, the registry wins.

## Directory roles (`webui/src`)

- `views/` — route components; `views/settings/` for the settings shell
  (`meta: { layout: 'settings' }`, separate `SettingsLayout`). Routes in
  `router/index.ts`; the two route categories are tabled in CLAUDE.md.
- `components/layout/` — app chrome (sidebar, player controls, queue,
  ContentScaffold, EditActionBar, HeroHeader/HeroActions).
- `components/library/` — domain cards/grids/rows. **All card grids render
  through the shared `VirtualCardGrid`** (AlbumGrid, ArtistGrid,
  RadioStationGrid…) — don't fork a new grid.
- `composables/` — all non-trivial logic. Server state goes through
  TanStack query composables (`useSubsonicQueries.ts` with a central
  `queryKeys` map — add new keys there, don't inline key arrays).
  `usePlayer.ts` is module-scoped singleton state.
- `lib/api/` — HTTP clients: `client.ts` (axios, `/api/v1`, overridable via
  `VITE_SERVER_URL_V1`) and `subsonic.ts` (`SubsonicClient`; same-origin
  default via `initWithDefaults()`, no auth today — see
  [subsonic-api.md](subsonic-api.md)).
- `types/` — one file per API domain, mirroring backend response shapes.
- `store/uiStore.ts` — Pinia, UI-only state. Player persistence is
  localStorage (`musicPlayer:*` keys) via `utils/localStorage.ts`, not Pinia.

## Player (`composables/usePlayer.ts`)

Dual `<audio>` elements: active plays, standby pre-buffers the next track
(`preload="auto"`); on `ended` the roles swap. This removes the audible
inter-track gap without Web Audio. **Sample-accurate gapless was assessed and
deferred** — `docs/gapless-playback-web-audio.md` records the Web-Audio
design; don't re-derive it. Queue/volume/shuffle/repeat persist to
localStorage.

## Styling

SCSS under `assets/scss/`; shared tokens in `_variables.scss` — notably
`--app-content-max-width` (never hardcode 1400px). Native scrollbars are kept
app-wide (never restyle). PrimeVue theme via `@primeuix/themes` (`theme.js`);
Inter variable font. One deliberate global dialog-footer rule in `_main.scss`
(confirm-first ordering — see registry 2 before touching dialog footers).

## Testing (see [testing.md](testing.md))

`npm test` = `vue-tsc --noEmit` + Vitest (jsdom). Specs live in `__tests__/`
next to the code. Convention from the layout registry: a new main content
view tests title, pluralized summary (absent at zero), header actions, and
empty/loading states — copy `RadioView.spec.ts` / `LibraryView.spec.ts`.
Count summaries are built as single strings so `.text()` assertions stay
reliable.
