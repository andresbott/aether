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
   favorites", and the track-row double-click contract (rows emit `enqueue`,
   which appends to the end of the queue — only the hero Play replaces it).

When a view diverges from these registries, the registry wins.

## Directory roles (`webui/src`)

- `views/` — route components; `views/settings/` for the settings shell
  (`meta: { layout: 'settings' }`, separate `SettingsLayout`). Routes in
  `router/index.ts`; the two route categories are tabled in CLAUDE.md.
- `components/layout/` — app chrome (sidebar, player controls, queue,
  ContentScaffold, EditActionBar, HeroHeader/HeroActions).
- `components/library/` — domain cards/grids/rows. **All card grids render
  through the shared `VirtualCardGrid`** (AlbumGrid, ArtistGrid,
  RadioStationGrid…) — don't fork a new grid. **`TrackActionSheet`** is the
  touch counterpart of row hover affordances (add to queue, favorite, add to
  playlist, go to album/artist), opened via the per-row ⋮ on
  `useViewport().isTouch`.
  The four library bodies (`AlbumGrid`, `AlbumListView`, `ArtistGrid`,
  `ArtistListView`) take their data from **`useAlbumSource`/`useArtistSource`**
  (`composables/useLibrarySource.ts`), not from `useAlbumTable`/`useArtistTable`
  directly: those pick between the full library and the favorites subset
  (`getStarred2`) behind one shared shape, so the components render either without
  knowing which they got and the `favoritesOnly` prop is their only concession to
  it. Both sources are instantiated and gated on `enabled` — composables can't be
  called conditionally, and the source nobody is viewing must not fetch. Add a new
  source (a genre filter, say) there rather than branching inside a body.
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
  default; `initWithDefaults()` is the **auth-method-`none`** path only —
  authenticated modes call `setApiKey()` with the PAT minted by
  `lib/subsonicSession.ts` — see [subsonic-api.md](subsonic-api.md) and
  [authentication.md](authentication.md)).
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

### Shells

`PlayerLayout` (`layouts/PlayerLayout.vue`) owns the shared skeleton —
`AppSidebar`, the `main`/`RouterView` outlet, `QueueSidebar` — and swaps only
the chrome components `DesktopShell` and `MobileShell` (both in `layouts/`)
via a `v-if` on `useViewport().shell`. **The route outlet must stay outside
the swap**: unmounting the active view on rotation would bypass its
unsaved-edit guards (`onBeforeRouteLeave` / `beforeunload` are written for
navigation, not teardown) and silently discard staged edits; guarded by
`PlayerLayout.shellSwitch.spec.ts`. The breakpoint decision is: desktop width
(≥1024px) → desktop, phone width (<768px) → mobile, tablet band → landscape
**and at least 600px tall** → desktop, otherwise mobile. The height gate
exists because landscape *phones* land in the tablet width band (iPhone 15:
852×393) and must stay on the mobile shell. The constants live in
`lib/breakpoints.ts` (`BP_PHONE_MAX = 768`, `BP_DESKTOP_MIN = 1024`,
`BP_SHELL_MIN_HEIGHT = 600`); the widths are mirrored in SCSS
(`_variables.scss`: `$bp-phone-max`, `$bp-desktop-min`); agreement is guarded
by `assets/scss/__tests__/breakpoints.spec.ts`. Media queries can't read CSS
custom properties, hence the SCSS twins.

`useViewport` (singleton composable) reports `shell` ('desktop' | 'mobile'),
`tier` ('phone' | 'tablet' | 'desktop'), and `isTouch` (from `(pointer:
coarse)`).

