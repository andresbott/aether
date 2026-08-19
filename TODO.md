    # TODO

Items are split into **1.0** (the release gate), **Future releases**, and a
**Backlog** of things that need investigation before they can be scoped.

# 1.0

## Security & Authentication

- [ ] Authentication — implement real user auth with Subsonic token validation. The model is **decided**: see [`docs/agents/authentication.md`](docs/agents/authentication.md) (two modes, `native` and `proxy-header`, sharing one token layer — **both implemented**). Implement it, don't invent alternatives. **PAT layer implemented**: `/rest` authenticates via OpenSubsonic `apiKey` (PATs, hash-only storage in `userauth`'s `pat` service); session-scoped mint endpoint (`POST /api/v1/auth/token`) + CRUD endpoints (`/api/v1/auth/tokens[/*]`); SPA lifecycle (boot mint, transparent re-mint, generation counter on logout); PAT management UI in UserSettingsView. Sec-Fetch-Site CSRF mitigation and the interim cookie resolver removed.
- [ ] Change own password — **no UI and no endpoint**. `authentication.md` only names the missing UI, but the backend half is the bigger gap: `SetPasswordHash` has exactly two callers, the CLI (`app/cmd/usercmd.go:115`) and the admin users CRUD (`PUT /api/v1/users/{id}`, `handlers/users/users.go:339`), and that route sits in the **admin-only** tier — so a non-admin native user cannot change their own password by any means, and an admin can only do it by editing their own row in the admin panel. Needs a session-tier route (e.g. `PUT /api/v1/auth/password` taking current + new password, verifying the current one, added to `apiV1SessionPath` in `api_v1.go:43`) plus a form in `UserSettingsView`'s General panel. Must be unmounted in proxy-header mode — there the IdP owns credentials, and there is no login password to change (the provisioned row's password is a random throwaway).
- [ ] Auth none => listen only to localhost (mandatory ?) + allow to edit/reset users
      (the shipped default is wide open in both dimensions: `Method: AuthMethodNone` (`app/cmd/config.go:201`) and `BindIp: ""` → `:8075` on every interface (`config.go:178,183`, `listenAddr`), so a fresh install with no config file serves the whole library to the LAN unauthenticated. Options: force `BindIp` to loopback whenever the method is `none`, or keep it configurable and emit a loud startup warning like the `TrustedProxies`-empty one (`app/cmd/users.go:144`). Second half: in `none` mode there is **no user store at all** (`setupNativeAuth` returns nils), so there is nothing to edit or reset — a user created while `none` was active does not exist. The CLI has only `aether user hash` and `aether user reset-password`; no create, list, or role subcommand, so recovering from a lockout or preparing users before switching to `native` needs new CLI surface.)
