# TODO

Items are split into **1.0** (the release gate) and **Future releases**.

# 1.0

## Security & Authentication

- [ ] Path traversal validation in `stream` and `getCoverArt` — validate that resolved file paths are within configured music directories before serving via `http.ServeFile`
- [ ] Authentication — implement real user auth with Subsonic token validation. The model is **decided**: see [`docs/agents/authentication.md`](docs/agents/authentication.md) (two modes, `builtin` and `proxy-header`, sharing one token layer). Implement it, don't invent alternatives. **Session-scoped `/rest` identity is implemented (interim)**: in native mode an `IdentityResolver` in `subsonic.Register` resolves session cookie → login, and per-user data (queue, stars, playlists, history) is owner-scoped; no session ⇒ error 40. This is the interim model — the PAT layer replaces it.
- [ ] Gate radio station CRUD (create/update/delete) behind admin role — reads stay open to all users, writes require admin. Rides on the role column that comes with the auth work
- [ ] Subsonic external-client auth — third-party Subsonic apps (Symfonium, DSub, etc.) authenticate with the Subsonic protocol's own query-string credentials. Implement via per-user **Personal Access Tokens (PATs)**, added as a generic CRUD feature in the `userauth` library (aether wires routing, authorization, and the management UI).
    - **Default: Recoverable PAT (encrypted-at-rest).** Subsonic token auth sends `t=md5(secret+salt)` with a fresh, client-chosen `salt` per request, so the server must hold the raw secret to recompute the hash. Hash-only/bcrypt storage cannot satisfy this, and MD5 is fixed by the protocol (no algorithm negotiation). Store the PAT symmetric-encrypted with a server key (not plaintext); optionally keep a `sha256` index alongside the ciphertext for fast lookup. Blast radius stays small — a PAT is a scoped, named, individually-revocable random value, never the user's bcrypt login password.
    - **Optional non-recoverable (hash-only) mode, per token/client.** Store only `sha256(token)` — more secure at rest, but only works with clients that transmit the raw token: OpenSubsonic `apiKey=<token>` or legacy `p=<token>`. It will NOT work with default token-auth (`t`+`s`) clients. `apiKey` is a recent (2024) fail-closed extension with no fallback, so an apiKey-only server locks out most current clients; treat hash-only as a per-token opt-in for clients known to support `apiKey`/`p=`.
    - Wiring: a thin Subsonic `AuthHandler` parses `u`/`t`/`s`/`p`/`apiKey` and validates against the PAT verifier. This **replaces the interim cookie resolver**. Per `authentication.md`, **`/rest` is token-only in every mode** — the SPA mints itself a token on boot and speaks standard Subsonic auth, so this is load-bearing for aether's own UI, not just for foreign clients.
- [ ] logout should stop playing, empty the local storage and clear tankstack cache

## Backend — API Surface

- [ ] Review the non-OpenSubsonic API surface — audit custom (non-Subsonic) endpoints, then move Libraries and Tasks management under an `/admin` path (e.g. `/api/admin/libraries`, `/api/admin/tasks`) so admin concerns are clearly separated from the Subsonic-compatible surface; update the frontend accordingly. **Pre-1.0 because it is a breaking URL reorg** — free now under the no-backwards-compat rule, expensive once anything depends on the paths

## Backend — OpenSubsonic Compliance

- [ ] XML response format — check compatibility with third-party Subsonic clients (DSub, Ultrasonic, Symfonium, etc.). XML is what several clients default to, so this gates the "third-party clients work" promise
- [ ] `albumToMap` missing `songCount`/`duration` when tracks are not preloaded — album list endpoints don't preload tracks, so these spec fields are absent in list responses and clients render "0 songs"

## Backend — Data Integrity & Scanning

- [ ] Use `errors.Is(err, gorm.ErrRecordNotFound)` in `FindOrCreateArtists` and `FindOrCreateAlbum` to distinguish not-found from real DB errors — today a genuine DB failure during a scan is treated as "not found"
- [ ] Full scan should drop each track's existing entries and re-insert from scratch (rather than updating in place) so stale/renamed artists, albums, genres, and other derived rows don't linger when tags change
      (partial: the *associations* are already rebuilt, not merged — `Association("Artists"/"Genres").Replace(...)` for the album in `reconcile.go:116` and for the track in `UpsertTrack` (`internal/store/track.go:12`), with `Cleanup` sweeping the orphans afterwards. What remains is album-level scalar fields, which are still updated in place on the row found by `(name_norm, album_artist_norm, mb_release_id)`.)
