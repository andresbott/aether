# TODO

Items are split into **1.0** (the release gate), **Future releases**, and a
**Backlog** of things that need investigation before they can be scoped.

# 1.0

## Security & Authentication

- [ ] Authentication — implement real user auth with Subsonic token validation. The model is **decided**: see [`docs/agents/authentication.md`](docs/agents/authentication.md) (two modes, `native` and `proxy-header`, sharing one token layer — **both implemented**). Implement it, don't invent alternatives. **PAT layer implemented**: `/rest` authenticates via OpenSubsonic `apiKey` (PATs, hash-only storage in `userauth`'s `pat` service); session-scoped mint endpoint (`POST /api/v1/auth/token`) + CRUD endpoints (`/api/v1/auth/tokens[/*]`); SPA lifecycle (boot mint, transparent re-mint, generation counter on logout); PAT management UI in UserSettingsView. Sec-Fetch-Site CSRF mitigation and the interim cookie resolver removed.
- [ ] Change own password — **no UI and no endpoint**. `authentication.md` only names the missing UI, but the backend half is the bigger gap: `SetPasswordHash` has exactly two callers, the CLI (`app/cmd/usercmd.go:115`) and the admin users CRUD (`PUT /api/v1/users/{id}`, `handlers/users/users.go:339`), and that route sits in the **admin-only** tier — so a non-admin native user cannot change their own password by any means, and an admin can only do it by editing their own row in the admin panel. Needs a session-tier route (e.g. `PUT /api/v1/auth/password` taking current + new password, verifying the current one, added to `apiV1SessionPath` in `api_v1.go:43`) plus a form in `UserSettingsView`'s General panel. Must be unmounted in proxy-header mode — there the IdP owns credentials, and there is no login password to change (the provisioned row's password is a random throwaway).
- [ ] Auth none => listen only to localhost (mandatory ?) + allow to edit/reset users
      (the shipped default is wide open in both dimensions: `Method: AuthMethodNone` (`app/cmd/config.go:201`) and `BindIp: ""` → `:8075` on every interface (`config.go:178,183`, `listenAddr`), so a fresh install with no config file serves the whole library to the LAN unauthenticated. Options: force `BindIp` to loopback whenever the method is `none`, or keep it configurable and emit a loud startup warning like the `TrustedProxies`-empty one (`app/cmd/users.go:144`). Second half: in `none` mode there is **no user store at all** (`setupNativeAuth` returns nils), so there is nothing to edit or reset — a user created while `none` was active does not exist. The CLI has only `aether user hash` and `aether user reset-password`; no create, list, or role subcommand, so recovering from a lockout or preparing users before switching to `native` needs new CLI surface.)
