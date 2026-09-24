# Aether

Music server written in Go with a Vue 3 frontend.

## Agent docs — read before implementing

Before implementation work, read [`docs/agents/architecture.md`](docs/agents/architecture.md)
and the subsystem doc for the area you're changing:
[`subsonic-api.md`](docs/agents/subsonic-api.md) (anything under `/rest`),
[`api-conventions.md`](docs/agents/api-conventions.md) (anything under `/api/v0`),
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
- **All music functionality is powered exclusively by `/rest/`** — browsing, playback, playlists, album/artist navigation, search, etc. When the standard lacks something the music UI needs, do not bolt it onto `/api/v0`; instead add it as a proper **OpenSubsonic extension**: implement the endpoint under `/rest/` and advertise it via `getOpenSubsonicExtensions` so non-supporting clients ignore it and nothing breaks. Prefer upstreaming the extension to the OpenSubsonic registry so other clients can consume it too.
- **The internal `/api/v0` API is for server-management actions only** — managing libraries (saved filters) and listing the scan folders, backups, scanning, tasks/scheduling, and other admin concerns that have no place in a music-client API. Never route music browsing/playback features through `/api/v0`.

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
| `MobileBrowseView` | `/browse` | **Mobile only** — the landing page and the whole navigation surface of the mobile shell (the phone's `AppSidebar`, and where every view's hamburger goes; passes `navRoot` so it shows none itself). One `BrowseShelf` per section — Library (samples the Discovery feed), one per dynamic library (`BrowseAlbumShelf`, newest albums, from the first library on), Playlists, Genres, Radio — each with a heading, a few items in a swipeable strip and a "See all" link. Search and the `UserMenu` account entries (User settings / Admin / About / Log out — the phone's only logout) sit in the header, the latter behind `⋮`. Redirects to `/library` at desktop width. Replaced the deleted `MobileNavDrawer` |
| `SearchView` | `/search` | Search results |
| `LibraryView` | `/library/:mode?`, `/library/:folderId/:mode?` | Browse the catalog — whole, or one library at a time (`:folderId` is the library's id, Subsonic's `musicFolderId`; not a scan folder). Its views — Discover / Artists / Albums — are real path segments, not a URL hash. The root offers all three (bare `/library` is Discover, then `/library/artists` and `/library/albums`; there is no `/library/discover`), switched from the sidebar's Library block on desktop. Each library is either one sidebar entry in that block or — with `split_views` (`splitViews`, `musicFolderViews` v2) — a sidebar section of its own: a header with its name and an entry per view it offers. The root makes the same choice (the Main library setting, `getMusicFolders`' `catalog` descriptor): its three views as entries, or one "All music" entry. A library offers only the views its admin ticked (`views` + `default_view`, the `musicFolderViews` extension): bare `/library/:folderId` opens its default view, `/library/:folderId/:mode` names another, and a view it does not offer is rewritten back to the bare path. A view switcher — a Discover / Artists / Albums `SelectButton` button group in the header's `#actions`, first before the filters, the same `as-button-group` look as the release-type row; icons only on phones — switches views wherever the sidebar does not: on the phone shell, and on desktop at a single-entry root or inside a single-entry library offering more than one. Choosing a view pushes its path. Discover is a ranked feed of albums + playlists (`getDiscovery` extension, infinite scroll, `DiscoveryFeed` component) — the whole catalog's at the root, the library's own inside one (`musicFolderId`: albums and playlists with a track in the library); there is deliberately no standalone `/discover` route. Albums carries a release-type tab row (All + MusicBrainz primary types: Album/Single/EP/Broadcast/Other) via the `releaseTypeFilter` extension, hidden while the favorites filter is on. It offers only the types the library's albums actually carry (`getReleaseTypes`, `releaseTypeFilter` v2; every type until that answers, and the active `?releaseType=` is always kept) and is dropped entirely when there is none |
| `AlbumView` | `/album/:id` | Single album detail. Hero cover is editable (`updateAlbum` / `albumCoverArt` extension) — the only place a manual album cover is set, since the metadata editor is file-only |
| `ArtistView` | `/artist/:id` | Single artist detail. The discography is grouped into ordered release-type sections (Albums / EPs / Singles / [Broadcast] / Other) via `lib/releaseGrouping.ts`: primary MusicBrainz type per album, untyped→Albums, and non-Album/EP types with fewer than 4 releases folded into Other |
| `PlaylistsView` | `/playlists` | List of playlists |
| `PlaylistDetailView` | `/playlist/:id` | Single playlist detail |
| `GenresView` | `/genres` | Browse by genre |
| `GenreDetailView` | `/genre/:name` | Single genre detail (hero + paged song list) |
| `RadioView` | `/radio` | List of radio stations |
| `RadioStationDetailView` | `/radio/:id`, `/radio/new` | Station detail + create form (same component, `create` prop) |
| `UserSettingsView` | `/user-settings/:tab?` | Personal settings (identity, theme, change password, API tokens). Vertical tablist whose active section is the `tab` path segment (`general` \| `account` \| `access`), so a reload or shared link reopens it; bare `/user-settings` is General, an unknown/unavailable section rewrites back to it. `account` holds the change-password form and appears only in native mode when signed in; `access` needs a signed-in user. Reached from the sidebar's `UserMenu` popup, not from `/settings` |
| `AboutView` | `/about` | About Aether (keyboard shortcuts reference, build info, source link). Reached from the `UserMenu` popup |

**Settings views** — nested under `/settings` with `meta: { layout: 'settings' }` (not the music layout); `/settings` redirects to `/settings/libraries`. Settings is administration only: account concerns (user settings, logout) live in the sidebar's `UserMenu` popup. The whole area is **admin-only**: `useAuth().isAdmin` hides the `UserMenu` Admin entry and `App.vue` redirects non-admins landing on a settings route; the backend enforces it with 403 on `/api/v0` (see `docs/agents/authentication.md`):

| View | Route | Purpose |
|------|-------|---------|
| `SettingsView` | `/settings` | Settings shell (renders children) |
| `LibrariesView` | `/settings/libraries` | Manage libraries — named, **filtered views** over the catalog, not directories (`LibrariesPanel` + `LibraryDialog`: name, icon, the views it offers and the one it opens on, whether the sidebar gives it one entry or one per view, whether its artists are hidden from the main Artists page, and AND-ed filters built in `LibraryFilterBuilder` with a live match count; deleting one never touches tracks) — below a read-only `ScanFoldersPanel` listing the directories declared under `ScanFolders:` in the config file, the only place they can be changed; its "Scan now" button is a shortcut to the Catalog Scan task. Between the two sits `MainLibraryPanel`: how the sidebar lists the root (whole catalog) — its three views, or one entry (`PUT /api/v0/libraries/catalog`) |
| `UsersView` | `/settings/users` | Manage native users (only with `Auth.Method: native`; nav entry hidden otherwise) |
| `TasksView` | `/settings/tasks` | Scanning / scheduled tasks |
| `MetadataEditorView` | `/metadata-editor` | Metadata editing (composes `EditPanel`, `FolderTree`, `TrackList` sub-components). Top-level route rendered in the settings layout (admin-only); reached from the sidebar `UserMenu` and the settings side-nav (Tools group) |
