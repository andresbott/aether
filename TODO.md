# TODO

## Security & Authentication

- [ ] Path traversal validation in `stream` and `getCoverArt` — validate that resolved file paths are within configured music directories before serving via `http.ServeFile`
- [ ] Authentication — implement real user auth with Subsonic token validation
- [ ] Multi-user — single-user for now; playlist owner column is pre-wired
- [ ] Gate radio station CRUD (create/update/delete) behind admin role once user management is implemented — reads stay open to all users, writes require admin
- [ ] Subsonic external-client auth (deferred) — third-party Subsonic apps (Symfonium, DSub, etc.) authenticate with the Subsonic protocol's own query-string credentials, separate from the web UI's session login. Plan to implement via per-user **Personal Access Tokens (PATs)**, added as a generic CRUD feature in the `userauth` library (aether wires routing, authorization, and the management UI).
    - **Default: Recoverable PAT (encrypted-at-rest).** Subsonic token auth sends `t=md5(secret+salt)` with a fresh, client-chosen `salt` per request, so the server must hold the raw secret to recompute the hash. Hash-only/bcrypt storage cannot satisfy this, and MD5 is fixed by the protocol (no algorithm negotiation). Store the PAT symmetric-encrypted with a server key (not plaintext); optionally keep a `sha256` index alongside the ciphertext for fast lookup. Blast radius stays small — a PAT is a scoped, named, individually-revocable random value, never the user's bcrypt login password.
    - **Optional non-recoverable (hash-only) mode, per token/client.** Store only `sha256(token)` — more secure at rest, but only works with clients that transmit the raw token: OpenSubsonic `apiKey=<token>` or legacy `p=<token>`. It will NOT work with default token-auth (`t`+`s`) clients. `apiKey` is a recent (2024) fail-closed extension with no fallback, so an apiKey-only server locks out most current clients; treat hash-only as a per-token opt-in for clients known to support `apiKey`/`p=`.
    - aether's own Vue SPA does NOT need any of this — it is same-origin and authenticates `/rest/*` via the session cookie. Only foreign apps need a PAT.
    - Wiring: a thin Subsonic `AuthHandler` parses `u`/`t`/`s`/`p`/`apiKey` and validates against the PAT verifier; chain it through the userauth `authenticator` (OR semantics) alongside session-cookie auth and (later) header auth on the `/rest/*` routes.

## Backend — API Surface

- [ ] Review the non-OpenSubsonic API surface — audit custom (non-Subsonic) endpoints, then move Libraries and Tasks management under an `/admin` path (e.g. `/api/admin/libraries`, `/api/admin/tasks`) so admin concerns are clearly separated from the Subsonic-compatible surface; update the frontend accordingly

## Backend — OpenSubsonic Completeness

- [ ] XML response format — check compatibility with third-party Subsonic clients (DSub, Ultrasonic, Symfonium, etc.)
- [ ] Transcoding — identify formats browsers can't play natively and add FFmpeg transcoding
- [ ] setRating persistence — add rating column to tracks/albums when needed (handler exists in annotation.go but no rating column yet, so not persisted)
- [ ] `getStarred` — folder-based starred list; JOIN each entity table with its star junction table filtered by `user_id`, return nested artist/album/track response
- [ ] getArtistInfo / getAlbumInfo — external metadata (MusicBrainz bios, similar artists)
- [ ] getTopSongs / getSimilarSongs — requires external data or play history analysis
- [ ] Podcasts, Radio, Bookmarks, Sharing, Chat, Jukebox — not in scope for this pass

## Backend — Performance

- [ ] `getPlaylists` N+1 queries — each playlist triggers separate count and duration queries; consider a single annotated query
- [ ] `albumToMap` missing `songCount`/`duration` when tracks are not preloaded — album list endpoints don't preload tracks, so these fields are absent in list responses
- [ ] cover art is extacted from the file on the fly, it might perform better if we extract at scannnig

## Backend — Data Integrity & Scanning

