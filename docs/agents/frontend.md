# Frontend — structure, conventions, the three registries

`webui/` is a Vue 3 + TypeScript SPA (Vite, PrimeVue 4, Pinia, TanStack
vue-query, vue-router). Built output is copied into `app/spa/files/ui` and
embedded in the Go binary (`make package-ui`); in the binary the SPA is the
catch-all route behind `/api/v0` and `/rest`. **The same `webui/dist` is also
the desktop player's window** (`player/`, Tauri — see
[player/architecture.md](player/architecture.md)), served there from a
`tauri://` origin rather than by the Go binary: keep the SPA origin-agnostic
(no hardcoded origins, no server-side `index.html` templating, no cookie-only
auth assumptions). Whatever differs inside the app goes through the host layer
(`lib/host/`, see "Desktop app mode" below), and player-only components are
never added ad hoc — they belong to the playback retrofit, which is tracked in
`TODO.md`.

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
  `SettingsLayout`'s `.settings-content` is the one vertical scroller for every
  settings view: the layout is a single viewport tall, so a view taller than the
  pane scrolls there, while the metadata editor fills it (`height: 100%`) and
  scrolls its own panels. Pinned off disk by `SettingsLayout.scrollStyles.spec.ts`
  — the pane used to clip, leaving the bottom of tall settings pages unreachable.
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
  `queryKeys` map — add new keys there, don't inline key arrays). The metadata
  editor lists the server's configured scan folders via `useScanFolders`
  (`GET /api/v0/scan-folders`) and addresses every editor request — folders,
  tracks, pictures, identify, artist image — by the chosen folder's `name`,
  not a numeric id.
  `usePlayer.ts` is module-scoped singleton state, as is
  `useIdentifyCache.ts` — the metadata editor's identify answers are cached
  per (scan folder, path) and per selection set, LRU-capped, so reopening an
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
  `useReleaseGroupTypes.ts` is its twin for the group's release types (primary
  first, then secondary): built on the same `releaseGroupCache.ts`, staged
  through the `release_types` field checkbox, and under the same rule — no
  types, nothing staged.
  After any metadata-editor write, call `invalidateAfterMetadataWrite(qc)`
  from `useMetadataEditor.ts` rather than inlining keys. It drops
  `['metadata','tracks']` and `['metadata','raw']` (the editor's own views)
  plus the entire `['subsonic']` tree (the music UI). The last one is blunt on
  purpose: artist and genre ids are not stable across a tag edit, and album ids
  still churn on partial edits, merges and identity swaps (see `scanning.md`).
  The editor works in file paths, so there is no precise key set to target. The
  server re-indexes synchronously, so a resolved write already means the DB is
  current; no polling needed.
- `lib/api/` — HTTP clients: `client.ts` (axios `apiClient`, `/api/v0`,
  overridable via `VITE_SERVER_URL_V0` in the browser; in the desktop app it
  rides the host transport — `adapter: 'fetch'` with `env.fetch` →
  `httpFetch` — and an interceptor sets `baseURL` to `${serverUrl}/api/v0` on
  every request, since the server can change while the app runs, and refuses
  to send while none is chosen and in a key session) and `subsonic.ts`
  (`SubsonicClient`; its server is the host layer's `serverUrl` — the page's
  origin in the browser — read when the client is configured, and every call
  goes through `httpFetch`; `initWithDefaults()` is the
  **auth-method-`none`** path only — authenticated modes call `setApiKey()`
  with the PAT minted by `lib/subsonicSession.ts`, or with the stored key in a
  key session — and `reset()` forgets either, server included; see
  [subsonic-api.md](subsonic-api.md) and
  [authentication.md](authentication.md)). `Server.ts` is the desktop app's
  server probe and, in `verifyApiKey`, the check of a pasted key (see "Desktop
  app mode" below), and `meShape.ts`'s `isMeResponse` is what counts as
  Aether's `/me`: `getMe` rejects any other answer, and the probe applies the
  same check.
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
with `scrollTo` instead (`QueueBody`'s current row). `scrollIntoView` reveals
its target in every scrollable *ancestor* and in
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
either moves into a visible cell (the libraries table's filter summary and
the scan folders table's path both render under the name on phone — see
"Admin — libraries and scan folders" below) or stays reachable through an
icon-only button (the schedule calendar icon) or the row's edit dialog. The
metadata editor (`MetadataEditorView`) stacks its split panels vertically and
shows an info notice ("works best on a larger screen") on phone tier. **The
`ContentScaffold` header wraps at any width and its title never shrinks below
12rem** (empty titles exempt) — this ensures the back button, title, and
actions stay readable when the header reflows across narrow viewports.

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
the ranked `useDiscoveryFeed`, the same query `/library`'s Discover view renders in
full), one `BrowseAlbumShelf` per dynamic library (its newest albums; a component
per library because a composable cannot be called in a loop over a reactive list),
then Playlists, Genres, Radio — each a heading, `BROWSE_SHELF_SIZE`
(`lib/browseShelf.ts`) items in a horizontally snapping strip, and a "See all"
link to the full view. Per-library shelves appear from the first library on,
matching the sidebar: a library is a saved filter, so even the only one is
narrower than the catalog the Library shelf samples.
Two things have no shelf to fill and sit in the header: Search, and
the account entries the desktop keeps in `UserMenu` (User settings → Admin →
About → Change server, in the desktop app only → Log out) behind a `⋮`
PrimeVue `Menu` — the phone's **only** way to log out, and, in the app, its
only way back to the server step. Now Playing and the queue stay reachable
through `MiniPlayer`. Mobile only:
at desktop width the view `replace()`s the route with `/library`, the mirror of
`HomeView`'s guard.

**useMediaSession** is bound once from `PlayerLayout` (shell-independent — desktop
gains hardware media keys from the same wiring), feature-detected so unsupported
browsers and jsdom are silent no-ops. Publishes `MediaMetadata` (title/artist/album
plus artwork at 96/256/512px), mirrors `isPlaying` into `playbackState`, wires
play/pause/previoustrack/nexttrack/seekto actions, and updates `setPositionState`
on duration/seek changes — not on every `currentTime` tick (the browser
extrapolates between updates).

## Desktop app mode (`lib/host/`)

Inside the desktop player the SPA runs from a `tauri://localhost` /
`http://tauri.localhost` origin, so the server is not the page's origin, and a
request the webview sends itself cannot reach it: cross-site, it would need
CORS and would never carry the `SameSite=Lax` session cookie, and the window's
CSP confines `connect-src` to IPC anyway (the Rust side and the why:
[player/architecture.md](player/architecture.md)). **`lib/host/` is the one
place that knows** — `index.ts` (the API, imported as `@/lib/host`),
`serverUrl.ts`, `session.ts`, `transport.ts`, `events.ts`. Everything else asks
it and stays the same code in a tab and in the app.

- `isDesktopApp()` — `isTauri()`, cached. `initHost()` settles the server
  before anything renders: `main.ts`'s `bootstrap()` awaits it before
  `app.mount` (an async function rather than a top-level await, which Vite's
  default build target rejects). In the app it imports
  `@tauri-apps/plugin-http` lazily (the browser never evaluates it), installs
  the plugin's `fetch` as the transport and reads `get_settings`. It never
  throws: a failure reading the settings leaves the server unset, so the login
  view asks for one instead of the window staying blank, and a failed
  `get_api_key` leaves the server and the kind set with no key, so the key
  card asks again.
- `serverUrl` (a ref) — where every `/api/v0` and `/rest` request goes, with
  no trailing slash: the page's origin in a tab (never null); in the app the
  saved server (an origin, plus a path prefix when the server sits under one),
  null until one is chosen. `setServerUrl(url, kind)` saves it, with how to
  sign in to it (`cookie` or `apiKey`), and adopts the URL as Rust normalised
  it; `clearServerUrl()` forgets it; both throw in a tab, where only the page
  decides the server.
  `setSession`/`clearSession` hand the `/rest` key to the Rust client and take
  it back (no-ops in a tab); `appVersion()` is the player's version (null in a
  tab).
- `sessionKind` / `apiKey` (`session.ts`) — how the SPA signs in (`cookie` |
  `apiKey`; a tab is always `cookie`) and, in a key session, the stored key in
  memory. `initHost` reads both (`get_api_key` only in a key session).
  `setSessionKind(kind)`, `setApiKey(key)` and `clearApiKey()` wrap
  `set_session_kind`, `set_api_key` and `clear_api_key`.
- `httpFetch` — `fetch` through the installed transport, else the global one,
  resolved on every call (specs stub the global after import). The plugin
  cancels only through `init.signal`, so `initHost`'s wrapper lifts the signal
  a `Request` carries — where axios's fetch adapter puts it — into `init`: an
  aborted request (a superseded preview, a cancelled identify) stops in Rust
  and rejects, as in a tab.
- `onTrayCommand(cb)` — subscribes to `tray://command` (`play_pause` or
  `next`; any other payload is dropped; a no-op in a tab). Its one caller is
  `useTrayBridge` (below).

**Every request to the server goes through `httpFetch`.** `apiClient` does in
the app (see `lib/api/` above), `subsonic.ts` always does, and the final
play-queue save on `pagehide` (`savePlayQueueBeacon`) takes it in the app
instead of `sendBeacon`, which the CSP blocks. Media is the deliberate
exception: `getStreamUrl` / `getCoverArtUrl` are plain URLs on the server with
the `apiKey` in the query, loaded by `<audio>` / `<img>` straight from it.
**Never show or log a transport rejection's text**: the plugin rejects with a
plain string that names the request URL, `apiKey` included. The server step
renders only its own copy, `subsonic.ts`'s warnings are bare labels, and
`apiErrorMessage` turns a request that got no response into
`NETWORK_MESSAGE`. Known gap: the metadata
editor's picture previews and copy-picture bypass the transport and do not
load in the app ([player/features.md](player/features.md)).

**Two rules, pinned off disk by `lib/host/__tests__/originAgnostic.spec.ts`.**
Outside `lib/host/`, no code reads `location.origin` — however it is reached
(`window.`, `document.`, `globalThis.` or bare): inside the app it is the wrong
answer to "where is the server", so ask `serverUrl` — and none imports
`@tauri-apps/*`, so the dependency on the app stays behind one seam. A
string-aware stripper tells code from comments and strings, so prose may quote
either, and a vacuity check asserts that the host layer's own files trip both
matchers, so a pass is never for lack of looking.

**The gate** (`useAuth`, `App.vue`, `LoginView`). The login view replaces the
app while any of three holds:

- `needsServer` — app only: no server is chosen, or the identity query errored
  (`/me` in a cookie session; `getUser` in a key session, for any reason but a
  refused key, and only while no identity is known yet — a failed refetch of a
  known one never drops the running app to the server step) — the saved server
  is down or has moved, and the window would otherwise stay blank with no way
  to pick another.
- `needsLogin` — unchanged: a cookie session on a `native` server with no user,
  or an expired session.
- `needsApiKey` — app only: a key session without a key (none pasted yet,
  forgotten after a logout, or refused — which also sets `sessionExpired`), and
  the one cookie case the app cannot sign in to, `/me` reporting `proxy-header`
  and no user (a proxy that lets an anonymous `/me` through). The key card asks
  for a key and offers *Change*.

Two kinds of session, `sessionKind` in `lib/host/` (`cookie` | `apiKey`, a
browser tab is always `cookie`): `useMe` is enabled only in a cookie session;
`useRestIdentity` (`composables/useUsers.ts`, key `userQueryKeys.restIdentity`)
asks `getUser` over `/rest` in a key session once the key is installed, and
maps `username`/`adminRole` to the `{login, role}` shape. The credential
watcher in `useAuth` installs the stored key on the subsonic client (no mint,
no hand-off: Rust holds the key itself) and resets that client when a key
session ends or loses the key it had, so nothing stale reaches the next
server. `useAuth` exposes `managementApi` — a cookie session whose `/me`
answered — and **everything that rides `apiClient` renders only when it is
true**: the Admin entry (`UserMenu`, the phone's `⋮`) and the Metadata editor
entry (`UserMenu`), the settings-layout redirect in `App.vue`, the Connected
apps tab, the artist image search and source note, the radio-browser Discover
button, the `/api/v0` version in About (a key session reads it off a `/rest`
ping instead). `isAdmin` alone keeps gating the `/rest` admin actions, so an
admin's key still edits covers and radio stations; the artist image's Remove,
though, needs the `/api/v0` image source to tell Aether's image from a
folder's, so a key session offers it only for an upload staged in that edit.
`canSignOut` (a native or a key session) is what shows Log out. In a key
session `client.ts` refuses to send at all, and a `/rest` error 40 or 44 sets
`sessionExpired` instead of minting (`handleRestAuthFailure` in
`lib/subsonicSession.ts`; `remintApiKey` answers `failed` without a verdict, so
a media element error reports connectivity rather than signing out); the expiry
purge forgets the key (`clearApiKey`) before the usual scrub.

`useMe` is disabled until a server is known (`enabled` — a disabled query is
not loading, so the gate renders the server step instead of waiting), does not
retry on mount (`retryOnMount: false` — the login view mounting on a failed
`/me` would refetch it, clear the error, unmount, fail and mount again, a
flicker loop), and in the app neither retries (`retry: false` — Connect is the
retry, and three more attempts on top of the timeout below would keep the
window blank for most of a minute) nor refetches when the network comes back
(`refetchOnReconnect` — it would take the server step away mid-typing); the
browser keeps both. `getMe` carries a 10 s timeout so a server that drops
packets cannot hold the window blank for minutes, and rejects an answer that
is not shaped like Aether's `/me` (`isMeResponse`), so a saved address that
now answers as something else — another web app on that port, a captive
portal's page — brings the server step back instead of rendering the app
against it.

**The server step** (`LoginView`, app only) is a "Server address" field
(placeholder `http://aether.local:8075`) and Connect. Entering the step, the
field offers `lastServerUrl` (`lib/authState.ts`: the server last left through
Change), else the saved server that did not answer — saying "Could not reach
{url}." under it. Connect runs `probeServer` (`lib/api/Server.ts`), which talks
through `httpFetch` directly rather than `apiClient` (that client refuses to
send without a server, and its 401 handling would read the probe's answers as
a lost session):

1. Normalise: trim, prepend `http://` when no scheme is typed, refuse another
   scheme, refuse embedded credentials up front ("Enter the server address
   without a username or password." — a typed password is never sent or shown
   back), keep the origin and a path prefix, drop query, fragment and trailing
   slashes.
2. `GET /api/v0/me` with `redirect: 'manual'` + `maxRedirections: 0` (a
   redirect is an answer to look at, not to follow) and a 10 s timeout. An ok
   answer shaped like Aether's `/me` (`isMeResponse`: a known `authMethod`, a
   `user` that is null or has a `login`, a `features` object) is Aether. A
   301/302/307/308 from an http address to `https` on the same host re-runs
   the probe once there, and that https address is the one saved.
3. Anything else: `GET /rest/ping?f=json&v=1.16.1&c=aether-player`. `/rest`
   stays open in every proxy setup, and Aether stamps `type: "Aether"` on every
   envelope, errors included — such an envelope (compared without case) is
   Aether behind a portal: the step saves the address as a key session
   (`setServerUrl(url, 'apiKey')`), and the gate shows the key card.
   Any other envelope (Navidrome, gonic…) or body gives "No Aether server
   answered at {url}.", and a request that fails or times out "Could not reach
   {url}."

An answer is saved (`setServerUrl(url, kind)`) and `serverSaved()` starts a
fresh session (`startFreshSession()`) and resets an errored identity query, so
the address is asked again even when unchanged. The kind is `cookie` for an
Aether `/me` and `apiKey` for a portal — and also for an Aether answer on the
saved server of a key session (`sessionKind` is `apiKey` and the probe settled
on the saved address), which is what Connect after an outage does. After
*Change* the kind is already `cookie`, so a new server never inherits a key
session. Rust keeps the stored key in that same case (the same server saved
again for a key session), so Connect never costs it. The gate then shows the
app (auth `none`, or a session the cookie jar still holds), the sign-in card
(`native` without one) or the key card (a key session without a key). In the
app the sign-in card carries a muted `Server · host` line with a *Change* link
above the username, and a "Use an API key instead" link
(`setSessionKind('apiKey')`, then `serverSaved()`).

**The key card** (`LoginView`, app only): the server line with *Change*, one
masked "API key" field with the hint "Create one in Aether → User settings →
Connected apps.", Connect, and a "Sign in with a password instead" link.
Connect verifies, seeds, then stores: `verifyApiKey(serverUrl, key)`
(`lib/api/Server.ts`, one `getUser` through `httpFetch` with the probe's
timeout, so no unverified key reaches `subsonicClient`), then
`useAuth().apiKeySaved(user)`, which seeds `useRestIdentity` with the answer,
then `setApiKey(key)` (Rust stores it and makes this a key session). Seeding
comes first on purpose: the stored key installs on the subsonic client and
enables the identity query, which then renders from the seed instead of asking
`getUser` a second time. Error 40 or 44 is "This API key was not accepted." (the
field is cleared); Aether's own other errors show their message; everything
else shows the probe's messages, never a transport rejection's text. A key the
server stopped accepting shows "Your API key was not accepted — it may have been
revoked. Enter a new one." (on `sessionLostUnexpectedly`, never after a
deliberate logout). The password link re-probes the saved server: an `aether`
answer reporting `native` or `none` switches the kind to `cookie`; a `portal`,
or `proxy-header`, shows "This server signs you in through its portal, so only
an API key works here." Both method links start a fresh session
(`serverSaved()`) once they switch the kind — this one and the sign-in card's —
so the other method's expiry flag never carries over (a cookie session's would
blame a revoked or expired key on someone who never had one). The key field is
cleared whenever the server step opens: a key belongs to one server, and must
not ride into the next one's `getUser`.

