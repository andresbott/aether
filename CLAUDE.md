# Aether

Music server written in Go with a Vue 3 frontend.

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
