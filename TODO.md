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
- [ ] CUE sheet support — single audio file + `.cue` sidecar (DJ mixes, EAC FLAC/APE rips) exposed as regular per-track albums with seamless web-UI playback; virtual tracks (file + time region), scanner pairing, ffmpeg remux slicing for third-party clients, OpenSubsonic extension for region offsets. Full assessment: `docs/cue-playing.md`
- [ ] setRating persistence — add rating column to tracks/albums when needed (handler exists in annotation.go but no rating column yet, so not persisted)
- [ ] getArtistInfo / getAlbumInfo — external metadata (MusicBrainz bios, similar artists)
- [ ] getTopSongs / getSimilarSongs — requires external data or play history analysis
- [ ] Podcasts, Bookmarks, Sharing, Chat, Jukebox — not in scope for this pass (Internet Radio has since been implemented: full CRUD under `/rest/` + Radio UI)

## Backend — Performance

- [ ] `getPlaylists` N+1 queries — each playlist triggers separate count and duration queries; consider a single annotated query (still present: `playlists.go:25-26`)
- [ ] `albumToMap` missing `songCount`/`duration` when tracks are not preloaded — album list endpoints don't preload tracks, so these fields are absent in list responses
- [ ] cover art is extacted from the file on the fly, it might perform better if we extract at scannnig

## Backend — Data Integrity & Scanning

- [ ] Favorites schema — three junction tables (`album_stars`, `artist_stars`, `track_stars`), each with composite PK `(user_id, item_id)`, a `starred_at` timestamp, and cascade deletes on user/item removal; replace the current single `starred_items` table if one exists
- [ ] Use `errors.Is(err, gorm.ErrRecordNotFound)` in `FindOrCreateArtists` and `FindOrCreateAlbum` to distinguish not-found from real DB errors
- [ ] Full scan should drop each track's existing entries and re-insert from scratch (rather than updating in place) so stale/renamed artists, albums, genres, and other derived rows don't linger when tags change
      (partial: the *associations* are already rebuilt, not merged — `Association("Artists"/"Genres").Replace(...)` for the album in `reconcile.go:116` and for the track in `UpsertTrack` (`internal/store/track.go:12`), with `Cleanup` sweeping the orphans afterwards. What remains is album-level scalar fields, which are still updated in place on the row found by `(name_norm, album_artist_norm, mb_release_id)`.)
- [ ] Album cover on the library grid can show another album's image after metadata edits + rescan (works correctly on the album detail view). Root causes:
    - ~~`internal/scanner/reconcile.go` only sets `album.CoverPath` when empty; it's never re-evaluated~~ — fixed: reconcile re-checks the stored path every pass via `IsUsableCoverPath` (`reconcile.go:106`, `cover.go:42`), repointing when the file is gone or its name isn't front art; regression tests in `reconcile_test.go`. Two albums sharing a directory can still both point at the same `cover.jpg`.
    - Embedded-cover lookup `internal/store/track.go:113` (`GetCoverTrackPath`) returns the *first* track with `has_embedded_cover=true`, with no ordering — which track wins is unstable across rescans.
    - `DeleteOrphanedAggregates` doesn't revalidate `CoverPath` for surviving albums.
    - ~~No `Cache-Control`/ETag on `getCoverArt` responses~~ — done: `Cache-Control: no-cache` on every response (`media.go:169`) plus an ETag over path+size+mtime with `Last-Modified` deliberately omitted (`serveCoverFile`, `media.go:217`), so swapping to an *older* file still invalidates; `If-None-Match` → 304 covered in `media_test.go:610`.
  Still open: pick a deterministic embedded-cover track (e.g. lowest `(disc, track)`) in `GetCoverTrackPath` (`internal/store/track.go:130` has no `Order`); have `DeleteOrphanedAggregates` revalidate `CoverPath` for surviving albums (`internal/store/scan_helpers.go:68`).

## Frontend — Music Browsing & Features

