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
   Play/Queue/Favorite live in `HeroActions` inside `HeroHeader`, never in the
   scaffold action bar; hidden in edit mode. Also pins the one favorite
   affordance used app-wide: `pi pi-heart(-fill)` with "Add to/Remove from
   favorites".

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
  `usePlayer.ts` is module-scoped singleton state, as is
  `useIdentifyCache.ts` — the metadata editor's identify answers are cached
  per (library, path) and per selection set, LRU-capped, so reopening an
  identify dialog costs no fingerprint pass. It is deliberately not a
  vue-query cache: identification is a POST over a path list with no stable
  query key, and reuse is per path, not per request. `useIdentifyRuns.ts`
  owns both identify flows over it (dialog state, abort controllers,
  cache reads); the way past a cached answer is the dialogs' Re-identify
  action or the editor's Reload, which clears the cache. This is the outer of
  two cache layers — the server shares the per-file fingerprint pass between
  both identify flows (`identify.Cache`, see
  [architecture.md](architecture.md)), so a Re-identify that misses here can
  still be answered from the server's cache without re-fingerprinting.
  `useReleaseGroupGenres.ts` is module-scoped for the same reason: neither
  identify response carries genres (MusicBrainz keeps genre votes on the
  release **group**, and the resolver only fetches the release's tracklist),
  so both dialogs look them up for the group the user settled on and stage
  them through the `genres` field checkbox. It is an LRU keyed by
  release-group MBID **with in-flight dedupe** — MusicBrainz allows 1 req/sec,
  and identifying a folder song by song means every row usually lands on the
  same album, which must cost one request rather than one per file. It never
  rejects: a failed lookup resolves to `[]` so the apply still stages the rest
  of the match, and a failure is not cached (a rate-limited lookup stays
  retryable) while an empty answer is. An empty list stages **nothing** —
  staging `[]` would wipe genres the file already carries.
  After any metadata-editor write, call `invalidateAfterMetadataWrite(qc)`
  from `useMetadataEditor.ts` rather than inlining keys. It drops
  `['metadata','tracks']` and `['metadata','raw']` (the editor's own views)
  plus the entire `['subsonic']` tree (the music UI). The last one is blunt on
  purpose: album/artist/genre ids are not stable across a tag edit (album
  identity is `(name_norm, album_artist_norm, mb_release_id)`, so renaming an
  album creates a new row), and the editor works in file paths, so there is no
  precise key set to target. The server re-indexes synchronously, so a
  resolved write already means the DB is current; no polling needed.
- `lib/api/` — HTTP clients: `client.ts` (axios, `/api/v1`, overridable via
  `VITE_SERVER_URL_V1`) and `subsonic.ts` (`SubsonicClient`; same-origin
  default via `initWithDefaults()`, no auth today — see
  [subsonic-api.md](subsonic-api.md)).
- `types/` — one file per API domain, mirroring backend response shapes.
- `lib/apiError.ts` — **the only place a thrown API error becomes text.** Use
  `apiErrorMessage(err, fallback)` instead of re-deriving
  `err?.response?.data?.error ?? err.message`: it reads the server's
  `{error, code}` envelope, unwraps a nested JSON body rather than showing it
  raw, and describes a no-response failure as a connectivity problem.
  `isRateLimitError(err)` spots 429 / `upstream_rate_limited` so a view can
  invite a retry (warn) rather than report a hard failure (error) — see the
  search tab of `PicturePickerDialog`. Third-party lookups (cover art,
  MusicBrainz, radio-browser) fail routinely; the composables expose
  `{ error, rateLimited }` and the server already sends a showable sentence.
- `store/uiStore.ts` — Pinia, UI-only state. Player persistence is
  localStorage (`musicPlayer:*` keys) via `utils/localStorage.ts`, not Pinia.

## Player (`composables/usePlayer.ts`)

Dual `<audio>` elements: active plays, standby pre-buffers the next track
(`preload="auto"`); on `ended` the roles swap. This removes the audible
inter-track gap without Web Audio. **Sample-accurate gapless was assessed and
deferred** — `docs/gapless-playback-web-audio.md` records the Web-Audio
design; don't re-derive it. Queue/volume/shuffle/repeat persist to
localStorage.

Mute is derived, not a flag: `isMuted` is `volume === 0`, so silence reached by
dragging the rail down and silence reached by clicking the speaker are the same
state. `toggleMute` restores `unmutedVolume` — the last non-zero volume, recorded
by the volume watcher and persisted under `musicPlayer:unmutedVolume` so a
session left muted still knows where to return (falling back to full volume if it
is itself 0). That watcher flushes **sync**: a mute in the same tick as a volume
change has to see the new level, and the audible volume should follow the rail
without waiting for a tick.

In `PlayerControls.vue`, `useRailDrag` owns both rails' pointer handling *and*
their interaction state, exposed as `active` (hovered or dragging) and rendered
as a `rail-active` class on the wrapper. An inactive bar carries no colour: the
knob is faded out and the fill uses `--app-player-range` instead of
`--app-accent`. Hover alone can't drive this in CSS — dragging past the bar's
edge fires `mouseleave` while the grab is still on, and the rail must not go
neutral mid-drag.

The knob is faded with **opacity only** — `visibility: hidden`/`display: none`
would drop the handle out of the tab order, and it is the slider's focusable
element (`tabindex=0`, `role=slider`), so that would make volume and seek
unreachable by keyboard; focus lights both the knob and the fill for the same
reason. `--app-player-range` must be defined by every palette that repaints
`--app-player-track` (both hidden themes use their dim variant, so the bright
accent stays the hover state).

The speaker has three states — loud / quiet / silent. PrimeIcons ships no
slashed-speaker glyph (`pi-volume-off` is a bare cone that reads as "quiet"
beside `pi-volume-down`), so silence is that glyph plus a `muted` class whose
`::after` draws the strike in `currentColor`. Every dimension of it is in `em`,
including the knockout ring: a px ring tuned at 40px swallows the cone at the
bar's actual 1rem.

Two style specs guard this, because scoped SFC styles never apply under
vue-test-utils — no mounted test can catch a colour regression:
`PlayerControls.railStyles.spec.ts` parses the SFC's style block, and
`assets/scss/__tests__/player-tokens.spec.ts` compiles the palettes and checks
the token is present in each.

## Styling

SCSS under `assets/scss/`; shared tokens in `_variables.scss` — notably
`--app-content-max-width` (never hardcode 1400px). Native scrollbars are kept
app-wide (never restyle). PrimeVue theme via `@primeuix/themes` (`theme.js`);
Inter variable font. One deliberate global dialog-footer rule in `_main.scss`
(confirm-first ordering — see registry 2 before touching dialog footers).

**Hidden themes (easter egg).** `_hidden-themes.scss` holds two unlockable
palettes — Winamp, CRT. They are token-only repaints layered over
`.dark-mode` (`useTheme` keeps that class on for them so the PrimeVue preset
stays dark) and are selected by a `theme-<name>` root class. Five clicks on the
wordmark's "e" in `AppSidebar` unlock them for good (`aether:hiddenThemes` in
localStorage) and cycle between them; once unlocked they appear in the
Settings → Profile theme picker. This is intentional — don't "clean it up".

## Testing (see [testing.md](testing.md))

`npm test` = `vue-tsc --noEmit` + Vitest (jsdom). Specs live in `__tests__/`
next to the code. Convention from the layout registry: a new main content
view tests title, pluralized summary (absent at zero), header actions, and
empty/loading states — copy `RadioView.spec.ts` / `LibraryView.spec.ts`.
Count summaries are built as single strings so `.text()` assertions stay
reliable.
