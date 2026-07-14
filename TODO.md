# TODO

## Security

- [ ] Path traversal validation in `stream` and `getCoverArt` — validate that resolved file paths are within configured music directories before serving via `http.ServeFile`
- [ ] Authentication — implement real user auth with Subsonic token validation
- [ ] Gate radio station CRUD (create/update/delete) behind admin role once user management is implemented — reads stay open to all users, writes require admin
- [ ] Subsonic external-client auth (deferred) — third-party Subsonic apps (Symfonium, DSub, etc.) authenticate with the Subsonic protocol's own query-string credentials, separate from the web UI's session login. Plan to implement via per-user **Personal Access Tokens (PATs)**, added as a generic CRUD feature in the `userauth` library (aether wires routing, authorization, and the management UI).
    - **Default: Recoverable PAT (encrypted-at-rest).** Subsonic token auth sends `t=md5(secret+salt)` with a fresh, client-chosen `salt` per request, so the server must hold the raw secret to recompute the hash. Hash-only/bcrypt storage cannot satisfy this, and MD5 is fixed by the protocol (no algorithm negotiation). Store the PAT symmetric-encrypted with a server key (not plaintext); optionally keep a `sha256` index alongside the ciphertext for fast lookup. Blast radius stays small — a PAT is a scoped, named, individually-revocable random value, never the user's bcrypt login password.
    - **Optional non-recoverable (hash-only) mode, per token/client.** Store only `sha256(token)` — more secure at rest, but only works with clients that transmit the raw token: OpenSubsonic `apiKey=<token>` or legacy `p=<token>`. It will NOT work with default token-auth (`t`+`s`) clients. `apiKey` is a recent (2024) fail-closed extension with no fallback, so an apiKey-only server locks out most current clients; treat hash-only as a per-token opt-in for clients known to support `apiKey`/`p=`.
    - aether's own Vue SPA does NOT need any of this — it is same-origin and authenticates `/rest/*` via the session cookie. Only foreign apps need a PAT.
    - Wiring: a thin Subsonic `AuthHandler` parses `u`/`t`/`s`/`p`/`apiKey` and validates against the PAT verifier; chain it through the userauth `authenticator` (OR semantics) alongside session-cookie auth and (later) header auth on the `/rest/*` routes.

## API Surface

- [ ] Review the non-OpenSubsonic API surface — audit custom (non-Subsonic) endpoints, then move Libraries and Tasks management under an `/admin` path (e.g. `/api/admin/libraries`, `/api/admin/tasks`) so admin concerns are clearly separated from the Subsonic-compatible surface; update the frontend accordingly

## API Completeness

- [ ] XML response format — check compatibility with third-party Subsonic clients (DSub, Ultrasonic, Symfonium, etc.)
- [ ] Transcoding — identify formats browsers can't play natively and add FFmpeg transcoding
- [ ] setRating persistence — add rating column to tracks/albums when needed
- [ ] `star` / `unstar` Subsonic endpoints — accept `id` query params that can be album, artist, or track IDs; split by entity type, then upsert/delete a star record (user + item + `starred_at` timestamp) per entity
- [ ] `getStarred` — folder-based starred list; JOIN each entity table with its star junction table filtered by `user_id`, return nested artist/album/track response
- [ ] `getStarred2` — tag-based starred list; same JOIN pattern ordered by `starred_at DESC`, return flattened artist/album/track lists
- [ ] getArtistInfo / getAlbumInfo — external metadata (MusicBrainz bios, similar artists)
- [ ] getTopSongs / getSimilarSongs — requires external data or play history analysis
- [ ] Podcasts, Radio, Bookmarks, Sharing, Chat, Jukebox — not in scope for this pass

## Frontend — Wire SPA to Subsonic API

