# Aether

Music server written in Go with a Vue 3 frontend.

## Agent docs — read before implementing

Before implementation work, read [`docs/agents/architecture.md`](docs/agents/architecture.md)
and the subsystem doc for the area you're changing:
[`subsonic-api.md`](docs/agents/subsonic-api.md) (anything under `/rest`),
[`scanning.md`](docs/agents/scanning.md) (scanner/tags/store reconcile),
[`frontend.md`](docs/agents/frontend.md) (webui). Check
[`docs/agents/features.md`](docs/agents/features.md) before adding a
capability — gaps are catalogued with chosen directions — and
[`docs/agents/testing.md`](docs/agents/testing.md) for the verification gates.

## Project State

- **No backwards compatibility — until further notice.** No users, no live deployment. Do not write migration code, schema bridges, config compat layers, or "if old shape, fall back to..." branches. When the schema or config shape changes, just change it; the user will drop the DB manually if needed. Structure can change freely.
- SQLite + GORM for persistence, gorilla/mux for routing, PrimeVue for UI.
- Single binary deployment with embedded SPA.

## API Compatibility

- **The `/rest/` API must stay compliant with the [OpenSubsonic](https://opensubsonic.netlify.app/) standard** so third-party Subsonic/OpenSubsonic clients can consume this server. Do not add ad-hoc non-standard endpoints, parameters, or response fields to `/rest/`.
- **All music functionality is powered exclusively by `/rest/`** — browsing, playback, playlists, album/artist navigation, search, etc. When the standard lacks something the music UI needs, do not bolt it onto `/api/v1`; instead add it as a proper **OpenSubsonic extension**: implement the endpoint under `/rest/` and advertise it via `getOpenSubsonicExtensions` so non-supporting clients ignore it and nothing breaks. Prefer upstreaming the extension to the OpenSubsonic registry so other clients can consume it too.
- **The internal `/api/v1` API is for server-management actions only** — managing collections/libraries, backups, scanning, tasks/scheduling, and other admin concerns that have no place in a music-client API. Never route music browsing/playback features through `/api/v1`.

## Frontend Conventions

- **Main content views must follow the uniform layout in [`docs/architecture/main-content-view-layout.md`](docs/architecture/main-content-view-layout.md).** Any top-level route view rendered into `PlayerLayout` uses the `ContentScaffold` header (title + count summary + `#actions`), a self-scrolling body centered on the `--app-content-max-width` column with a flush-right scrollbar, and `meta: { flush: true }` on its route. Read that doc before adding or refactoring a main content view. Now Playing (`QueueView`), Library, and Radio are the reference implementations.

### Views

Routes are defined in `webui/src/router/index.ts`. There are two categories,
plus one non-route view: `LoginView` is rendered by `App.vue` in place of the
whole app whenever auth method `native` reports no session (see
`docs/agents/authentication.md`) — deliberately not a route, so there is no
`/login` URL and no redirect dance.

**Main content views** — top-level routes rendered into `PlayerLayout` with `meta: { flush: true }`, using the `ContentScaffold` header (see layout doc above):

| View | Route | Purpose |
|------|-------|---------|
| `HomeView` | `/` | Now Playing. Desktop renders `QueueView variant="full"`; on the mobile shell `/` is only an alias — it replaces itself with `/browse#playing` (queue filled) or `/browse` (empty). The phone's Now Playing is `NowPlayingSheet`, an always-mounted bottom sheet addressed by the `#playing`/`#queue` hash on the current route (see `docs/agents/frontend.md`) |
| `MobileBrowseView` | `/browse` | **Mobile only** — the landing page and the whole navigation surface of the mobile shell (the phone's `AppSidebar`, and where every view's hamburger goes; passes `navRoot` so it shows none itself). One `BrowseShelf` per section — Library (samples the Discovery feed), one per dynamic library (`BrowseAlbumShelf`, newest albums, only above one library), Playlists, Genres, Radio — each with a heading, a few items in a swipeable strip and a "See all" link. Search and the `UserMenu` account entries (User settings / Admin / About / Log out — the phone's only logout) sit in the header, the latter behind `⋮`. Redirects to `/library` at desktop width. Replaced the deleted `MobileNavDrawer` |
| `SearchView` | `/search` | Search results |
| `LibraryView` | `/library/:folderId?` | Browse the music library by folder. Tabs: Discover / Albums / Artists. Discover is a ranked feed of albums + playlists (`getDiscovery` extension, infinite scroll, `DiscoveryFeed` component) and is the default tab on the root `/library` only — the ranking is cross-collection, so discovery is never library-scoped. There is deliberately no standalone `/discover` route |
| `AlbumView` | `/album/:id` | Single album detail |
| `ArtistView` | `/artist/:id` | Single artist detail |
| `PlaylistsView` | `/playlists` | List of playlists |
| `PlaylistDetailView` | `/playlist/:id` | Single playlist detail |
| `GenresView` | `/genres` | Browse by genre |
| `GenreDetailView` | `/genre/:name` | Single genre detail (hero + paged song list) |
| `RadioView` | `/radio` | List of radio stations |
| `RadioStationDetailView` | `/radio/:id`, `/radio/new` | Station detail + create form (same component, `create` prop) |
| `UserSettingsView` | `/user-settings/:tab?` | Personal settings (identity, theme, API tokens). Vertical tablist whose active section is the `tab` path segment (`general` \| `access`), so a reload or shared link reopens it; bare `/user-settings` is General, an unknown/unavailable section rewrites back to it. Reached from the sidebar's `UserMenu` popup, not from `/settings` |
| `AboutView` | `/about` | About Aether (keyboard shortcuts reference, build info, source link). Reached from the `UserMenu` popup |

**Settings views** — nested under `/settings` with `meta: { layout: 'settings' }` (not the music layout); `/settings` redirects to `/settings/libraries`. Settings is administration only: account concerns (user settings, logout) live in the sidebar's `UserMenu` popup. The whole area is **admin-only**: `useAuth().isAdmin` hides the `UserMenu` Admin entry and `App.vue` redirects non-admins landing on a settings route; the backend enforces it with 403 on `/api/v1` (see `docs/agents/authentication.md`):

| View | Route | Purpose |
|------|-------|---------|
| `SettingsView` | `/settings` | Settings shell (renders children) |
| `LibrariesView` | `/settings/libraries` | Manage collections/libraries |
| `UsersView` | `/settings/users` | Manage native users (only with `Auth.Method: native`; nav entry hidden otherwise) |
| `TasksView` | `/settings/tasks` | Scanning / scheduled tasks |
| `MetadataEditorView` | `/settings/metadata` | Metadata editing (composes `EditPanel`, `FolderTree`, `TrackList` sub-components) |
