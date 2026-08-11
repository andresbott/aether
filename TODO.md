# TODO

Items are split into **1.0** (the release gate), **Future releases**, and a
**Backlog** of things that need investigation before they can be scoped.

# 1.0

## Security & Authentication

- [x] Path traversal validation in `stream` and `getCoverArt` — **done**. `internal/pathguard` answers "is this absolute path inside one of these roots?" (symlink-resolving, boundary-aware so `/music` never contains `/music-private`, deny-on-no-roots). The media handlers enforce it at the three points a DB-sourced path is used: the track file in `stream`, `coverPath` in `coverSources`, and the embedded-cover track path in `embeddedCoverSource`. Roots come from `store.LibraryRoots` read **on demand** (`subsonic.WithLibraryRoots`, wired at `app/router/main.go:205`) rather than snapshotted, since libraries are created at runtime; the guard is rebuilt only when the root set changes. A refused stream answers 70 (no existence oracle); a refused cover falls through to the next source. The `//nolint:gosec` comments and the `.golangci.yaml` note now say *enforced* instead of *assumed*.
- [ ] Authentication — implement real user auth with Subsonic token validation. The model is **decided**: see [`docs/agents/authentication.md`](docs/agents/authentication.md) (two modes, `native` and `proxy-header`, sharing one token layer — **both implemented**). Implement it, don't invent alternatives. **PAT layer implemented**: `/rest` authenticates via OpenSubsonic `apiKey` (PATs, hash-only storage in `userauth`'s `pat` service); session-scoped mint endpoint (`POST /api/v1/auth/token`) + CRUD endpoints (`/api/v1/auth/tokens[/*]`); SPA lifecycle (boot mint, transparent re-mint, generation counter on logout); PAT management UI in UserSettingsView. Sec-Fetch-Site CSRF mitigation and the interim cookie resolver removed.
- [x] Gate radio station CRUD (create/update/delete) behind admin role — **done**. The three write handlers call `requireAdmin` (Subsonic error 50; reads stay open). Role plumbing: `subsonic.WithAdminChecker` injects an `AdminChecker` (owner login → is-admin) next to the `IdentityResolver`; the router wires `restAdminChecker` (`app/router/main.go`), which resolves the login to a user row and applies `users.RoleOf`. nil checker (auth "none") passes everyone — single fixed owner. Proxy mode mirrors the header-derived role into the DB groups on `/api/v1` requests (`resolveProxyIdentity`) because `/rest` is proxy-bypassed and the checker can only consult the DB.
- [ ] Review the 25-tokens-per-user default (pat.Opts.MaxPerUser). **The spa half is fixed**: the mint sweep now revokes ALL of the caller's spa tokens (live included, since the SPA holds exactly one and the new mint supersedes it), so spa tokens are bounded at ~1/user and repeated boots can no longer hit the cap. What remains is the cap *policy* for user-created client PATs — 25 live client tokens alone still 409s on the next mint.
- [x] Subsonic external-client auth — **done**. Third-party Subsonic apps authenticate via PATs in two forms: `apiKey` (any PAT) and password flows (`u`+`t`+`s` or `u`+`p`, where `u` is a usertoken PAT's tokenID). Both types implemented: apikey (hash-only) and usertoken (AES-256-GCM encrypted at rest, key in `<DataDir>/pat.keys`). See [`docs/agents/authentication.md`](docs/agents/authentication.md).

## Backend — API Surface

- [ ] Review the non-OpenSubsonic API surface — audit custom (non-Subsonic) endpoints, then move Libraries and Tasks management under an `/admin` path (e.g. `/api/admin/libraries`, `/api/admin/tasks`) so admin concerns are clearly separated from the Subsonic-compatible surface; update the frontend accordingly. **Pre-1.0 because it is a breaking URL reorg** — free now under the no-backwards-compat rule, expensive once anything depends on the paths. Not started: everything is still on one `/api/v1` subrouter (`app/router/main.go:201`), no `/admin` prefix anywhere. Note the *authorization* half already landed — `/api/v1` defaults to admin-only in both modes via the three-tier guards (`api_v1.go:56`, `proxy_auth.go`), so this is now purely about URL shape, not access control.

## Backend — OpenSubsonic Compliance

- [ ] XML response format — check compatibility with third-party Subsonic clients (DSub, Ultrasonic, Symfonium, etc.). XML is what several clients default to, so this gates the "third-party clients work" promise. Today `f=xml` is explicitly rejected with an error (`subsonic/subsonic.go:66-67`), so those clients fail at the first request. Note the handlers build `map[string]any` throughout (`albumToMap`, `trackToChild`, …), which does not marshal to spec-shaped XML — this needs a serialization layer, not a flag.
- [x] `albumToMap` missing `songCount`/`duration` when tracks are not preloaded — **done**. Three callers (`getAlbumList2`, `getStarred2`, `discoveryAlbums`) already batched it via `store.AlbumTrackStats`; the two that still omitted both fields — `search3` and `getArtist` — now do the same. The per-call-site inline copies were collapsed onto one `applyAlbumStats` helper (`subsonic/browsing.go`), so a new list handler has one obvious thing to call. No preloading of whole tracks into list responses.

## Backend — Data Integrity & Scanning

- [x] Use `errors.Is(err, gorm.ErrRecordNotFound)` in `FindOrCreateArtists` and `FindOrCreateAlbum` to distinguish not-found from real DB errors — **done**. Both now return the error instead of falling through to a create, so a transient DB failure during a scan can no longer silently duplicate an artist or split one album in two.
- [ ] Full scan should drop each track's existing entries and re-insert from scratch (rather than updating in place) so stale/renamed artists, albums, genres, and other derived rows don't linger when tags change
      (partial, still accurate: the *associations* are rebuilt, not merged — `Association("Artists"/"Genres").Replace(...)` for the album at `internal/scanner/reconcile.go:116,121` and for the track at `internal/store/track.go:12,15`, with `Cleanup` sweeping the orphans afterwards. What remains is album-level scalar fields: `FindOrCreateAlbum` (`store/album.go:12`) matches on `(name_norm, album_artist_norm, mb_release_id)` and returns the existing row untouched, so scalars are only ever written at create time or updated in place elsewhere.)
- [ ] Album cover on the library grid can show another album's image after metadata edits + rescan (works correctly on the album detail view). Remaining root causes:
    - [x] Embedded-cover lookup `GetCoverTrackPath` — **fixed**: the query now orders by `disc_number, track_number, file_path`, so the lowest `(disc, track)` track always wins and the choice no longer shifts with directory-walk order across rescans.
    - `DeleteOrphanedAggregates` doesn't revalidate `CoverPath` for surviving albums (`internal/store/scan_helpers.go:68` — 15 `DELETE` statements, no album `UPDATE`).
    - Two albums sharing a directory can still both point at the same `cover.jpg` — reconcile already re-checks a stored path every pass (`IsUsableCoverPath`), but nothing arbitrates between albums competing for one file.
- [] how about to allow to configure an library ID number has fixed id in the config

## Backend — Resource Leaks

- [ ] Nothing ever evicts from the image cache — `Cache.Delete(kind, key)` exists (`internal/imagecache/imagecache.go:130`) and still has **zero callers of any kind, production or test**, so deleting an entity leaves its derivative directory behind forever. No prune task exists in `app/tasks` either. Superseded *fingerprints* of a still-live entry are already swept on rebuild (`Cache.sweep`), so this is only about entities that go away. Wire `Delete` in alongside the existing `assets.Delete` calls: `subsonic/artists.go:67`, `subsonic/genres.go:56`, `subsonic/playlists.go:330,355`, `subsonic/radio.go:226-274`, plus album deletion in the scanner's orphan cleanup (`store.DeleteOrphanedAggregates`, which has no assetstore counterpart today). Two traps:
    - **The cache key is not always the assetstore key.** imagecache always keys on the DB ID (`media.go` `cacheKey:` sites), while assetstore keys can be an MBID (artist) or `RadioKey(streamURL)` (radio). Copying the key from the adjacent `assets.Delete` call would silently delete nothing — pass the DB ID.
    - **Editor thumbnails (`kind: "editor"`) can't be swept this way at all** — `pictureThumbKey` keys them by a hash of the file path or the image bytes (`metadata/pictures.go:419`), which is not derivable from an entity id. They need either a different key scheme or an age-based sweep, so a periodic prune task in `app/tasks` may be the better shape for the whole problem than per-deletion hooks. **The editor-thumbnail half is the 1.0-critical part** — those grow on every normal use of the metadata editor, not just on deletion. The per-entity wiring could slip to a later release if needed.

## Frontend 

- [ ] Create an app icon / logo — PWA icons (various sizes) and favicon wiring. **Partly done**: source art exists in `zarf/icon/` (`icon.svg`, `favicon.svg`, `256.png`, `64.png`) and the sidebar has a text wordmark + `◈` brand mark (`AppSidebar.vue:186-187`). What remains is shipping them to the browser: `webui/index.html` declares **no `<link rel="icon">` and no manifest**, there is no `webui/public/`, and `/favicon.ico` falls through to the SPA handler (answers 200 with `text/html`). Also decide whether the wordmark stays as styled text or becomes the SVG.
- [] library also shows songs additionally to albums and artists
- [x] double click on a song adds it to the queue instead of replacing — **done**. Track rows emit `enqueue` (not `play`) and every host answers with `player.enqueueAndPlayIfIdle([song])` (`webui/src/composables/usePlayer.ts`): the track appends to the end, `currentIndex` and the shuffle run are left intact, and playback only starts when the player was idle (empty queue / no loaded track) so the gesture still makes sound on a fresh session. Applied to all four track lists — `AlbumView`, `GenreDetailView`, `SearchView`, `PlaylistDetailView` — so the gesture means one thing app-wide; replacing the queue stays the hero Play button's job. `QueueRow` keeps `play` (inside the queue, double-click means "jump to this slot"). Contract recorded in [`docs/architecture/unified-play-experience.md`](docs/architecture/unified-play-experience.md).
- [] impelemt radio mode queue => keep playing based on same type/taste

---

# Future releases

## Backend — Multi-user

- [ ] Multi-user — per-user scoping of queue, stars, playlists, and history landed via session identity (owner-keyed schemas). PAT layer landed (replaces the interim cookie resolver and unblocks third-party clients). Remaining work: re-key the owner columns on `User.ID` instead of the login string, then re-enable rename. **The caveat is now contained, not live**: `owner` is still the LOGIN string (`patIdentityResolver` returns `info.LoginID`, `app/router/main.go:133`), so a rename would orphan queue/stars/playlists/history — but renaming is refused with 400 (`errRenameUnsupported`, `handlers/users/users.go`) and `UserDialog.vue` shows the login read-only, so nothing can trigger the orphaning. Lifting the refusal is the last step of this item, not a prerequisite.
- [ ] Favorites schema — superseded by `Owner` on `starred_items` (junction table keyed `(owner, item_type, item_id)` with unique index `idx_starred_item`). A per-type split (`album_stars`, `artist_stars`, `track_stars`) with `(user_id, item_id)` PKs and cascade deletes is optional later if there's a concrete benefit; the current schema works.

## Backend — OpenSubsonic Completeness

- [ ] Transcoding — identify formats browsers can't play natively and add FFmpeg transcoding. Not a 1.0 gate: browsers natively handle FLAC/MP3/OGG/Opus/AAC, so this is for exotic formats and bandwidth-limited remote listening, and it's a whole subsystem (ffmpeg dependency, cache, per-client profiles)
- [ ] CUE sheet support — single audio file + `.cue` sidecar (DJ mixes, EAC FLAC/APE rips) exposed as regular per-track albums with seamless web-UI playback; virtual tracks (file + time region), scanner pairing, ffmpeg remux slicing for third-party clients, OpenSubsonic extension for region offsets. Full assessment: `docs/cue-playing.md`
- [ ] setRating persistence — add rating column to tracks/albums when needed. Confirmed unimplemented: `setRating` is a two-line `writeResponse(w, nil)` (`subsonic/annotation.go:103-105`) and no model carries a rating field. The handler **answers OK for a write it drops** — if this stays unimplemented through 1.0, that silent lie is the part worth revisiting
- [ ] getArtistInfo / getAlbumInfo — external metadata (MusicBrainz bios, similar artists)
- [ ] getTopSongs / getSimilarSongs — requires external data or play history analysis
- [ ] Podcasts, Jukebox — not in scope for this pass (Internet Radio has since been implemented: full CRUD under `/rest/` + Radio UI)
- [ ] Bookmarks — **not needed for resume**, `savePlayQueue`'s `position` already covers it. Only worth adding for per-track offsets that survive a queue replacement (audiobooks, long sets). If added, the play queue stays the single source of truth for the *current* track's position — do not write both on the same tick

## Backend — Performance

- [x] `getPlaylists` N+1 queries — **done**. `store.PlaylistTrackStats(ids)` returns count + duration for every playlist in one grouped query (LEFT JOIN, so an entry whose track row is gone still counts toward the length, matching the old `GetPlaylistTrackCount`), hoisted out of the loop next to the existing `StarredAt`/`PlaylistStats` batches. Measured by a query-counting test: 15 SELECTs → 3 for 6 playlists. `GetPlaylistTrackCount`/`GetPlaylistDuration` are still used by the single-playlist paths.

## Backend — Library

- [ ] Add statistics in backend library: e.g. albums, artists, songs, genres, disk space used

## Frontend — Music Browsing & Features

- [ ] Artists tab in Library — replace the grid-of-artist-cards + drill-down into a single scrollable page grouped by artist: one header per artist (alphabetical), followed by that artist's albums sorted by year; no per-artist navigation step
      (partial, still accurate: Library has an Artists tab with grid and virtualized list views + alphabet rail — `components/library/ArtistGrid.vue`/`ArtistListView.vue`, wired at `LibraryView.vue:206-211` — but each card is a `router-link` to the `artist` route (`ArtistCard.vue:33`), so it's still artists-that-navigate, not the grouped artist-header + albums layout)
- [ ] Spotify-style hover selection in song list — on row hover, show a checkbox next to the duration for multi-select
      (a checkbox-in-the-index-cell pattern already exists in queue *edit* mode — `components/layout/QueueRow.vue:68-82` — but the browsing song lists (`components/library/AlbumTrackRow.vue`, `GenreTrackRow.vue`) select by plain/ctrl/shift click with no hover affordance; they only tint the row on `:hover`)
- [ ] Album cover drag and drop in the album view
- [ ] Better genre handling — **needs scoping before it can be planned**
- [ ] Playlist edit is not a nice experience for now — **needs scoping**: name the specific interactions that are wrong (reorder? multi-remove? add-from-search?) before this can be estimated

## Frontend — Metadata editor

- [ ] Make the path that is visible in the top (on the side of choose folder) a fast way to load libraries
- [ ] Make it easier / faster to load a folder
- [ ] Check if we can add comments to metadata as part of the standard (e.g. the unreleased Alesha Dixon "Fool 4 U I Love")
- [ ] When identifying multiple songs, allow staging only a subset of the changes (e.g. genre only)
- [x] Add cache to identify upstream calls — both paths cache now. The *album* path caches its MusicBrainz release lookups (`albumidentify.NewCachingReleaseLookup`, `api_v1.go:218`); the per-file path caches fingerprint identifications in `identify.Cache` (`internal/identify/cache.go`, keyed by content-ish identity — path + size + modtime), consulted in `Identifier.Identify` (`internal/identify/identify.go:86,105`) and wired in production at `app/cmd/server.go:178` with `identify.DefaultCacheSize` (5000).

## Frontend — Player & Controls

- [ ] Jukebox functionality — use the web UI only to control the audio
- [ ] Relay — like jukebox, but loading songs from another instance

## Frontend — Layout

- [ ] Improve icon theme

## Metadata & External Integrations

- [ ] Last.fm scrobbling — forward each play to the user's Last.fm account so listening history lives off-box, plus the `track.love`/`artist.getInfo`/`album.getInfo` features that ride the same API key. **Nothing outbound exists today** (confirmed: no `lastfm`/`listenbrainz` reference anywhere in Go) — the local side only: the browser applies Last.fm's own 50%/4min/30s rule (`usePlayer.ts:53`), calls `/rest/scrobble`, and the server appends to `play_history` (`subsonic/annotation.go:62`). No client, credentials, config, or retry queue. Two prerequisites: the `submission` parameter is currently ignored, so now-playing pings are recorded as completed plays; and there is no user/settings table to hold a session key. Consider **ListenBrainz first** — same off-box history and stats, one unsigned POST with a token, no signed handshake or app registration
- [ ] DLNA / UPnP endpoint — expose the library as a DLNA MediaServer so devices on the LAN (TVs, receivers, stock media players) can browse and stream without the Subsonic client

---

# Backlog — needs investigation

Items parked here are **not release-gated**. Each one has been looked at enough to
know it is real and roughly why, but the direction is undecided — so they wait for
a deliberate scoping pass rather than sitting in 1.0 or Future releases pretending
to be planned work.

- [ ] **Radio stations are not saved in the play queue between sessions / devices.** Investigated 2026-08-11, not fixed; needs a direction decision before any code.
    - **Root cause:** a station enters the queue as a synthetic `Song` (`webui/src/utils/radioSong.ts:8` — `id: radio-<name>` plus a `streamUrl`, deliberately *not* the real `rs-<n>`). `useQueueSync.pushQueue` (`webui/src/composables/useQueueSync.ts:50`) sends `queue.map(s => s.id)` verbatim, and the backend's `decodeTrackIDs` (`subsonic/playlists.go:249`) silently drops every non-`tr-` id. `model.PlayQueueEntry.TrackID uint` is track-only anyway, and restore rebuilds entries from `model.Track` via `starredSongList`, which emits no `streamUrl` — so nothing radio-shaped can round-trip even if it were stored.
    - **Two collateral bugs are worse than the missing station, and are worth fixing whichever direction wins:** (a) the client's `currentIndex` counts the dropped entry, so a station *before* the playing track makes `currentIndex >= len(trackIDs)` and `savePlayQueueByIndex` answers error 10 (`subsonic/playqueue.go:73`) — **discarding the whole save, tracks included**; a station *after* it nominates the wrong current track. (b) A radio-only queue decodes to zero ids, which routes to `clearSavedQueue` (`playqueue.go:58`) and **deletes the other device's saved session**. Minor: `usePlayer.ts:74` scrobbles `radio-<name>`, which answers `invalid id` (log noise only).
    - **OpenSubsonic has no mechanism for this.** `savePlayQueue`'s `id`/`current` are defined strictly as song ids; `PlayQueue.entry` is an array of `Child` ("the list of songs in the queue") with a MUST that `current` be "a valid id in the list of songs"; `Child.type` ∈ `{music, podcast, audiobook, video}` and `mediaType` ∈ `{song, album, artist}` — no stream/radio value, no `streamUrl` field. Stations are a separate entity with their own CRUD: the spec's model is that a station is something you *play*, not something you *queue*. The extension registry (11 entries) has nothing for it — podcast episodes got a dedicated extension, radio-in-queue never did — so there is no upstream extension to adopt, only one to author.
    - **Options:** **(A)** follow the spec — radio doesn't persist cross-device, but the client strips non-track entries and recomputes `currentIndex` over the survivors so (a)/(b) become impossible. Pure bugfix, no spec deviation, no schema change. **(B)** author a `radioQueue` extension — polymorphic `PlayQueueEntry` (kind + ref), `rs-` ids accepted on the `ByIndex` variant only, spec-shaped `getPlayQueue` still filtered to songs. Schema drop, and deviate-first-upstream-later.
    - **Trap for (B):** `stream` discards the id kind (`_, id, err := decodeID(...)`, `subsonic/media.go:29`), so `stream?id=rs-3` today serves **track 3's file**. A latent bug a radio-in-queue design walks straight into — and worth fixing on its own regardless.

---

# Won't implement

- **Sharing and Chat.** Sharing exists to hand out public unauthenticated links (`/share.php?id=…&secret=…` + an HTML landing page) that bypass auth by design; Chat is a global message wall with no rooms or delivery, vestigial in the ecosystem and pointless on a single-user server. Don't add them, and don't file them as gaps again