**Leaving a server** is `switchServer()` in `useAuth`, which never rejects: a
best-effort logout when signed in (bounded by the logout request's 10 s
timeout, and sent while the server is still set, since `apiClient` refuses
once it is forgotten) → remember it in `lastServerUrl` → `serverUrl = null`
before any cache reset (the now-disabled `/me` cannot refetch from the server
being left) → `purgeLocalSession` on every path, signed in or not (the old
server's queue, and the queue sync, would otherwise carry its track ids over to
the next one) → `startFreshSession()` → `clearServerUrl()`, best effort. The
purge forgets a key session's key first (`clearApiKey`), and `clearServerUrl`
deletes it on disk and resets the kind; a key session has no `/api/v0` logout
to send. What the switch overtook still answers afterwards, before the next
server is saved or after, speaks for the session just ended and flags nothing —
kept, an expiry flag would stop a server without login from ever rendering. In
the app `client.ts` flags an expiry only for a 401 whose request went to the
server configured now (its stamped `baseURL`); a mint's outcome counts only
while its session generation is current (see the hand-off below); and
`serverSaved()` clears anything else that flagged the old session before the
next server was saved. The sign-in card's *Change* calls `switchServer()`
directly. The account menus go through
`composables/useChangeServer.ts`: `confirmChangeServer()` asks "Disconnect from
{host}? You will be signed out." in the `change-server` confirm group, which the
one grouped `ConfirmDialog` that `PlayerLayout.vue` mounts in the app answers —
outside the shell swap, so it serves both shells, and grouped so the views' own
ungrouped dialogs do not show the question too. *Change server* (`ms-dns`)
sits in `UserMenu` and in `MobileBrowseView`'s `⋮`, rendered in the app only.
About (`AboutView`) adds *App* (the player's version, from `appVersion()`) and
*Server* (the full address) rows in the app, and relabels *Version* as *Server
version*.

**The session hand-off** (`lib/subsonicSession.ts`): `remintApiKey` pushes
each new `spa` token to Rust (`setSession`) only inside its generation-checked
branch — Rust takes a bare key for whatever server is set, so a key minted for
a server the user has since left must never reach it — and
`resetSubsonicSession` withdraws it (`clearSession`) and resets the client
whole (`SubsonicClient.reset()`: the key, auth `none` and the server), so
nothing addresses the server left behind until the next session configures
it. The reset also bumps the session generation and lets go of a mint still
out: that mint's outcome, success or refusal, is discarded (`'failed'`, no
expiry flag from `remintApiKey` itself, no retry budget spent), the next
session mints its own instead of joining it, and `useAuth`'s mint watcher
ignores an outcome whose generation moved while it waited
(`sessionGeneration()`). That mint's request rides `apiClient`, though, so the
`client.ts` interceptor still flags its 401 whenever the request went to the
configured server (always in a browser tab; in the app when the server did not
change). A failed hand-off only warns, with neither the key nor an error text
that could hold it. In the app `getDeviceName()` (`lib/deviceIdentity.ts`)
answers "Aether Player on <platform>" — the webview's user agent would read as
a browser — so Connected apps names the player.