- [ ] . No brute-force protection on login. => userauth library? 
      (confirmed absent: no rate limiting, lockout or attempt counting anywhere in `app/` — the only throttles are the outbound MusicBrainz/fanart ones in `internal/upstream`. The login flow is single-factor `RequireAny` with a **nil `AttemptStore`** (`handlers/auth/auth.go:37-46`), which the library permits for one-step policies, so nothing bounds password guessing against `POST /api/v1/auth/login`. `userauth` ships `flow/login.AttemptStore` but it exists to persist partially completed multi-factor logins, not to throttle — so this is aether's to add (per-IP + per-login backoff in front of the login handler) unless the library grows it.)

## Backend — API Surface

- [ ] Review the non-OpenSubsonic API surface — audit custom (non-Subsonic) endpoints, then move Libraries and Tasks management under an `/admin` path (e.g. `/api/admin/libraries`, `/api/admin/tasks`) so admin concerns are clearly separated from the Subsonic-compatible surface; update the frontend accordingly. **Pre-1.0 because it is a breaking URL reorg** — free now under the no-backwards-compat rule, expensive once anything depends on the paths. Not started: everything is still on one `/api/v1` subrouter (`app/router/main.go:201`), no `/admin` prefix anywhere. Note the *authorization* half already landed — `/api/v1` defaults to admin-only in both modes via the three-tier guards (`api_v1.go:56`, `proxy_auth.go`), so this is now purely about URL shape, not access control.

## Backend — OpenSubsonic Compliance

- [ ] `getUser` / `getUsers` — spec endpoints, **not routed at all** (absent from the register list in `subsonic/subsonic.go:207-269`). Clients call `getUser` to discover what the authenticated user may do — `streamRole`, `playlistRole`, `downloadRole`, `adminRole`, `coverArtRole`, … — and some disable UI or refuse to start without it. Aether already has everything the response needs: `requestOwner(r)` gives the login, `users.RoleOf` gives the vertical (the same lookup `restAdminChecker` uses), so the roles are a fixed mapping with `adminRole` the only variable one. Two decisions: the `username` a PAT-authenticated caller sees (the real login, not the tokenID virtual username), and whether `getUsers` (admin-only, lists everyone) is worth mounting at all given the users CRUD already lives on `/api/v1` — `getUser` for the caller's own record is the part clients actually need.
- [ ] XML response format — check compatibility with third-party Subsonic clients (DSub, Ultrasonic, Symfonium, etc.). XML is what several clients default to, so this gates the "third-party clients work" promise. Today `f=xml` is explicitly rejected with an error (`subsonic/subsonic.go:66-67`), so those clients fail at the first request. Note the handlers build `map[string]any` throughout (`albumToMap`, `trackToChild`, …), which does not marshal to spec-shaped XML — this needs a serialization layer, not a flag.

## Backend — Data Integrity & Scanning

- [ ] Full scan should drop each track's existing entries and re-insert from scratch (rather than updating in place) so stale/renamed artists, albums, genres, and other derived rows don't linger when tags change
      (partial, still accurate: the *associations* are rebuilt, not merged — `Association("Artists"/"Genres").Replace(...)` for the album at `internal/scanner/reconcile.go:116,121` and for the track at `internal/store/track.go:12,15`, with `Cleanup` sweeping the orphans afterwards. What remains is album-level scalar fields: `FindOrCreateAlbum` (`store/album.go:12`) matches on `(name_norm, album_artist_norm, mb_release_id)` and returns the existing row untouched, so scalars are only ever written at create time or updated in place elsewhere.)
- [ ] Album cover on the library grid can show another album's image after metadata edits + rescan (works correctly on the album detail view). Remaining root causes:
    - `DeleteOrphanedAggregates` doesn't revalidate `CoverPath` for surviving albums (`internal/store/scan_helpers.go:68` — 15 `DELETE` statements, no album `UPDATE`).
    - Two albums sharing a directory can still both point at the same `cover.jpg` — reconcile already re-checks a stored path every pass (`IsUsableCoverPath`), but nothing arbitrates between albums competing for one file.

## Backend — Resource Leaks

- [ ] Nothing ever evicts from the image cache — `Cache.Delete(kind, key)` exists (`internal/imagecache/imagecache.go:130`) and still has **zero callers of any kind, production or test**, so deleting an entity leaves its derivative directory behind forever. No prune task exists in `app/tasks` either. Superseded *fingerprints* of a still-live entry are already swept on rebuild (`Cache.sweep`), so this is only about entities that go away. Wire `Delete` in alongside the existing `assets.Delete` calls: `subsonic/artists.go:67`, `subsonic/genres.go:56`, `subsonic/playlists.go:330,355`, `subsonic/radio.go:226-274`, plus album deletion in the scanner's orphan cleanup (`store.DeleteOrphanedAggregates`, which has no assetstore counterpart today). Two traps:
    - **The cache key is not always the assetstore key.** imagecache always keys on the DB ID (`media.go` `cacheKey:` sites), while assetstore keys can be an MBID (artist) or `RadioKey(streamURL)` (radio). Copying the key from the adjacent `assets.Delete` call would silently delete nothing — pass the DB ID.
    - **Editor thumbnails (`kind: "editor"`) can't be swept this way at all** — `pictureThumbKey` keys them by a hash of the file path or the image bytes (`metadata/pictures.go:419`), which is not derivable from an entity id. They need either a different key scheme or an age-based sweep, so a periodic prune task in `app/tasks` may be the better shape for the whole problem than per-deletion hooks. **The editor-thumbnail half is the 1.0-critical part** — those grow on every normal use of the metadata editor, not just on deletion. The per-entity wiring could slip to a later release if needed.

## Frontend 

- [x] Create an app icon / logo. Browser/PWA side: `zarf/icon/web/` holds the cleaned web sources (rounded, square, maskable) derived from the `icon2.svg` master, `make icons` (`zarf/icon/render.sh`) renders them into `webui/public/` (`icon.svg`, `favicon.ico`, `apple-touch-icon.png`, `icon-{192,512}.png`, `icon-maskable-{192,512}.png`), and `index.html` + the manifest link them all. In-app side: the diamond rendition (`assets/aether-mark.svg` via `components/common/BrandMark.vue`) replaced the `◈` placeholder in the sidebar and the mobile drawer and now sits beside the wordmark on the login card. The wordmark itself **stays styled text** — only the mark is artwork.
- [] library also shows songs additionally to albums and artists
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

## Backend — Library

- [ ] Add statistics in backend library: e.g. albums, artists, songs, genres, disk space used

## Frontend — Music Browsing & Features

- [ ] Artists tab in Library — replace the grid-of-artist-cards + drill-down into a single scrollable page grouped by artist: one header per artist (alphabetical), followed by that artist's albums sorted by year; no per-artist navigation step
      (partial, still accurate: Library has an Artists tab with grid and virtualized list views + alphabet rail — `components/library/ArtistGrid.vue`/`ArtistListView.vue`, wired at `LibraryView.vue:206-211` — but each card is a `router-link` to the `artist` route (`ArtistCard.vue:33`), so it's still artists-that-navigate, not the grouped artist-header + albums layout)
- [ ] Spotify-style hover selection in song list — on row hover, show a checkbox next to the duration for multi-select
      (a checkbox-in-the-index-cell pattern already exists in queue *edit* mode — `components/layout/QueueRow.vue:68-82` — but the browsing song lists (`components/library/AlbumTrackRow.vue`, `GenreTrackRow.vue`) select by plain/ctrl/shift click with no hover affordance; they only tint the row on `:hover`)
- [ ] Album cover drag and drop in the album view
- [ ] Better genre handling — **needs scoping before it can be planned**
- [ ] Search should also return genres
- [ ] Playlist edit is not a nice experience for now — **needs scoping**: name the specific interactions that are wrong (reorder? multi-remove? add-from-search?) before this can be estimated

## Frontend — Metadata editor

- [ ] Make the path that is visible in the top (on the side of choose folder) a fast way to load libraries
- [ ] Make it easier / faster to load a folder
- [ ] Check if we can add comments to metadata as part of the standard (e.g. the unreleased Alesha Dixon "Fool 4 U I Love")
- [ ] When identifying multiple songs, allow staging only a subset of the changes (e.g. genre only)

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