- [ ] Album cover on the library grid can show another album's image after metadata edits + rescan (works correctly on the album detail view). Remaining root causes:
    - Embedded-cover lookup `internal/store/track.go:113` (`GetCoverTrackPath`) returns the *first* track with `has_embedded_cover=true`, with no ordering — which track wins is unstable across rescans. Fix: order deterministically (e.g. lowest `(disc, track)`) — `internal/store/track.go:130` has no `Order`.
    - `DeleteOrphanedAggregates` doesn't revalidate `CoverPath` for surviving albums (`internal/store/scan_helpers.go:68`).
    - Two albums sharing a directory can still both point at the same `cover.jpg` — reconcile already re-checks a stored path every pass (`IsUsableCoverPath`), but nothing arbitrates between albums competing for one file.

## Backend — Resource Leaks

- [ ] Nothing ever evicts from the image cache — `imagecache.Delete(kind, key)` exists (`internal/imagecache/imagecache.go`) but has **zero production callers**, so deleting an entity leaves its derivative directory behind forever. Superseded *fingerprints* of a still-live entry are already swept on rebuild (`Cache.sweep`), so this is only about entities that go away. Wire `Delete` in alongside the existing `assets.Delete` calls: `subsonic/artists.go:67`, `subsonic/genres.go:56`, `subsonic/playlists.go:330,355`, `subsonic/radio.go:226-274`, plus album deletion in the scanner's orphan cleanup (`store.DeleteOrphanedAggregates`, which has no assetstore counterpart today). Two traps:
    - **The cache key is not always the assetstore key.** imagecache always keys on the DB ID (`media.go` `cacheKey:` sites), while assetstore keys can be an MBID (artist) or `RadioKey(streamURL)` (radio). Copying the key from the adjacent `assets.Delete` call would silently delete nothing — pass the DB ID.
    - **Editor thumbnails (`kind: "editor"`) can't be swept this way at all** — `pictureThumbKey` keys them by a hash of the file path or the image bytes (`metadata/pictures.go:419`), which is not derivable from an entity id. They need either a different key scheme or an age-based sweep, so a periodic prune task in `app/tasks` may be the better shape for the whole problem than per-deletion hooks. **The editor-thumbnail half is the 1.0-critical part** — those grow on every normal use of the metadata editor, not just on deletion. The per-entity wiring could slip to a later release if needed.

## Frontend — Branding

- [ ] Create an app icon / logo — favicon, PWA icons (various sizes), and a wordmark for the topbar. A public 1.0 cannot ship with the framework default favicon

---

# Future releases

## Backend — Multi-user

- [ ] Multi-user — per-user scoping of queue, stars, playlists, and history landed via session identity (`requestOwner` from the interim cookie resolver, owner-keyed schemas). Remaining work: the PAT layer (replaces the interim resolver and unblocks third-party clients) and per-user schema niceties (user ID instead of login string, cleaner renames). Known caveat: **owner is the LOGIN string**, so renaming a user detaches their owner-keyed data (queue, stars, playlists, history); acceptable pre-1.0, revisit with the PAT/user-id work.
- [ ] Favorites schema — superseded by `Owner` on `starred_items` (junction table keyed `(owner, item_type, item_id)` with unique index `idx_starred_item`). A per-type split (`album_stars`, `artist_stars`, `track_stars`) with `(user_id, item_id)` PKs and cascade deletes is optional later if there's a concrete benefit; the current schema works.

## Backend — OpenSubsonic Completeness