**`useTrayBridge`** (`composables/useTrayBridge.ts`) is bound once from
`App.vue`, which is always mounted, in a detached effect scope like
`useMediaSession`'s: the tray's `play_pause` calls `usePlayer().togglePlayPause()`
and `next` calls `playNext()` on every layout — the settings routes unmount
`PlayerLayout` while the music plays on. A refused subscription only warns;
the app works without the tray.

## Admin — libraries and scan folders

`LibrariesView` (`views/settings/LibrariesView.vue`, `/settings/libraries`)
renders two independent panels: `ScanFoldersPanel` (read-only) then
`LibrariesPanel` (create/edit/delete). A library owns no directory — it is a
named, **filtered view** over the whole catalog (see
[architecture.md](architecture.md#scan-folders-config-only)) — so neither
panel shows a path, a last-scan time, or a config-provisioned badge; the
directories actually scanned live only in the server's config file.

**`LibrariesPanel`** columns: name (icon + name, plus a `severity="warn"`
"Needs attention" `Tag` — tooltip is the joined `warnings[].detail` — shown
only when the library has any), Filters (one `Tag` per
`lib/libraryFilters.ts`'s `summarize(filters)` line, or a single "Whole
catalog" tag when `filters` is empty), Tracks, actions. Deleting asks "Delete
library "<name>"? Only this view is removed — no track, star or play history
is touched." — deleting a library never touches `tracks`. On phone
(`useViewport().tier === 'phone'`) the Filters and Tracks columns hide and
the filter summary renders a second time under the name instead (the
"Settings on phones" pattern above); `settingsTables.phone.spec.ts` pins both
tiers for this table.

**`ScanFoldersPanel`** lists the folders read-only — no dialogs, nothing to
edit — over `useScanFolders()` (`GET /api/v0/scan-folders`): Name, Path,
Excludes (a count; the patterns themselves are the tooltip), Symlinks
("Followed"/"Not followed"), Tracks, Status (a `Tag`, `success` "Available"
or `danger` "Not usable" with the server's raw `problem` string as tooltip
AND as small secondary text under the tag — a tooltip alone is unreachable by
touch and keyboard, and this read-only panel has no dialog to show it in).
Its hint, shown above the table regardless of data: "Defined in the server's
config file under ScanFolders; restart the server to apply changes." With
zero scan folders configured, the empty state spells out the consequence
rather than just the fact: "No scan folders are configured — nothing is
scanned and no on-disk media is served. Add them under ScanFolders in the
server's config file and restart." — the media guard fails closed with no
scan folders configured (see [subsonic-api.md](subsonic-api.md)). On phone,
Path/Excludes/Symlinks hide and the path renders under the name instead;
Tracks and Status stay on every width.

Its one button, **"Scan now"** in the section header, is a shortcut to the
incremental `scan` task ("Catalog Scan", the same one the Tasks page runs),
through `useCatalogScan()` (`composables/useCatalogScan.ts`):

- It follows the runner's execution list through `useExecutions()`, the
  query the Tasks view uses too (polled while any run is queued or running).
  While a `scan` run is active, whoever started it (this button, the Tasks
  page, a schedule), the button is disabled and its label is that run's
  `progressLabel` ("Queued", "Running", "N% complete"). A trigger that joins
  a run already in flight (`reused`) gets an info toast, and a refused
  trigger gets an error toast.
- When a run it saw in flight settles, it invalidates `scan-folders`,
  `libraries` (which includes the filter pick-lists) and the whole
  `['subsonic']` tree: a scan changes the track counts on this page and the
  catalog every music view shows. The run its own trigger returned counts
  too, since a small scan can finish before any poll sees it running. A run
  that ends in anything other than `complete` or `canceled` gets an error
  toast pointing at the Tasks queue. A run that had already settled when the
  page loaded changes nothing.
- It is disabled only when the list loaded empty (there is nothing to walk).
  While loading, or after a failed fetch, it stays usable.

Both panels distinguish a failed request from a genuinely empty one: the
empty-state copy above renders only once the query actually resolved to an
empty list (`scanFolders && scanFolders.length === 0` /
`libraries && libraries.length === 0`) — on `isError` a red `.error-state`
block ("Could not load the scan folders"/"Could not load the libraries…
Check that the server is reachable and reload the page.") takes that slot
instead, so a failed fetch is never reported as "nothing is configured" or
"no libraries yet". `LibrariesPanel`'s "Add library" and `ScanFoldersPanel`'s
"Scan now" stay available in that error state; both live in the section
header, outside the loading/error/empty/table switch.

**`LibraryDialog`'s view settings** are three separate controls, all over
`lib/libraryViews.ts` (the single list of views, labels, icons and display
order — the sidebar's root Library block shows the same three in the same
order): a **Views** checkbox group (at least one: the last ticked box is
disabled), **Opens on** (a `Select` offering only the ticked views; unticking
the view it opened on moves it to the first view left; disabled with one
view), and **Main Artists page — Hide this library's artists**
(`hide_from_artist_index`). The last is deliberately independent of the
Artists view: it only keeps the artists off the cross-library index, and a
library can list its artists in its own Artists view while hiding them from
the main page. A fourth, **Sidebar** (`SidebarLayoutPicker`: two radio
cards, One entry / Entry per view, each holding a themed miniature of the app —
grey bars for text, the view switch drawn in the page header or as sidebar
lines; `split_views`), picks how the library appears in `AppSidebar`: one
entry in the Library block whose page switches views, or a section of its
own below it — a `.nav-section-label` header with the library's name, then an
entry per offered view in display order (the view it opens on at the bare
`/library/:id`, the others at `/library/:id/:mode`; each active only on its
own view). Collapsed, the header drops, the section keeps its spacer and the
tooltips read "Library · View". The root has the same choice: the **Main
library** block (`MainLibraryPanel`, between the scan folders and the
libraries, the same picker, saving on pick through `useUpdateCatalogSettings`
→ `PUT /api/v0/libraries/catalog`). The sidebar and `LibraryView` read it from
`getMusicFolders`' `catalog` descriptor via `useCatalogView`, which shares
`useMusicFolders`' query (one request, each `select`ing its part); a missing
descriptor means split. Split, the Library block starts with Discover /
Artists / Albums; not split, with one **All music** entry (`/library`,
compass icon, the Library shortcut badge, active on any root view). New libraries default to every view, opening on Discover —
the server's defaults too. `/views…` errors render on the Views row,
`/default_view` under Opens on.

**`LibraryView`'s views** follow the library: `offeredViews` is every view at
the root and `folder.views` inside a library (the `musicFolderViews`
extension); the path's `:mode` shows when offered, else the library's
default (`openingView`). A mode the library does not offer is rewritten to
the bare `/library/:folderId` path (query kept) once the folders are loaded —
before that, nothing is rendered or fetched, since every view looks
unoffered. The view switcher — a `SelectButton` (`.library-view-switch`,
the header's `as-button-group` look, first in `#actions` before the filters)
— renders wherever more than one view is offered and the sidebar is not
already switching them: always on the mobile shell (no sidebar), and on the
desktop only where the root or the library has a single sidebar entry (a
split root or `splitViews` library is switched by the sidebar); on phones
its labels are visually hidden (kept for screen readers) and the header's
actions may wrap, since the switcher plus the release-type row overflow 390px
otherwise. Choosing a view pushes its canonical path: the landing view is the
bare path, every other
view a segment. A library's Discover passes `folderId` to `DiscoveryFeed` /
`useDiscoveryFeed`, so it is that library's own feed.

**`LibraryFilterBuilder`** (`components/admin/LibraryFilterBuilder.vue`,
inside `LibraryDialog`) is the one place filters are edited. `LibraryDialog`
always sends `filters` on both create and update — never omits the key: on
`PUT /libraries/{id}` an absent `filters` key means "keep what's stored" and
`[]` means "clear them", so round-tripping exactly what the builder shows is
what makes an update mean what it looks like. Rules worth knowing about the
builder itself before touching it:

- A row's position IS its identity: the server's `422` pointers
  (`/filters/i/field`, `/filters/i/values`, `/filters/i/values/j`) index the
  filters array as it was last **sent**, so `lib/libraryFilters.ts`'s
  `rowErrors()` maps purely by array index. `LibraryDialog` hides every
  `/filters…` error the moment the admin edits the filters after a failed
  save (add/remove/change a row) — they return only with the next failed
  submit — because a stale error would otherwise attach to the wrong row;
  field-level errors (`/name`, `/hide_from_artist_index`, …) are unaffected.
- The app's own code never trims, lowercases or otherwise normalizes a
  value: `genre` / `release_type` / `path` are matched verbatim against
  scanned data by the server, so a client-side rewrite could turn a matching
  value into one that matches nothing. `""` for `release_type` is a real
  selector ("(none)"), not a blank. One thing is *not* verbatim, and it is
  PrimeVue's doing rather than ours: the `path` row's `AutoComplete` chips
  input commits `event.target.value.trim()` on Enter, so a **typed** path is
  trimmed — a directory whose real name is padded can only be added with
  "Browse…", which appends the server's own path unchanged (the server then
  `filepath.Clean`s a path value and rejects a relative one). Every other
  field's values are picked from a list, never typed, so nothing there is
  retyped or trimmed. `LibraryFilterBuilder.spec.ts` drives the real
  `AutoComplete` for that one case, so a PrimeVue upgrade that changes it
  fails the suite rather than silently contradicting this paragraph.
- A row's value options come from `useLibraryFilterOptions()`
  (`GET /libraries/filter-options`) plus, via `optionsFor()`, every value
  the row already holds that the server no longer offers — flagged
  `missing` and labeled with why (`(not configured)` for a `scan_folder`
  value, `(not in the catalog)` for everything else) — so a
  stored-but-stale value (a renamed scan folder, a retagged genre) stays
  visible and removable instead of silently vanishing or silently
  surviving the next save.
- **"Not loaded" is not "not offered".** While that query is still in
  flight — which is every first open of the dialog on a page load — or if
  it failed outright, `optionsFor()` gets `undefined` and returns the row's
  own values as plain options, never `missing`: otherwise every stored
  value would read as gone, and an admin who believed the label would
  delete a perfectly good one. On `isError` the builder says so once, above
  the rows ("Could not load the values to pick from. Stored values are
  shown as they are; reload the page to try again."), rather than leaving
  empty pick-lists that look like an empty catalog.
- A `missing` `scan_folder` value is worse than cosmetic, and only once
  the options have loaded does the builder know: the dialog always sends
  `filters`, so that value is re-sent on every Save and refused with a
  `422` at `/filters/i/values/j` — the library cannot be saved at all until
  it is removed. The row says so, per value, instead of letting the first
  Save be the messenger.
- A 400ms-debounced call to `previewLibrary()` renders "Matches N tracks in
  M albums" (`countLabel` — singular for exactly one) (or "Fix the
  filters…" on a 422); every external change to
  `modelValue` restarts the debounce, and a superseded response is dropped
  by identity-checking the in-flight `AbortController`, not just waiting
  for its promise to settle. The very first preview is not debounced at
  all: `onMounted` fires `runPreview()` immediately, separately from that
  400ms timer, so opening the dialog on an existing library shows its
  match count right away instead of after a blank 400ms.

The `path` filter's "Browse…" opens `FolderPickerDialog`, rooted at the
configured scan folders: calling `browseFolders()` with **no** `path`
answers their roots, and nothing outside them is reachable — it is a
directory picker for one filter value, not a library-root picker. Selecting
a folder at or below a symlink shows a `severity="warn"` `Message` ("tracks
are recorded under the real location the link points to") because the
scanner indexes a symlinked tree under its resolved path, so a `path` filter
on the link itself would match nothing; the ancestry is tracked per-node
(`data.symlinked` on each `TreeNode`) so a descendant loaded later still
knows a parent was a link.

**Nested dialogs and Escape.** PrimeVue's `Dialog` binds a document-level
`keydown` listener per visible dialog and none checks which dialog is on
top, so one Escape closes every open dialog. `LibraryFilterBuilder`
therefore emits `update:browsing` while its folder picker is open, `IconSelect`
emits `update:open` while its icon `Popover` is, and `LibraryDialog` passes
`:closeOnEscape="!pickerOpen && !iconPickerOpen"` — `closeOnEscape` is
read at event time, so this is independent of where focus sits. The listener
itself is bound once, when the dialog enters, and only if `closeOnEscape` is
true at that moment (`bindGlobalListeners`) — so the flag must be reset BEFORE
the dialog opens, as `LibraryDialog`'s form-reset watch does. Any dialog or
`Popover` opened from inside a dialog needs the same treatment (`IconSelect` in
`LibraryDialog` is the second case: PrimeVue's `Popover` hides on Escape
without stopping propagation, and binds its own document listener besides).
A `Popover` needs one thing more than a nested `Dialog`: PrimeVue emits its
`hide` at the START of the leave, and a trusted key press lets microtasks run
between the popover's own keydown handler and the document-level listeners — so
the host must clear its "overlay open" flag in a macrotask (`setTimeout(…, 0)`),
or the same Escape reaches the Dialog with `closeOnEscape` already back to
`true`. jsdom cannot show this with one synthetic `dispatchEvent` (no microtask
checkpoint between listeners): the spec emulates the trusted ordering in three
steps, and the behaviour was verified with a real key press.
A spec for it must mount
with `transition: false`: Vue Test Utils' default transition stub never
fires the `@enter` hook in which PrimeVue binds that listener, so the
default harness cannot see the bug.

**A focused `Select` swallows Escape.** PrimeVue 4.5.5's `Select.onEscapeKey`
calls `event.stopPropagation()` even when its overlay is closed (their source
marks it `@todo`), so while a `Select` holds focus Escape never reaches any
`Dialog`'s document-level listener — in every dialog of the app, not only this
one. (`MultiSelect` stops propagation only while its overlay is open.) Seen in
a real browser; deliberately not worked around. Re-check on a PrimeVue upgrade.

`useScanFolders()` (`composables/useScanFolders.ts`) is shared by
`ScanFoldersPanel`, the filter builder and the metadata editor (see the
`composables/` entry above). In the builder it feeds exactly one thing: the
"not usable right now" note on a `scan_folder` value naming a folder that IS
configured but reports `available: false` (an unmounted root, say). It does
**not** feed the `MultiSelect`'s options — those come from
`useLibraryFilterOptions().scan_folders` via `optionsFor()` — and it says
nothing about a *dangling* value, one naming a folder that is not configured
at all: that one gets the `(not configured)` label from `optionsFor()` plus
the note that it blocks saving (above).

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

The speaker has three states — loud / quiet / silent: `ms-volume-up`,
`ms-volume-down`, and `ms-volume-off` (Material's slashed speaker, so silence
needs no drawn strike).

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
Playing is `/`, where `HomeView` renders `QueueView` on desktop and on mobile
redirects to `/browse` with `NowPlayingSheet` open — there is no separate
now-playing route), `D` → `library` with no `folderId` (the cross-collection root
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

Sidebar anchors come from a per-item `shortcut` field on `AppSidebar`'s `NavItem`
(not a `NAV_SHORTCUT_ANCHORS` routeName map): every nav loop binds the same
`:data-shortcut="item.shortcut"` attribute, so which entries anchor is entirely
data-driven — every entry in `topItems` and `bottomItems` sets one, as does the
Discover entry in `libraryModes`, six in total. The three library modes
(Discover/Artists/Albums) share `routeName: 'library'`, but only Discover's
entry sets `shortcut`; Albums/Artists and every `folderItems` entry leave
it unset, since anchoring more than one of them would let the overlay badge
whichever it found first instead of the cross-collection root. That is why
`AppSidebar.shortcutAnchor.spec.ts` asserts a bare *count* of anchored entries — a
count is what catches an anchor leaking onto the folder rows or the other three
modes.

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
Error text uses `--app-danger` (defined per theme in
`assets/scss/_variables.scss`): PrimeVue's `--p-red-*` do not flip with the
theme and no single red passes AA on both grounds.

**Hidden themes (easter egg).** `_hidden-themes.scss` holds two unlockable
palettes — Winamp, CRT. They are token-only repaints layered over
`.dark-mode` (`useTheme` keeps that class on for them so the PrimeVue preset
stays dark) and are selected by a `theme-<name>` root class. Five clicks on the
wordmark's "e" in `AppSidebar` unlock them for good (`aether:hiddenThemes` in
localStorage) and cycle between them; once unlocked they appear in the
User settings (`/user-settings`) theme picker. This is intentional — don't "clean it up".

## Icons

Material Symbols **Rounded**, self-hosted — never a CDN. `build/material-symbols`
(a Vite plugin) reads `@iconify-json/material-symbols` from node_modules:

- **Static icons are classes**: `ms-<name>` is the **filled** glyph — the app's
  icon style — and `mso-<name>` the outlined one, used only for the "off" half
  of a toggle whose "on" half is the filled glyph (`mso-favorite`/`ms-favorite`
  hearts, `mso-circle`/`ms-circle` selection). Names are kebab-case Material
  names (`ms-play-arrow`). They are plain
  class strings, so PrimeVue `icon=` props and menu models take them as-is. The
  plugin scans `src/**/*.{vue,ts}` (specs excluded) and emits CSS-mask rules for
  exactly the tokens it finds; an unknown name, or a class built at runtime
  (`` `ms-${x}` ``), **fails the build** — write the literal class.
- Icons are `1.2em` boxes following `font-size`/`color`, tuned with
  `--app-icon-size` / `--app-icon-align`. Spin with `icon-spin`; the sheet also
  styles `pi-spin` because PrimeVue hard-codes it on its own loading icons.
- **Library icons** are runtime data: a snake_case Material name
  (`queue_music`, the `musicFolderIcon` vocabulary). Render them with
  `<LibraryIcon :name>`, which masks with the per-icon (filled) SVG the build
  emits to `dist/icons/ms/<name>.svg`; a well-formed name that is not a real Material
  Symbols icon renders blank, only malformed names fall back to `folder`.
- **The picker** (`IconSelect`) lazily imports `virtual:material-symbols/catalogue`
  — every Rounded icon name, sorted — and searches by name with
  `lib/iconSearch.ts` (exact, prefix, word start, substring). An empty query
  shows `SUGGESTED_LIBRARY_ICONS` (`lib/libraryIcons.ts`), a hand-picked
  music-flavoured list checked against the catalogue by a test.

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

The same `make icons` run renders the desktop player's app icon from
`web/icon.svg` into `player/src-tauri/icons/` — see
[player/releasing.md](player/releasing.md#cautions).

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