### Phase 1: Make existing views work
- [ ] Auto-connect without credentials (backend has no auth)
- [ ] Library view — albums and artists fetch and display correctly
- [ ] Album view — shows cover, metadata, track list; play/queue works
- [ ] Artist view — shows artist info and discography
- [ ] Playlists view — lists playlists from API
- [ ] Player — streaming playback works end-to-end
- [ ] Home view — shows now playing or sensible empty state

### Phase 2: Fill in missing features
- [ ] Search UI — add search bar that uses the existing search composable
- [ ] Playlist create/edit — create new playlists, add/remove songs
- [ ] Song view — display track detail from queue
- [ ] Favorites/starring:
  - Star/unstar toggle on album detail, artist view, and track rows (album view and queue)
  - Starred indicator on album grid cards in library view
  - Starred library section — browse starred albums, artists, and tracks (backed by `getStarred2`)
- [ ] Songs tab in Library — fetch and display all songs
- [ ] Artists tab in Library — replace the grid-of-artist-cards + drill-down into a single scrollable page grouped by artist: one header per artist (alphabetical), followed by that artist's albums sorted by year; no per-artist navigation step

## Integration

- [ ] Last.fm scrobbling — explore Last.fm API integration for external scrobbling
- [ ] Tag editor ↔ MusicBrainz/DB sync — let the metadata/tag editor pull MusicBrainz data into the DB and write tags back, in particular populate/correct the `MusicBrainz Artist Id` that drives artist-image fetching (see `docs/superpowers/specs/2026-06-30-durable-artist-image-store-design.md`)
- [ ] Multi-user — single-user for now; playlist owner column is pre-wired
- [ ] DLNA / UPnP endpoint — expose the library as a DLNA MediaServer so devices on the LAN (TVs, receivers, stock media players) can browse and stream without the Subsonic client

## Performance

- [ ] `getPlaylists` N+1 queries — each playlist triggers separate count and duration queries; consider a single annotated query
- [ ] `albumToMap` missing `songCount`/`duration` when tracks are not preloaded — album list endpoints don't preload tracks, so these fields are absent in list responses
- [ ] cover art is extacted from the file on the fly, it might perform better if we extract at scannnig 

## Data Integrity

- [ ] Favorites schema — three junction tables (`album_stars`, `artist_stars`, `track_stars`), each with composite PK `(user_id, item_id)`, a `starred_at` timestamp, and cascade deletes on user/item removal; replace the current single `starred_items` table if one exists
- [ ] Cleanup orphaned `playlist_tracks`, `album_stars`, `artist_stars`, `track_stars`, and `play_histories` when tracks/albums/artists are deleted during scan cleanup
- [ ] Use `errors.Is(err, gorm.ErrRecordNotFound)` in `FindOrCreateArtists` and `FindOrCreateAlbum` to distinguish not-found from real DB errors
- [ ] Full scan should drop each track's existing entries and re-insert from scratch (rather than updating in place) so stale/renamed artists, albums, genres, and other derived rows don't linger when tags change
- [ ] Album cover on the library grid can show another album's image after metadata edits + rescan (works correctly on the album detail view). Root causes:
    - `internal/scanner/reconcile.go:92-97` only sets `album.CoverPath` when empty; it's never re-evaluated. Re-tagged tracks leaving their old album don't repoint or clear the stale path, and two albums sharing a directory can end up pointing at the same `cover.jpg`.
    - Embedded-cover lookup `internal/store/track.go:113` (`GetCoverTrackPath`) returns the *first* track with `has_embedded_cover=true`, with no ordering — which track wins is unstable across rescans.
    - `DeleteOrphanedAggregates` doesn't revalidate `CoverPath` for surviving albums.
    - No `Cache-Control`/ETag on `getCoverArt` responses; browser keeps serving the stale body until mtime or URL changes.
  Fix options: clear `album.CoverPath` at the start of each reconcile pass and redetect; or drop `CoverPath` entirely and resolve per-request from a current track's directory. Pick a deterministic embedded-cover track (e.g. lowest `(disc, track)`). Add a stable ETag (album `updated_at`) so edits immediately invalidate client caches.