- [ ] Transcoding — identify formats browsers can't play natively and add FFmpeg transcoding. Not a 1.0 gate: browsers natively handle FLAC/MP3/OGG/Opus/AAC, so this is for exotic formats and bandwidth-limited remote listening, and it's a whole subsystem (ffmpeg dependency, cache, per-client profiles)
- [ ] CUE sheet support — single audio file + `.cue` sidecar (DJ mixes, EAC FLAC/APE rips) exposed as regular per-track albums with seamless web-UI playback; virtual tracks (file + time region), scanner pairing, ffmpeg remux slicing for third-party clients, OpenSubsonic extension for region offsets. Full assessment: `docs/cue-playing.md`
- [ ] setRating persistence — add rating column to tracks/albums when needed (handler exists in `annotation.go` but no rating column yet, so nothing is persisted). Note the handler currently **answers OK for a write it drops** — if this stays unimplemented through 1.0, that silent lie is the part worth revisiting
- [ ] getArtistInfo / getAlbumInfo — external metadata (MusicBrainz bios, similar artists)
- [ ] getTopSongs / getSimilarSongs — requires external data or play history analysis
- [ ] Podcasts, Jukebox — not in scope for this pass (Internet Radio has since been implemented: full CRUD under `/rest/` + Radio UI)
- [ ] Bookmarks — **not needed for resume**, `savePlayQueue`'s `position` already covers it. Only worth adding for per-track offsets that survive a queue replacement (audiobooks, long sets). If added, the play queue stays the single source of truth for the *current* track's position — do not write both on the same tick

## Backend — Performance

- [ ] `getPlaylists` N+1 queries — each playlist triggers separate count and duration queries; consider a single annotated query (still present: `playlists.go:25-26`)

## Backend — Library

- [ ] Add statistics in backend library: e.g. albums, artists, songs, genres, disk space used

## Frontend — Music Browsing & Features

- [ ] Artists tab in Library — replace the grid-of-artist-cards + drill-down into a single scrollable page grouped by artist: one header per artist (alphabetical), followed by that artist's albums sorted by year; no per-artist navigation step
      (partial: Library now has an Artists tab with grid and virtualized list views + alphabet rail — `ArtistGrid`/`ArtistListView` — but it's still rows of artists that navigate to `ArtistView`, not the grouped artist-header + albums layout)
- [ ] Spotify-style hover selection in song list — on row hover, show a checkbox next to the duration for multi-select
      (a checkbox-in-the-index-cell pattern already exists in queue *edit* mode — `QueueRow.vue:65` — but the browsing song lists (`AlbumTrackRow`, `GenreTrackRow`) select by plain/ctrl/shift click with no hover affordance)
- [ ] Album cover drag and drop in the album view
- [ ] Better genre handling — **needs scoping before it can be planned**
- [ ] Playlist edit is not a nice experience for now — **needs scoping**: name the specific interactions that are wrong (reorder? multi-remove? add-from-search?) before this can be estimated

## Frontend — Metadata editor

- [ ] Make the path that is visible in the top (on the side of choose folder) a fast way to load libraries
- [ ] Make it easier / faster to load a folder
- [ ] Check if we can add comments to metadata as part of the standard (e.g. the unreleased Alesha Dixon "Fool 4 U I Love")
- [ ] When identifying multiple songs, allow staging only a subset of the changes (e.g. genre only)
- [ ] Add cache to identify upstream calls — the *album* identify path already caches its MusicBrainz release lookups (`albumidentify.NewCachingReleaseLookup`, `api_v1.go:82`); this is about the per-file identify path

## Frontend — Player & Controls

- [ ] Jukebox functionality — use the web UI only to control the audio
- [ ] Relay — like jukebox, but loading songs from another instance

## Frontend — Layout

- [ ] Improve icon theme

## Metadata & External Integrations

- [ ] Last.fm scrobbling — forward each play to the user's Last.fm account so listening history lives off-box, plus the `track.love`/`artist.getInfo`/`album.getInfo` features that ride the same API key. **Nothing outbound exists today** — the local side only: the browser applies Last.fm's own 50%/4min/30s rule (`usePlayer.ts:53`), calls `/rest/scrobble`, and the server appends to `play_history` (`annotation.go:61`). No client, credentials, config, or retry queue. Two prerequisites: the `submission` parameter is currently ignored, so now-playing pings are recorded as completed plays; and there is no user/settings table to hold a session key. Consider **ListenBrainz first** — same off-box history and stats, one unsigned POST with a token, no signed handshake or app registration
- [ ] DLNA / UPnP endpoint — expose the library as a DLNA MediaServer so devices on the LAN (TVs, receivers, stock media players) can browse and stream without the Subsonic client

---

# Won't implement

- **Sharing and Chat.** Sharing exists to hand out public unauthenticated links (`/share.php?id=…&secret=…` + an HTML landing page) that bypass auth by design; Chat is a global message wall with no rooms or delivery, vestigial in the ecosystem and pointless on a single-user server. Don't add them, and don't file them as gaps again