- [ ] `updateArtist` and `updateGenre` accept cover writes from any authenticated user — decide and document the `/rest` write-authorization policy. `updateAlbum` was gated on `requireAdmin` (`handlers/subsonic/albums.go`, Subsonic error 50) when it was added; the two older sibling cover-art extensions were left as they were because they predate that change, so today the same class of endpoint is admin-only in one case and open in two. `requireAdmin` is already wired in production (`WithAdminChecker`, `app/router/main.go`), passes everyone when the auth method is `none`, and is used by the radio CRUD writes (`handlers/subsonic/radio.go:64,131,257`) — so gating the other two is two lines each plus a test. The larger half is the policy: which `/rest` *writes* are admin-only (cover art, radio CRUD) versus per-user (stars, playlists, play queue), written down next to the compliance invariants in [`docs/agents/subsonic-api.md`](docs/agents/subsonic-api.md) instead of living implicitly in three views' `v-if`s. Note the UI already gates artist covers on `isAdmin` (`ArtistView.vue`) while `GenreDetailView` does not, so the frontend is inconsistent too.
- [ ] . No brute-force protection on login. => userauth library? 
      (confirmed absent: no rate limiting, lockout or attempt counting anywhere in `app/` — the only throttles are the outbound MusicBrainz/fanart ones in `internal/upstream`. The login flow is single-factor `RequireAny` with a **nil `AttemptStore`** (`handlers/auth/auth.go:37-46`), which the library permits for one-step policies, so nothing bounds password guessing against `POST /api/v1/auth/login`. `userauth` ships `flow/login.AttemptStore` but it exists to persist partially completed multi-factor logins, not to throttle — so this is aether's to add (per-IP + per-login backoff in front of the login handler) unless the library grows it.)

## Backend — API Surface

- [ ] Review the non-OpenSubsonic API surface — audit custom (non-Subsonic) endpoints, then move Libraries and Tasks management under an `/admin` path (e.g. `/api/admin/libraries`, `/api/admin/tasks`) so admin concerns are clearly separated from the Subsonic-compatible surface; update the frontend accordingly. **Pre-1.0 because it is a breaking URL reorg** — free now under the no-backwards-compat rule, expensive once anything depends on the paths. Not started: everything is still on one `/api/v1` subrouter (`app/router/main.go:201`), no `/admin` prefix anywhere. Note the *authorization* half already landed — `/api/v1` defaults to admin-only in both modes via the three-tier guards (`api_v1.go:56`, `proxy_auth.go`), so this is now purely about URL shape, not access control.

## Backend — OpenSubsonic Compliance

- [ ] `getUser` / `getUsers` — spec endpoints, **not routed at all** (absent from the register list in `subsonic/subsonic.go:207-269`). Clients call `getUser` to discover what the authenticated user may do — `streamRole`, `playlistRole`, `downloadRole`, `adminRole`, `coverArtRole`, … — and some disable UI or refuse to start without it. Aether already has everything the response needs: `requestOwner(r)` gives the login, `users.RoleOf` gives the vertical (the same lookup `restAdminChecker` uses), so the roles are a fixed mapping with `adminRole` the only variable one. Two decisions: the `username` a PAT-authenticated caller sees (the real login, not the tokenID virtual username), and whether `getUsers` (admin-only, lists everyone) is worth mounting at all given the users CRUD already lives on `/api/v1` — `getUser` for the caller's own record is the part clients actually need.
- [ ] XML response format — check compatibility with third-party Subsonic clients (DSub, Ultrasonic, Symfonium, etc.). XML is what several clients default to, so this gates the "third-party clients work" promise. Today `f=xml` is explicitly rejected with an error (`subsonic/subsonic.go:66-67`), so those clients fail at the first request. Note the handlers build `map[string]any` throughout (`albumToMap`, `trackToChild`, …), which does not marshal to spec-shaped XML — this needs a serialization layer, not a flag.
- [ ] expose server version correctly

## Backend — Data Integrity & Scanning

- [ ] Full scan should drop each track's existing entries and re-insert from scratch (rather than updating in place) so stale/renamed artists, albums, genres, and other derived rows don't linger when tags change
      (partial, still accurate: the *associations* are rebuilt, not merged — `Association("Artists"/"Genres").Replace(...)` for the album at `internal/scanner/reconcile.go:116,121` and for the track at `internal/store/track.go:12,15`, with `Cleanup` sweeping the orphans afterwards. What remains is album-level scalar fields: `FindOrCreateAlbum` (`store/album.go:12`) matches on `(name_norm, album_artist_norm, mb_release_id)` and returns the existing row untouched, so scalars are only ever written at create time or updated in place elsewhere.)
- [ ] Album cover on the library grid can show another album's image after metadata edits + rescan (works correctly on the album detail view). Remaining root causes:
    - `DeleteOrphanedAggregates` doesn't revalidate `CoverPath` for surviving albums (`internal/store/scan_helpers.go:68` — 15 `DELETE` statements, no album `UPDATE`).
    - Two albums sharing a directory can still both point at the same `cover.jpg` — reconcile re-detects the stored path every pass, but nothing arbitrates between albums competing for one file.
      (updated: reconcile now *always* re-detects rather than only when the stored path is unusable, so a newly written `cover.jpg` supersedes an existing `folder.jpg`. It deliberately does **not** re-detect when the current track's directory holds no art and the album's stored cover lives in a sibling directory — otherwise an art-less disc folder of a multi-disc release blanks the whole album's cover. `IsUsableCoverPath` guards exactly that case. See `internal/scanner/reconcile.go` and the multi-disc regression test in `reconcile_test.go`.)
      (one contributing cause removed: the grid/list components now build their cover URLs through `versionedCoverUrl` like the artist ones do, so an *own* cover edited in-session no longer lingers from the browser's image cache. That was a separate mechanism from the wrong-album problem, which is still open.)
- [x] Album ids are not stable across the very edits this editor performs.
      (done: `scanner.planAlbumContinuity` retags an album row in place when a
      whole album moves in one pass, so `albums.id` and `created_at` — and with
      them the manual cover in the asset store, stars, the `newest` ordering,
      the discovery feed's recency term and client-cached `/album/:id` — survive
      the editor's retags, including `identify-album` writing an MBID to a whole
      selection. Partial edits, splits, merges into an existing identity and
      identity swaps still churn by design; a merge inside one batch keeps the
      largest album's row. See
      [`docs/superpowers/specs/2026-08-18-album-identity-continuity.md`](docs/superpowers/specs/2026-08-18-album-identity-continuity.md)
      and the identity rules in `docs/agents/scanning.md`. **Still open:** the
      asset-store / image-cache sweep for albums that genuinely disappear —
      that is the item below on resource leaks, not this one.)
- [x] **Moving or renaming a music file no longer drops it from playlists or
  discards its history, star and queue position.** `scanner.planTrackContinuity`
  re-points the row at the new path when it can prove the move (equal
  `file_size` + `title`, `duration` ±1s, old path gone from disk, unambiguous
  1:1), so `tracks.id` survives and nothing cascades. `Scan` is also two-phase
  now — `preflight` validates and walks every library before any is reconciled —
  so a library whose root is unavailable or unexpectedly empty fails the scan
  before the first write instead of having its rows harvested by an
  earlier-sorting library's re-link pass. Design:
  `docs/superpowers/specs/2026-08-18-track-identity-across-moves-design.md`.
- [ ] **A move that also rewrites the tags still loses playlists, history and
  stars.** The bytes change, so `file_size` cannot anchor the match and the only
  remaining signals are `duration` + `title` + `track_number` +
  `MBRecordingID` — and `MBRecordingID` identifies a *recording*, not a file
  (the same recording on an album and a compilation shares it). Declined
  deliberately: the false-merge surface outweighs the coverage. Revisit only
  with a signal that survives a retag (an audio-stream hash).
- [ ] **A move that straddles two scan runs is unrecoverable.** By the time the
  new path appears, `Cleanup` has deleted the row. Fixing it means tombstones —
  soft-delete plus re-link on reappearance — which makes every read path
  (`/rest` browsing, playlists, search, queue) decide whether to show missing
  tracks and makes a purge flow mandatory. A feature, not a fix.
- [ ] **An unreadable subtree is still swept silently — and can now be *re-linked*.**
  `makeWalkFn` swallows per-entry errors, so a directory that becomes unreadable
  (permissions, a partial mount) looks like a deletion. Needs a way to tell
  transient from permanent per-entry failures; the root-level guards do not cover
  it — `Scan`'s two-phase preflight validates every library *root* before any
  library is reconciled, which fixes the cross-library case, but a sub-mount
  inside a present root (`/music/flac` on its own mount, unmounted, leaving an
  empty mountpoint directory) is invisible to it. Those files stat ENOENT
  indistinguishably from deleted ones, so `planTrackContinuity` can now re-point
  such a row onto a byte-identical new file elsewhere, transferring stars,
  playlist entries and history to the wrong track — a step worse than being
  swept. Deliberately not fixed by requiring a vanished row's parent directory to
  exist: reorganising a library moves whole directories, which is the primary use
  case. The `fs.ErrNotExist` narrowing (pinned by
  `TestScanDoesNotRelinkWhenTheOldDirectoryIsUnreadable`) only covers the EACCES
  flavour, not an empty mountpoint.
- [ ] **Artist and genre ids churn on a rename, taking covers, stars and cached derivatives with them.** Same root cause as the closed album item above, still open for the two remaining scanner-derived aggregates.
    - **Artists:** identity is `name_norm` alone (`internal/store/artist.go:21`, unique index `internal/model/artist.go:8`), so correcting a spelling creates a new row and `DeleteOrphanedAggregates` deletes the old one (`scan_helpers.go:75`). Lost with it: the star (`scan_helpers.go:83`), the imagecache derivative (keyed on the DB id, `subsonic/media.go:141,153`), `LastImageFetchAt` — which resets to nil, so the artist-image task re-hits the rate-limited fanart.tv / TheAudioDB — and `/artist/:id` links. **The manual cover survives only for artists with an MBID**: `artistCoverKey` prefers `MBArtistID` and falls back to the DB id (`subsonic/artists.go:15-20`). So the covers that break are exactly the unmatched artists' — the ones most likely to hold a *hand-uploaded* image, since no MBID means no auto-fetch.
    - **Genres:** identity is `name`, not even normalised (`internal/store/genre.go:19`, unique index `internal/model/genre.go:9`), and the cover keys on the DB id with no fallback (`subsonic/genres.go:48`), so a genre rename always orphans it. Milder otherwise — there is no genre star type in the cleanup list, and `/genre/:name` routes by name, so links survive.
    - **Why the album fix does not port.** `planAlbumContinuity` proves continuity from "every track this album holds is in this batch". An artist spans many albums and hundreds of tracks through two associations (`album_artists` and `track_artists`), so that test is essentially never true for an incremental scan — the guard would decline precisely when it is needed — and credits are multi-valued, so "the tracks agree on one new identity" does not even have the same shape. Artists need a different signal, most plausibly the MBID (already their durable asset key) plus explicit rename detection. Also recorded in `docs/agents/scanning.md`'s known-scanner-debt list.
    - **Three fix shapes exist and the codebase already contains one of each:** *preserve the row* (albums — `planAlbumContinuity`); *migrate on re-key* (radio — `subsonic/radio.go:223-245` computes old and new `RadioKey(streamURL)` and moves the cover so a URL edit does not orphan it); *key on content* (artist MBIDs). The remaining work is applying them, not inventing them.
    - **Cheap partial win, independent of rename detection:** stop keying genre and unmatched-artist covers on a positional id. That merges with the backlog item on DB rebuilds misattributing images — both want a non-positional key and should be scoped together.
- [ ] **`PRAGMA busy_timeout` is applied to one pooled connection out of ten, so nine have none.** `app/cmd/server.go:102-104` sets `SetMaxOpenConns(10)` and then issues `PRAGMA journal_mode=WAL` and `PRAGMA busy_timeout=5000` as two `db.Exec` calls, which run on whichever single connection the pool hands out. WAL is recorded in the database file so it sticks; **`busy_timeout` is per-connection state and does not**, so the other nine connections keep the default of 0 and return `SQLITE_BUSY` immediately instead of waiting. Fix: set it in the DSN (a `_pragma=busy_timeout(5000)` parameter on the `sqlite.Open` path) so every connection acquires it on open. Pre-existing and unrelated to album continuity, but surfaced by that review — and scans already write concurrently with `/rest` reads, with one more write transaction now taken from this pool.
- [ ] Small code-health follow-ups noted during the album-continuity review:
    - `internal/store/albumidentity.go` defines a third copy of the 500-row chunk constant (`FilterChanged` and `BulkUpdateLastSeen` in `scan_helpers.go` each have their own), and `store.IsUniqueViolation` is the third copy of the `UNIQUE constraint failed`/`duplicate` string sniff (`handlers/libraries/libraries.go:439-445`, `handlers/users/users.go:421`). The store version is the best of the three — handles `nil`, tries `gorm.ErrDuplicatedKey` first, lowercases before matching — so hoist the constant and have both handlers call it.
    - `planAlbumContinuity` runs a whole batch in one transaction while its proof is per album, so one album's DB error rolls back every other album's retag in that batch (`internal/scanner/albumcontinuity.go`). Not a correctness bug — the caller logs and degrades to the old behaviour — but the granularity does not match the proof.
- [] Big review of the whole import task


## Backend — Resource Leaks

- [ ] Nothing ever evicts from the image cache — `Cache.Delete(kind, key)` exists (`internal/imagecache/imagecache.go:130`) and still has **zero callers of any kind, production or test**, so deleting an entity leaves its derivative directory behind forever. No prune task exists in `app/tasks` either. Superseded *fingerprints* of a still-live entry are already swept on rebuild (`Cache.sweep`), so this is only about entities that go away. Wire `Delete` in alongside the existing `assets.Delete` calls: `subsonic/artists.go:67`, `subsonic/genres.go:56`, `subsonic/playlists.go:330,355`, `subsonic/radio.go:226-274`, plus album deletion in the scanner's orphan cleanup (`store.DeleteOrphanedAggregates`, which has no assetstore counterpart today). One trap:
    - **Editor thumbnails (`kind: "editor"`) can't be swept this way at all** — `pictureThumbKey` keys them by a hash of the file path or the image bytes (`metadata/pictures.go:419`), which is not derivable from an entity id. They need either a different key scheme or an age-based sweep, so a periodic prune task in `app/tasks` may be the better shape for the whole problem than per-deletion hooks. **The editor-thumbnail half is the 1.0-critical part** — those grow on every normal use of the metadata editor, not just on deletion. The per-entity wiring could slip to a later release if needed.

## Frontend 

- [x] Create an app icon / logo. Browser/PWA side: `zarf/icon/web/` holds the cleaned web sources (rounded, square, maskable) derived from the `icon2.svg` master, `make icons` (`zarf/icon/render.sh`) renders them into `webui/public/` (`icon.svg`, `favicon.ico`, `apple-touch-icon.png`, `icon-{192,512}.png`, `icon-maskable-{192,512}.png`), and `index.html` + the manifest link them all. In-app side: the diamond rendition (`assets/aether-mark.svg` via `components/common/BrandMark.vue`) replaced the `◈` placeholder in the sidebar and the mobile drawer and now sits beside the wordmark on the login card. The wordmark itself **stays styled text** — only the mark is artwork.
- [] library also shows songs additionally to albums and artists
- [] impelemt radio mode queue => keep playing based on same type/taste
  - if i just listened to an album put the next album of the same artists
    - if the artist has no more albums, jup to the next artist with similar tags
- [] now playing should alow you to navigate to artists similar like album
- [] player should propageta error if server not accessible

## Frontend - mobile


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

## Backend — Cleanup

- [ ] Extract a shared helper for the three cover-art extension handlers. `updateArtist` (`handlers/subsonic/artists.go`), `updateGenre` (`genres.go`) and `updateAlbum` (`albums.go`) differ in five places — the endpoint name in one error string, the `decodeID` kind, the store lookup, the `assetstore.Kind`, and the key derivation (`artistCoverKey`'s MBID-or-DB-id logic vs `strconv.FormatUint`) — while everything around them is character-identical: multipart guard, byte cap, `id` presence, kind check, 404, `readCoverFile`, the put/clear switch, `writeResponse`. Roughly 40 duplicated lines. Shape: a helper taking `(endpointName, idKind string, kind assetstore.Kind, resolve func(uint) (writeKey string, clearKeys []string, err error))`, where the artist's extra DB-id slot clear fits `clearKeys` naturally. `dupl` flagged the albums/genres pair when `albums.go` was added and stopped firing once `requireAdmin` diverged them, so nothing is currently suppressed — this is a real DRY item, not a lint workaround. While there: `maxRadioRequestBytes` / `radioMultipartMemory` / `radioCoverMaxBytes` are now imported by four non-radio handlers and deserve cover-neutral names.
- [ ] `assetstore`'s named-entry API is dead code. `GetNamed`, `GetEntryNamed`, `PutManualNamed` and `DeleteNamed` (`internal/assetstore/assetstore.go`) have no production callers — only the package's own tests. The feature existed solely for the metadata editor's removed `db` picture slot, which stored per-type entries (`back`, `disc`, `booklet`, …) that nothing ever read. Note `Get`/`PutManual` are thin wrappers over the named variants with `DefaultName`, so the collapse is mechanical rather than a rewrite. Deleting them and their tests keeps the store API honest, the same rationale that retired `SetAlbumCoverPath` / `SetTrackHasEmbeddedCover` / `GetAlbumByTrackPath` / `GetAlbumByTrackDir`; check `internal/assetstore` stays above the 70% coverage gate afterwards.
- [ ] `internal/scanner/reconcile.go` states the cover-detection rule twice — the full comment above `dir := filepath.Dir(...)` and a near-verbatim repeat of its middle immediately above the `if`. Deleting the second block is the whole fix.

## Backend — Library

- [ ] Add statistics in backend library: e.g. albums, artists, songs, genres, disk space used
- [ ] rework kibraries as virual filtered items instead of mapped to filesystem
- [ ] Guard a vanished sub-tree *inside* a present library root — a mounted subdirectory that is not attached leaves a hollow directory whose files are indistinguishable from deletions, so those tracks are swept **and can be re-linked onto a byte-identical file elsewhere**, moving their stars, playlist entries and history onto the wrong file with no error. Full analysis, the assumption that defers it, and five candidate fixes with their objections: [`docs/architecture/caveats.md`](docs/architecture/caveats.md#vanished-sub-trees-inside-a-present-library-root). **Deferred on an explicit assumption: mounts happen at a library root (`mount /music/library1`), never at a directory inside a library** — when the mount *is* the root, phase 1's guards already fail the scan atomically. Revisit the moment a sub-mount, bind mount, junction or autofs subdirectory appears under a library root. Favoured direction is the portable "volume tripwire" (generalise the existing zero-files guard from *all* to *too many*); note soft delete fixes only the loss half, not the misattribution half, because re-linking happens before any deletion.

## Frontend — Music Browsing & Features

- [ ] Artists tab in Library — replace the grid-of-artist-cards + drill-down into a single scrollable page grouped by artist: one header per artist (alphabetical), followed by that artist's albums sorted by year; no per-artist navigation step
      (partial, still accurate: Library has an Artists tab with grid and virtualized list views + alphabet rail — `components/library/ArtistGrid.vue`/`ArtistListView.vue`, wired at `LibraryView.vue:206-211` — but each card is a `router-link` to the `artist` route (`ArtistCard.vue:33`), so it's still artists-that-navigate, not the grouped artist-header + albums layout)
- [ ] Spotify-style hover selection in song list — on row hover, show a checkbox next to the duration for multi-select
      (a checkbox-in-the-index-cell pattern already exists in queue *edit* mode — `components/layout/QueueRow.vue:68-82` — but the browsing song lists (`components/library/AlbumTrackRow.vue`, `GenreTrackRow.vue`) select by plain/ctrl/shift click with no hover affordance; they only tint the row on `:hover`)
- [ ] Album cover drag and drop in the album view
- [ ] Album cover Remove has no way to know whether there is anything to remove. `AlbumView`'s hero Remove clears aether's managed cover via `updateAlbum`'s `coverClear`, but most albums are served from folder art or embedded tags instead — so Remove → Save deletes a non-existent asset entry and the old cover reappears. Currently mitigated only by helper text spelling out the semantics; `HeroHeader` already has a `coverRemovable` prop for suppressing the affordance, and `ArtistView` drives it from an image-source query (`/api/v1/artists/{id}/image-source`, surfaced as `canRemoveImage`). The album equivalent needs `/rest` to report whether the served cover is aether-managed — an OpenSubsonic extension field or small endpoint, not an `/api/v1` route, since album covers are music functionality. Same gap exists for genres.
- [ ] Better genre handling — **needs scoping before it can be planned**
- [ ] Search should also return genres
- [ ] Playlist edit is not a nice experience for now — **needs scoping**: name the specific interactions that are wrong (reorder? multi-remove? add-from-search?) before this can be estimated
- [ ] Add filter to artist / album etc 

## Frontend — Metadata editor

- [ ] Make the path that is visible in the top (on the side of choose folder) a fast way to load libraries
- [ ] Make it easier / faster to load a folder
- [ ] Check if we can add comments to metadata as part of the standard (e.g. the unreleased Alesha Dixon "Fool 4 U I Love")
- [ ] When identifying multiple songs, allow staging only a subset of the changes (e.g. genre only)
- [ ] the library selector should not be a drop donw as it takes too many clicks
- [ ] the folder selector dialog should be bigger and have a filter option
- [ ] once you have selected the album of one artist navigating to another album folder should be easy ( use nav bar on top)
- [ ] in attached pictures, front cover should always be an option even if no picture is detected at all
- [ ] generated cover files are not browsable by samba
- [ ] in artist selection dialog, check if there might be more pictures for the same artist
- [ ] when identifying albums sometiemes track position os wrong, can we improve that
- [ ] for images i want some metadata info: size, dimensions and format, both for what is stored as well as what is found
- [ ] in raw metadata edit, after save return to the non raw view
- [ ] identify album selections should print more details in the header
- [ ] 431 Request Header Fields Too Large
  /api/v1/metadata/pictures?library_id=1&path=Apocaliptica%2F2001_Cult++-+Special+edition%2FCD1&type=Front+Cover&slot=embedded&paths=Apocaliptica%2F2001_Cult++-+Special+edition%2FCD1%2F01+-+Path.mp3&paths=Apocaliptica%2F2001_Cult++-+Special+edition%2FCD1%2F02+-+Struggle.mp3&paths=Apocaliptica%2F2001_Cult++-+Special+edition%2FCD1%2F03+-+Romance.mp3&paths=Apocaliptica%2F2001_Cult++-+Special+edition%2FCD1%2F04+-+Pray!.mp3&paths=Apocaliptica%2F2001_Cult++-+Special+edition%2FCD1%2F05+-+In+Memoriam.mp3&paths=Apocaliptica%2F2001_Cult++-+Special+edition%2FCD1%2F06+-+Hyperventilation.mp3&paths=Apocaliptica%2F2001_Cult++-+Special+edition%2FCD1%2F07+-+Beyont+Time.mp3&paths=Apocaliptica%2F2001_Cult++-+Special+edition%2FCD1%2F08+-+Hope.mp3&paths=Apocalipti
- [] Unscanned music has no address at all, has an impact on album art
- [x] Drop DB dependency in metadata editor => metadata editor should only dealt with editin file data, album covers and other stored in the DB should be handed outside of metadata editor or with extra API
      (done: the editor's picture slots are now `embedded` + `folder` only — the `db` slot, both direct DB pokes (`SetAlbumCoverPath`, `SetTrackHasEmbeddedCover`) and the `Assets` dependency are gone from `handlers/metadata`, whose only remaining `Store` calls are `GetLibrary` reads. Manual album covers were rehomed to the `updateAlbum` / `albumCoverArt` OpenSubsonic extension (`handlers/subsonic/albums.go`), driven from the admin-gated hero cover editor on `AlbumView`. The editor's post-write rescan stays — that is index sync, not metadata-in-DB editing. Reconcile now owns `album.CoverPath` outright, which is what made the pokes unnecessary; see `docs/agents/scanning.md`)
- [] alignt v1 API with open api speck

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

- [x] **Rebuilding the DB without also wiping the asset store attaches stored images to the wrong entities.**
    - **Shipped:** asset keys are derived from natural identity (`internal/assetkey`)
      — the same tuple the database uses as its unique index — rather than
      autoincrement ids. Albums hash `idx_album_identity`; matched artists keep
      literal MBID keys (shared with the auto-fetcher); unmatched artists hash
      `name_norm`; genres hash the raw name (no normalisation, because genre
      names are stored and matched exactly); playlists key on a new `uuid` column
      (user-owned rather than tag-derived); radio hashes the stream URL. The
      imagecache uses the same key as its source asset. A rebuild re-attaches
      every image to the entity that means the same thing — no misattribution, no
      manifest, no reconciliation pass. Three re-key hooks carry images when
      continuity moves the row: the album planner when it retags, an artist
      gaining an MBID, and a radio stream-URL edit. All tolerate failure without
      failing the scan or request. See `docs/agents/subsonic-api.md` (key scheme)
      and `docs/agents/scanning.md` (re-key hooks).
    - **Operational note:** changing the keys strands existing files under
      `data/metadata/` at their old keys. Under the no-backwards-compatibility
      rule, delete that directory and re-upload. A bridge would contradict the
      project rule.
    - **Still open:** artist and genre **rename** continuity — a renamed artist
      is a different artist to the data model, and no continuity proof exists
      (the album planner's proof does not transfer); star loss on aggregate churn
      (row churn, not a key problem); imagecache eviction (the item below).

---

# Won't implement

- **Sharing and Chat.** Sharing exists to hand out public unauthenticated links (`/share.php?id=…&secret=…` + an HTML landing page) that bypass auth by design; Chat is a global message wall with no rooms or delivery, vestigial in the ecosystem and pointless on a single-user server. Don't add them, and don't file them as gaps again