**Never size anything in `vh`; the app-shell height chain owns the viewport.**
`_main.scss` makes `<html>` the one viewport-sized box (`height: 100dvh`,
`overflow: hidden`, `overscroll-behavior: none`) and `body` / `#app` / both
player shells are `height: 100%` of it. `100vh` on a mobile browser is the
URL-bar-HIDDEN (large) viewport: the document outgrows the screen, the page
itself scrolls, and the `ContentScaffold` header (hamburger included) slides
under the URL bar while the shell-less tail of `#app` shows as dead space above
the system nav. Use `dvh` for the rare box that must measure the viewport
itself (`SettingsLayout`, `LoginView`, the play view's artwork cap). **Scrollbar
chrome is gated on `(pointer: fine)`** in the same file, with `(pointer:
coarse)` hiding bars outright: styling `::-webkit-scrollbar` at all opts mobile
Chrome out of its auto-hiding overlay scrollbars, turning them into permanently
visible classic bars that also claim layout width — which is why `--sb-w`
(`useScrollbarWidth`) measures 0 on touch and the recipes reserve no clearance
there. Both invariants are pinned off disk by
`assets/scss/__tests__/appShell.spec.ts`, because neither regression reproduces
under DevTools device emulation — emulation has no retractable URL bar (`vh ==
dvh`) and desktop-style scrollbars.

**Never `scrollIntoView` inside the app shell** — scroll the intended scroller
with `scrollTo` instead (`QueueBody`'s current
row). `scrollIntoView` reveals its target in every scrollable *ancestor* and in
the **visual viewport**: mobile Chrome's URL bar shrinks the visual viewport
while the layout viewport stays large, so there is always URL-bar-height of room
to offset it — revealing a panel slid the whole app up under the URL bar and
left the layout viewport's tail as dead space above the system nav, permanently,
since no document-level scroller can put it back. Emulation has no separate
visual viewport, so it never reproduces.

**Settings on phones.** `SettingsLayout` (`layouts/SettingsLayout.vue`)
renders its sidebar as a horizontally-scrolling icon bar below 768px —
collapse is a desktop concept with no room on a bar, so the collapse button
and width machinery are hidden entirely. Settings tables use `useViewport().tier`
to hide secondary columns on phones (via `Column :hidden` binding); hidden data
either moves into a visible cell (the library path renders under the name on
phone) or stays reachable through an icon-only button (the schedule calendar
icon) or the row's edit dialog. The metadata editor (`MetadataEditorView`)
stacks its split panels vertically and shows an info notice ("works best on a
larger screen") on phone tier. **The `ContentScaffold` header wraps at any width
and its title never shrinks below 12rem** (empty titles exempt) — this ensures
the back button, title, and actions stay readable when the header reflows across
narrow viewports.

Keyboard shortcuts (`useKeyboardShortcuts`) and `ShortcutHelpOverlay` bind in
**`DesktopShell` only** — mount-scoped listeners (the reason the shells are
components rather than inline `v-if` blocks).

Mobile chrome is **one component**: `NowPlayingSheet` (`components/layout/`),
an always-mounted bottom sheet rendered by `MobileShell` over the route
content (plus a `.mini-spacer` flex child reserving the collapsed strip's
height, since the sheet overlays rather than docks). The sheet has three
detents — collapsed (mini strip) / playing (`PlayerFace`) / queue
(`QueuePanel`) — addressed by the **hash of whatever route is underneath**:
`''` / `#playing` / `#queue`. The hash is the single source of truth
(spec: `docs/superpowers/specs/2026-08-16-mobile-sheet-navigation-design.md`):
buttons and gestures commit it via `commitDetent` (deeper = `push`, so system
back walks `#queue` → `#playing` → page; shallower = `back()` when
vue-router's `history.state.back` matches the target, else `replace()`), and
the sheet's hash watcher animates toward it. Do not add parallel open/close
state, and do not turn Now Playing back into a route view — the content view
staying mounted underneath is what makes the dismiss drag reveal real UI.

Gestures (all finger-following, pure math in `lib/sheetGesture.ts`, state in
the `useNowPlayingSheet` singleton): lift the strip to open; drag the face up
for the queue or down to dismiss; pull the queue list down from its top — or
drag the queue heading at any scroll position — to return to the face. Claims
need 8px of dominant-vertical movement, the seek bar never arms, a claimed
drag swallows its release click, and settles honour flick velocity. While the
sheet is above collapsed, `PlayerLayout` sets `inert` on `.body-row`; the
collapsed sheet body and the faded strip are likewise `inert`. Reduced motion
is pure CSS (nothing waits on `transitionend`). On mobile, `/` is an alias:
`HomeView` `replace()`s to `/browse#playing` (queue filled) or `/browse`
(empty). The sheet strips a live sheet hash on unmount (queue emptied,
rotation to desktop). `MiniPlayer` is the sheet's dumb collapsed strip — it
renders and emits `open`, nothing more.

**`/browse` (`MobileBrowseView`) is the mobile shell's landing page and its whole
navigation surface** — the phone's stand-in for `AppSidebar`, and where the
hamburger `ContentScaffold` puts at the head of every top-level view's header
navigates (detail views show Back in that spot instead; the browse view itself
passes `navRoot` so it grows no button back to itself). It is a *page*, so each
destination shows what is in it: one `BrowseShelf` per section — Library (samples
the ranked `useDiscoveryFeed`, the same query `/library`'s Discover tab renders in
full), one `BrowseAlbumShelf` per dynamic library (its newest albums; a component
per library because a composable cannot be called in a loop over a reactive list),
then Playlists, Genres, Radio — each a heading, `BROWSE_SHELF_SIZE`
(`lib/browseShelf.ts`) items in a horizontally snapping strip, and a "See all"
link to the full view. Per-library shelves appear only above one library, matching
the sidebar. Two things have no shelf to fill and sit in the header: Search, and
the account entries the desktop keeps in `UserMenu` (User settings → Admin →
About → Log out) behind a `⋮` PrimeVue `Menu` — the phone's **only** way to log
out. Now Playing and the queue stay reachable through `MiniPlayer`. Mobile only:
at desktop width the view `replace()`s the route with `/library`, the mirror of
`HomeView`'s guard.

**useMediaSession** is bound once from `PlayerLayout` (shell-independent — desktop
gains hardware media keys from the same wiring), feature-detected so unsupported
browsers and jsdom are silent no-ops. Publishes `MediaMetadata` (title/artist/album
plus artwork at 96/256/512px), mirrors `isPlaying` into `playbackState`, wires
play/pause/previoustrack/nexttrack/seekto actions, and updates `setPositionState`
on duration/seek changes — not on every `currentTime` tick (the browser
extrapolates between updates).

## Player (`composables/usePlayer.ts`)

Dual `<audio>` elements: active plays, standby pre-buffers the next track
(`preload="auto"`); on `ended` the roles swap. This removes the audible
inter-track gap without Web Audio.

**`swapToStandby` must pause the outgoing element**, and does so *after* the swap.
On the `ended` path it has already stopped, but a **skip** hands over
mid-playback and would otherwise leave the old track sounding under the new one.
That was a real bug and it only showed at the END of the queue: mid-queue the
following `updatePreload()` re-points the outgoing element's `src`, which stops it
as a side effect, but with nothing left to preload `updatePreload()` bails early
on `url === preloadedUrl` (both null) and never touches it — so skipping from the
second-to-last to the last track played both at once. Do not "simplify" that
`pause()` away, and keep it after the swap: the 'pause' listener is guarded by
`el !== activeEl`, so pausing before the swap sets `isPlaying = false`, and since
browsers fire 'pause' asynchronously it can land after the incoming `play()` and
leave a play icon over a playing track. Covered by `usePlayer.skipToLast.spec.ts`,
whose fake element models the real abort-on-`src`-assignment so the masking
effect is reproduced rather than assumed. **Sample-accurate gapless was assessed and
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

## Keyboard shortcuts

`utils/shortcuts.ts` is the **single registry**: the key map the global listener
matches, the rows the About view (`/about`) renders, and the badges the help overlay
pins all read from `SHORTCUTS`. Adding a binding there is all that is needed for
it to appear in both surfaces — never hand-copy the list into a view.

Every binding is a **bare key**; the matcher declines any press carrying
Ctrl/Cmd/Alt, so nothing can collide with a browser shortcut. Shift is the one
exception, because `?` cannot be typed without it. Bindings: `Space` play/pause,
`N`/`B` next/previous (`B` as in "back"), `←`/`→` seek ±5s, `↑`/`↓` volume,
`M` mute, `L` favorite, `Q` queue, `C` Now Playing, `D` library, `P` playlists,
`G` genres, `R` radio, `S` search, `?` help, `Esc` close. There is deliberately
**no shuffle or repeat binding** — which is what leaves `S` and `R` free. Previous
was `P` and playlists `O` at first; moving previous to `B` freed `P` for the
destination whose initial it is, so **`O` and `/` are both unbound now** — `/` went
back to Firefox's quick-find. Don't re-add either as an alias.

Every nav binding pushes a **named route with no params**: `C` → `home` (Now
Playing is `/`, where `HomeView` renders `QueueView` on desktop and
on mobile redirects to `/browse` with `NowPlayingSheet` open — there is no separate now-playing
route), `D` → `library` with no `folderId` (the cross-collection root
that opens on the Discover feed — a `folderId` would scope it to one library,
which is not what the sidebar's Library entry does), plus `P` → `playlists`,
`G` → `genres`, `R` → `radio`, `S` → `search`.

Two registry-wide guards in `shortcuts.spec.ts` matter at this size: no key may be
bound to two actions, and every **displayed** key must resolve to the action it is
listed under. A duplicate silently makes one action unreachable (its `KEY_MAP`
entry never wins), and a row whose `keys` disagree with `KEY_MAP` renders a badge
for a key that does nothing — both invisible without those tests.

The four arrows are the transport, matching what every web player does (YouTube:
`←`/`→` seek, `↑`/`↓` volume). Handled keys are `preventDefault`ed, so `↑`/`↓`
**no longer scroll the page** — `PageUp`/`PageDown`/`Home`/`End` are left unbound
on purpose as the keyboard-scroll fallback, and a press on a focused rail handle
still reaches PrimeVue (see `isTypingTarget`) so keyboard seeking keeps working.
Volume was on `+`/`-` initially; don't reintroduce those as aliases — one binding
per action, so one badge teaches the whole thing.

`useKeyboardShortcuts` is bound from **`PlayerLayout`**, not globally — these are
player actions and `SettingsLayout` gets none of them. `isTypingTarget` skips a
press whose focus is in an `input`/`textarea`/`select`/`[contenteditable]` or on
a `[role="slider"]`; that last one is load-bearing, since the volume and seek
handles own the arrow keys and taking them away would break keyboard seeking.
Handled keys are `preventDefault`ed (Space and the arrows scroll the page);
unhandled ones fall through untouched. `Esc` is only claimed while
the overlay is up, so `EditActionBar` and dialogs keep it otherwise.

`ShortcutHelpOverlay` is **measurement-driven, not breakpoint-driven**: on open
it reads `getBoundingClientRect()` for each `[data-shortcut="<anchor>"]` control
and pins a badge at the rect's centre, lifted clear by a
`translate(-50%, -100%)` transform so the badge's own size never has to be known
in JS. That is why the three media queries in `PlayerControls` need no counterpart
here — and why a control the layout has hidden (the volume rail and speaker are
`display:none` under 768px) reports a **zero rect and is skipped**, with the side
panel listing that key instead. It re-measures on `resize`. Do not replace this
with per-breakpoint offsets.

**Two actions may share one anchor**, and then their badge shows both keys: the
volume pair (`↑ ↓`) sits on the volume rail and the seek pair (`← →`) on the
progress rail, because a lone arrow over a bar teaches half the binding. The pair
renders in registry order, so each is listed the way it should read on the badge.
An anchor need not be in the player bar — all six nav shortcuts are on their
`AppSidebar` nav entries, the only controls that open them. Actions carrying a
badge are dropped from the panel (both halves of a shared pair), so no key is
listed twice.

Two levels of hiding, and they are not the same: **`hidden`** drops a row from
the About list *and* the overlay (`Esc`, which only means anything while the
overlay is up), while **`overlayHidden`** drops it from the overlay only (`?` —
you just pressed it to get there, so a row for it teaches nothing, but About is
the full reference and keeps it). Hence two exports: `VISIBLE_SHORTCUTS` for
About, `OVERLAY_SHORTCUTS` for the overlay. With `?` gone from the overlay and every
other action anchored, the panel's list is legitimately **empty on a wide screen**,
so it is `v-if`'d away rather than rendered blank under the title; the panel still
carries any action whose control this width has hidden.

A registry entry's **`place`** picks the badge's side of the control: `above`
(default) floats it in the dimmed space over the player bar; `side` puts it just
past the control's right edge, vertically centred on its row. Every nav shortcut
uses `side`, because a sidebar entry has another nav item above it, so the
floating placement would cover the neighbour instead of labelling the target. The
transform per placement is what does the actual placing, and scoped styles never
apply under vue-test-utils, so `ShortcutHelpOverlay.badgeStyles.spec.ts` parses
the style block — same reason `PlayerControls.railStyles.spec.ts` exists.

Sidebar anchors come from `NAV_SHORTCUT_ANCHORS` in `AppSidebar`, applied to
`primaryItems` and `libraryExtras` but **never to `folderItems`**: the per-folder
entries share `routeName: 'library'` with the root, so anchoring them too would let
the overlay badge whichever it found first instead of the cross-collection root.
That is why this is a `routeName` lookup applied per loop rather than one blanket
attribute, and why `AppSidebar.shortcutAnchor.spec.ts` asserts a bare *count* of
anchored entries — a count is what catches an anchor leaking onto the folder rows.

The panel sits **top-right**, not centred: the player bar it badges runs along the
bottom and the nav entries it badges run down the left edge, both of which a
centred panel covered. Its placement is a scoped style, so
`ShortcutHelpOverlay.badgeStyles.spec.ts` asserts it off disk (pinned by
`top`/`right`, not `left: 50%` + translate, and bounded by
`--app-player-height`). The two anchor specs **partition the
registry** by `place` — `PlayerControls.shortcutAnchors.spec.ts` asserts every
above-placed anchor, `AppSidebar.shortcutAnchor.spec.ts` every side-placed one —
both derived from `SHORTCUTS`, so a newly added anchor cannot be missed by both.

The favorite action is shared: `useSongFavorite` owns the optimistic `starred`
flip, and `useCurrentTrackFavorite` is just that composable bound to the playing
track, so the bar's heart, the `L` key and every track row's heart cannot
diverge. Anchors are asserted against the registry in
`PlayerControls.shortcutAnchors.spec.ts`, so a renamed control cannot silently
lose its badge.

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
User settings (`/user-settings`) theme picker. This is intentional — don't "clean it up".

## Icons and static assets (`webui/public/`)

Everything in `webui/public/` is copied verbatim into `webui/dist` by Vite and
from there into the Go binary by `make package-ui` — so a file added here is
served at its own root path (`/favicon.ico`) instead of hitting the SPA
index.html fallback.

The icon set is **generated, not hand-edited**: `zarf/icon/icon2.svg` is the
Inkscape master, `zarf/icon/web/` holds three cleaned derivatives, and
`make icons` (`zarf/icon/render.sh`, needs inkscape + ImageMagick) renders them
into `webui/public/`. Re-run it after changing the artwork and commit the
output.

| Source | Output | Why it exists |
|--------|--------|---------------|
| `web/icon.svg` (rounded) | `icon.svg`, `favicon.ico` (16/32/48), `icon-192.png`, `icon-512.png` | tab favicon (SVG preferred, ICO for clients that just GET `/favicon.ico`) and the manifest's `purpose: any` icons |
| `web/icon-square.svg` (full-bleed, opaque) | `apple-touch-icon.png` (180) | iOS home screen: Safari ignores manifest icons and applies its own corner mask, so this one must bleed into the corners and carry no alpha |
| `web/icon-maskable.svg` (mark at 70%) | `icon-maskable-192.png`, `icon-maskable-512.png` | Android adaptive icons (`purpose: maskable`) only guarantee the inner 80% circle |

**In-app brand mark.** The same artwork appears inside the UI as the diamond
rendition: `assets/aether-mark.svg` (cleaned from `zarf/icon/icon.svg`), wrapped
by `components/common/BrandMark.vue` — the single place that decides it is
decorative (empty `alt` + `aria-hidden`, because the "Aether" wordmark always
sits beside it) and takes a `size` prop. Used by `AppSidebar` and `LoginView`; in
the sidebar it is also the hidden-themes easter-egg trigger, so it must stay a
non-focusable, unannounced element. It is an `<img>`, not inline SVG, because two
inlined copies would collide on the gradient's element id.

Two server-side details make this work: `app/spa/spa.go` registers the
`.webmanifest` and `.ico` MIME types (Go's built-in table has neither, and a
sniffed `text/plain` manifest disables install/standalone mode), and
`index.html` carries the `apple-mobile-web-app-*` metas — Safari needs its own
copies of what the manifest already says.

## Testing (see [testing.md](testing.md))

`npm test` = `vue-tsc --noEmit` + Vitest (jsdom). Specs live in `__tests__/`
next to the code. Convention from the layout registry: a new main content
view tests title, pluralized summary (absent at zero), header actions, and
empty/loading states — copy `RadioView.spec.ts` / `LibraryView.spec.ts`.
Count summaries are built as single strings so `.text()` assertions stay
reliable.