- [ ] Favorites schema — three junction tables (`album_stars`, `artist_stars`, `track_stars`), each with composite PK `(user_id, item_id)`, a `starred_at` timestamp, and cascade deletes on user/item removal; replace the current single `starred_items` table if one exists
- [ ] Cleanup orphaned `playlist_tracks`, `album_stars`, `artist_stars`, `track_stars`, and `play_histories` when tracks/albums/artists are deleted during scan cleanup
- [ ] Use `errors.Is(err, gorm.ErrRecordNotFound)` in `FindOrCreateArtists` and `FindOrCreateAlbum` to distinguish not-found from real DB errors
- [ ] `store.GetArtist` doesn't reliably populate each album's `Artists` — it combines `Preload("Artists")` with a manual `Joins("JOIN album_artists ...")` on the same many-to-many, which can return empty `Artists` (GORM gotcha). Result: `getArtist`'s album children omit `artist`/`artistId`, leaving the artist page's album-card subtitle blank (worked around in `ArtistView.vue` by falling back to the artist name). Fix the query so `Artists` preloads cleanly — e.g. filter album IDs via a subquery instead of a manual JOIN, then Preload.
- [ ] Full scan should drop each track's existing entries and re-insert from scratch (rather than updating in place) so stale/renamed artists, albums, genres, and other derived rows don't linger when tags change
- [ ] Album cover on the library grid can show another album's image after metadata edits + rescan (works correctly on the album detail view). Root causes:
    - `internal/scanner/reconcile.go:92-97` only sets `album.CoverPath` when empty; it's never re-evaluated. Re-tagged tracks leaving their old album don't repoint or clear the stale path, and two albums sharing a directory can end up pointing at the same `cover.jpg`.
    - Embedded-cover lookup `internal/store/track.go:113` (`GetCoverTrackPath`) returns the *first* track with `has_embedded_cover=true`, with no ordering — which track wins is unstable across rescans.
    - `DeleteOrphanedAggregates` doesn't revalidate `CoverPath` for surviving albums.
    - No `Cache-Control`/ETag on `getCoverArt` responses; browser keeps serving the stale body until mtime or URL changes.
  Fix options: clear `album.CoverPath` at the start of each reconcile pass and redetect; or drop `CoverPath` entirely and resolve per-request from a current track's directory. Pick a deterministic embedded-cover track (e.g. lowest `(disc, track)`). Add a stable ETag (album `updated_at`) so edits immediately invalidate client caches.

## Frontend — Music Browsing & Features

- [ ] Favorites/starring: (partial — album detail & artist view toggles done; player now-playing has a like)
  - [ ] Star/unstar toggle on track rows (album view and queue)
  - [ ] Starred indicator on album grid cards in library view
  - [ ] Starred library section — browse starred albums, artists, and tracks (backed by `getStarred2`)
- [ ] Songs tab in Library — fetch and display all songs
- [ ] Artists tab in Library — replace the grid-of-artist-cards + drill-down into a single scrollable page grouped by artist: one header per artist (alphabetical), followed by that artist's albums sorted by year; no per-artist navigation step
- [ ] Spotify-style hover selection in song list — on row hover, show a checkbox next to the duration for multi-select
- [ ] Album cover drag and drop in the album view
- [ ] Improve the hero view of the album with details and actions
- [ ] Improve CRUD and views of playlists (check if playlist is part of the OpenSubsonic API)
- [ ] Better genre handling
- [ ] Remove podcast placeholder
- [ ] Custom icons for libraries

## Frontend — Player & Controls

- [ ] Improve execution history and runtime task management — surface long-running jobs (scans, imports, transcodes) with progress, status, and cancel controls; persist recent run history for inspection
- [ ] Mute support in the player — clicking the volume icon toggles mute (preserving the previous volume level to restore on unmute)
- [ ] Keyboard shortcuts — play/pause (space), next/previous track, volume up/down, mute, seek, toggle queue sidebar; add a help overlay listing them
- [ ] Jukebox functionality — use the web UI only to control the audio
- [ ] Relay — like jukebox, but loading songs from another instance

## Frontend — Branding & Layout

- [ ] Create an app icon / logo — favicon, PWA icons (various sizes), and a wordmark for the topbar
- [ ] Improve icon theme
- [ ] Unify the "Now Playing" / Queue view (`QueueView.vue`) onto the shared `LibraryScaffold` component introduced by the library-scaffold work (spec: `docs/superpowers/specs/2026-07-02-library-scaffold-and-artist-list-design.md`). 
      That work extracted QueueView's fixed-header + flush-right-scroll pattern into a generic `LibraryScaffold` (fixed header with title/summary + `#actions` slot, `flex:1;min-height:0` body slot) and adopted it across the library album/artist × list/cover views. QueueView still carries its own bespoke copy of that layout (`.queue-view` / `.queue-view-header` / `.queue-body`). **Deferred from that effort** because QueueView had active uncommitted WIP and its own drag/sortable complexity, so refactoring it then would have entangled unrelated changes. Follow-up: have `QueueView` render `LibraryScaffold` (title/summary; `#actions` = its edit/save/clear buttons; body = the history/current/upcoming or edit list), deleting the duplicated layout CSS. Once it has this second consumer, consider moving `LibraryScaffold` out of `components/library/` to a neutral home (e.g. `components/layout/`) and renaming it accordingly.

## Metadata & External Integrations

- [ ] Metadata editor for identifying songs
- [ ] Tag editor ↔ MusicBrainz/DB sync — let the metadata/tag editor pull MusicBrainz data into the DB and write tags back, in particular populate/correct the `MusicBrainz Artist Id` that drives artist-image fetching (see `docs/superpowers/specs/2026-06-30-durable-artist-image-store-design.md`)
- [ ] Last.fm scrobbling — explore Last.fm API integration for external scrobbling
- [ ] DLNA / UPnP endpoint — expose the library as a DLNA MediaServer so devices on the LAN (TVs, receivers, stock media players) can browse and stream without the Subsonic client



Radio stations Grid do not share style => verify that all grid items use the same component