## Frontend Layout — Pre-existing items surfaced during topbar refactor

Surfaced by reviews of the topbar refactor (2026-04-26) but pre-existing — not introduced by that work. To review:

- [ ] `PlayerControls` is `position: fixed` (not a flex row inside `App.vue`'s shell), so the bottom of `.main-content` sits *under* the player. Any content scrolled to the bottom is partially obscured. Fix candidates: give `.app-container` a `padding-bottom: var(--app-player-height)`, or restructure `PlayerControls` into the column flex
- [ ] `.nav-item.active` in `AppSidebar.vue` uses a hard-coded `#eef2ff` background instead of a `var(--app-*)` token — odd one out vs. the rest of the file
- [ ] `.view-placeholder` styles in `App.vue` look orphaned (the class isn't used in `App.vue`'s template) — verify whether any view actually consumes them, then either remove or move into the consuming view
- [ ] Clicking a row in the expanded `QueueSidebar` currently navigates to the song detail view — it should instead play that song (jump the queue to that track and start playback) and stay on the current page. The song-detail view should only be reached from the "Now Playing" surface, not by clicking queue entries
- [ ] Make songs in the `QueueSidebar` sortable via drag and drop — reorder tracks within the current queue by dragging
- [ ] Clicking a track row in the album view should not start playback — only the explicit play control on the row should play the song
- [ ] Album view should support multi-disc albums — group tracks by disc number (CD1, CD2, etc.) with per-disc headers
- [ ] Dark mode support — add a theme toggle and dark color tokens; audit hard-coded colors (e.g. `#eef2ff`, `#e0e7ff`) and migrate them to CSS variables
- [ ] Improve execution history and runtime task management — surface long-running jobs (scans, imports, transcodes) with progress, status, and cancel controls; persist recent run history for inspection
- [ ] Mute support in the player — clicking the volume icon toggles mute (preserving the previous volume level to restore on unmute)
- [ ] Keyboard shortcuts — play/pause (space), next/previous track, volume up/down, mute, seek, toggle queue sidebar; add a help overlay listing them
- [ ] "Now Playing" view should combine the current track detail (cover, metadata, lyrics, etc.) with a list view of the full queue below it — single landing page instead of separate song-detail + queue-sidebar split
- [ ] Rework `RadioView.vue` to match the album grid style used in `LibraryView.vue` — reuse the `AlbumCard` layout (square cover tile, title/subtitle below, hover lift) for radio stations so the library and radio views feel consistent
- [ ] Create an app icon / logo — favicon, PWA icons (various sizes), and a wordmark for the topbar

## Frontend Layout — Library scaffold follow-up

- [ ] Unify the "Now Playing" / Queue view (`QueueView.vue`) onto the shared `LibraryScaffold` component introduced by the library-scaffold work (spec: `docs/superpowers/specs/2026-07-02-library-scaffold-and-artist-list-design.md`). That work extracted QueueView's fixed-header + flush-right-scroll pattern into a generic `LibraryScaffold` (fixed header with title/summary + `#actions` slot, `flex:1;min-height:0` body slot) and adopted it across the library album/artist × list/cover views. QueueView still carries its own bespoke copy of that layout (`.queue-view` / `.queue-view-header` / `.queue-body`). **Deferred from that effort** because QueueView had active uncommitted WIP and its own drag/sortable complexity, so refactoring it then would have entangled unrelated changes. Follow-up: have `QueueView` render `LibraryScaffold` (title/summary; `#actions` = its edit/save/clear buttons; body = the history/current/upcoming or edit list), deleting the duplicated layout CSS. Once it has this second consumer, consider moving `LibraryScaffold` out of `components/library/` to a neutral home (e.g. `components/layout/`) and renaming it accordingly.


juckebox funcionality ( use the wbe ui only to controll the audio)
relay => like juckebox, but loading songs from another instance