- [ ] Favorites/starring: (partial — album detail, artist view and song-detail toggles done; player now-playing has a like; playlist stars done via the `playlistStar` extension. Every toggle now uses the `pi pi-heart(-fill)` icon with "Add to/Remove from favorites" wording, and `/rest` emits `starred` on artists/albums/songs/playlists so state survives a reload)
  - [ ] Star/unstar toggle on track rows (album view and queue)
  - [x] Starred indicator on album grid cards in library view — `AlbumCard`/`ArtistCard` carry a
        hover-revealed heart that stays visible while favorited, same pattern as `PlaylistCard`
  - [ ] Starred library section — browse starred albums, artists, and tracks (backed by `getStarred2`)
        (FULLY OPEN again. This used to be partially answered by the Discovery "Favorites" section at
        `/discover/favorites`, but that route was deleted when Discovery became a single ranked feed.
        Favorites are now only a scoring term in that feed — a ranking boost, not visible or
        browsable (the reason badge that used to surface it was removed too). Nothing browses starred
        items today: no albums, no artists, no tracks.)
- [ ] Artists tab in Library — replace the grid-of-artist-cards + drill-down into a single scrollable page grouped by artist: one header per artist (alphabetical), followed by that artist's albums sorted by year; no per-artist navigation step
      (partial: Library now has an Artists tab with grid and virtualized list views + alphabet rail — `ArtistGrid`/`ArtistListView` — but it's still rows of artists that navigate to `ArtistView`, not the grouped artist-header + albums layout)
- [ ] Spotify-style hover selection in song list — on row hover, show a checkbox next to the duration for multi-select
      (a checkbox-in-the-index-cell pattern already exists in queue *edit* mode — `QueueRow.vue:65` — but the browsing song lists (`AlbumTrackRow`, `GenreTrackRow`) select by plain/ctrl/shift click with no hover affordance)
- [ ] Album cover drag and drop in the album view
- [ ] Better genre handling

## Frontend — Metadata editor
- [ ] make the path that is visible in the top ( on the side of choose folder) asa fast way to load libraries
- [] make it easeir / faster to load a folder
- [] check if we can add comments to metadata as part of the sandard ( e..g the unreleased alesiah dixon fon 4 u i love
- [x] ~~add a toolip to let t he users kjnow that albums are groped by release id~~ — done: `!` marker on the album Name field in `EditPanel.vue` spelling out the `(album name, album artist, release ID)` identity and the empty-ID-splits-the-album trap
- when i identify multiple songs i might want to only stage a subset of the changes, e.g. genre
- [ ] add cache to identify upstream calls

## Frontend — Player & Controls

- [x] ~~Improve execution history and runtime task management~~ — done: `ExecutionHistory.vue` shows status/cancel per run, backend persists executions with list/cancel/logs endpoints (`/api/v1/tasks/executions`, `taskrunner` persistence)
- [ ] Mute support in the player — clicking the volume icon toggles mute (preserving the previous volume level to restore on unmute)
- [ ] Keyboard shortcuts — play/pause (space), next/previous track, volume up/down, mute, seek, toggle queue sidebar; add a help overlay listing them
- [ ] Jukebox functionality — use the web UI only to control the audio
- [ ] Relay — like jukebox, but loading songs from another instance
- [] All music should also contain playlists, and add filters by genre and star valuation, move the libraries at same level, if only one library make it automatic; move all music to a new entry "discover"
      (partial: `/discover` now exists as a single ranked album+playlist feed — `DiscoveryView`, served
      by the `getDiscovery` extension; folding Library's "All Music" into it, the genre/star filters and
      the library-level restructuring are still open)
- the aeteher icon shoul go to play now if playing otherwise go to discover 
- [] search should also return genres 
- [] sen't cover images are full images instead of optimized and cached

## Frontend — Branding & Layout

- [ ] Create an app icon / logo — favicon, PWA icons (various sizes), and a wordmark for the topbar
- [ ] Improve icon theme

## Metadata & External Integrations

- [ ] Last.fm scrobbling — explore Last.fm API integration for external scrobbling
- [ ] DLNA / UPnP endpoint — expose the library as a DLNA MediaServer so devices on the LAN (TVs, receivers, stock media players) can browse and stream without the Subsonic client



