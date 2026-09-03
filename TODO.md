<!-- todo:guide — managed by todo; this block is rewritten on save. Docs: https://github.com/andresbott/todo
This file is a todo list managed by "todo", a terminal TODO app:
https://github.com/andresbott/todo

todo watches this file and reloads it automatically when it changes on disk, so
you — human or agent — can edit it directly in any editor. Keep to this format
so todo can parse what you write:

  # Heading           Headings ("#" to "######") are categories; they nest by
                      heading level.
  - [ ] Open task     A "- [ ]" line is an open task; "- [x]" marks it done.
  - [/] In progress   "- [/]" flags a task in progress, "- [>]" defers it.
  - [x] Done task     Tasks must live under a category heading.
    - [ ] Subtask     Indent by two spaces to nest a subtask under a task.
    Description text  An indented, non-checkbox line is the task's description.

Notes for editors:
- Text above the first heading (this block included) is preserved on save.
- todo rewrites the file into the canonical form above on every change, so any
  other free-form markdown placed between items is not kept.
-->

# 1.0

## Features

- [ ] Proper playlist editing
- [ ] multi user playlists
- [ ] detach scaned folders from libraries
  use metaadata queries insetd of folders for sources of songs

## Security & Authentication

## Backend

### Backend — API Surface

- [ ] Extend the OpenAPI response-contract test to the upstream-mocked and still-uncovered endpoints
  `app/router/openapi_response_contract_test.go`'s kin-openapi response-contract test (the `TestContract*` functions) validates real handler responses against `docs/openapi/aether-v1.yaml`'s schemas, but only for endpoints reachable with just an in-memory store — bootstrap, auth/tokens, libraries, users, tasks. Closing the gap REQUIRES mocking the radio-browser and MusicBrainz upstreams (`internal/radiobrowser`, `internal/artistimage.MusicBrainzSearch`) so `searchRadioStations`, `getRadioFavicon`, `searchMusicBrainzArtists`, `searchMusicBrainzReleases`, `getReleaseGroupGenres`, `listArtistImageCandidates` and `setArtistImageFromSearch` can be asserted without hitting the real internet. Still uncovered beyond that: fixtures for identify/identify-album audio-fingerprint identification (needs sample audio plus a fake AcoustID backend), the whole `metadata` group (folders/tracks browsing, pictures inventory/apply/removals, artist-folder/artist-image), binary responses (image bytes from `getPictureImage`/`getArtistImage`/`getRadioFavicon` — schema validation only applies to their JSON error paths), and the update/delete/patch mutation variants (`updateTracks`, `clearPictureSelection`, `deleteArtistImage`, `deleteToken`, `deleteUser`, `deleteLibrary`, `patchTaskSchedule`, `deleteTaskSchedule`, `cancelTaskExecution`) whose response shapes are never exercised today.

### Backend — OpenSubsonic Compliance

- [ ] Review the non-OpenSubsonic API surface
  Audit custom (non-Subsonic) endpoints, then move Libraries and Tasks management under an `/admin` path (e.g. `/api/admin/libraries`, `/api/admin/tasks`) so admin concerns are clearly separated from the Subsonic-compatible surface; update the frontend accordingly. Pre-1.0 because it is a breaking URL reorg — free now under the no-backwards-compat rule, expensive once anything depends on the paths. Not started: everything is still on one `/api/v1` subrouter (`app/router/main.go:201`), no `/admin` prefix anywhere. Note the authorization half already landed — `/api/v1` defaults to admin-only in both modes via the three-tier guards (`api_v1.go:56`, `proxy_auth.go`), so this is now purely about URL shape, not access control.
- [ ] XML response format for third-party clients
  Check compatibility with third-party Subsonic clients (DSub, Ultrasonic, Symfonium, etc.). XML is what several clients default to, so this gates the "third-party clients work" promise. Today `f=xml` is explicitly rejected with an error (`subsonic/subsonic.go:66-67`), so those clients fail at the first request. Note the handlers build `map[string]any` throughout (`albumToMap`, `trackToChild`, …), which does not marshal to spec-shaped XML — this needs a serialization layer, not a flag.

### Backend — Data Integrity & Scanning

- [ ] Document the audio-hash format limit in the user-facing docs
  The eight formats `libs/audiohash` does not cover are now a deliberate non-goal (see "Won't implement" → "Audio-hash coverage for the remaining eight audio formats"), and that decision is **user-visible**: on a library of FLAC/MP3/M4A/WAV/AIFF/Ogg/Opus, an external tagger that retags and re-files in one pass (Picard, beets) keeps every track's playlists, stars, play history and queue position; on a WMA, APE, WavPack, raw AAC, Matroska/WebM, TTA or DSF library the same operation silently loses them. Today that is written down only in agent-facing docs (`docs/agents/scanning.md`, `planTrackContinuity`'s doc comment), which no user reads. It needs a home in `README.md`: which formats survive an external retag-and-move with their library data intact, and that the rest fall back to a size-and-title heuristic that a retag defeats. `README.md` has no limitations section today, so this either adds one or extends "Features" with the honest caveat — decide which when writing it. Worth stating the same way the auth caveat already is: plainly, near the top, not buried.
  Same doc owes the operator **one full scan after any release that widens hash coverage**, and says why: the move proof needs the *old* row to already carry a hash, an incremental scan only re-reads files that changed, and a release that adds a format changes the server, not the files — so newly-supported files stay unarmed until something force-reads them. Widening coverage is therefore inert on an existing library until that scan runs. Applies to the WAV/AIFF/Ogg/Opus release specifically, and to any future one.
- [x] A row that has never been hashed stays unhashed until a full scan
  **Accepted, not fixed:** "run one full scan" is the answer, and the only obligation left is telling the user so — folded into the user-facing-docs item above. The audio-hash move proof needs the **old** row to already carry a hash, and an incremental scan only reads the files it thinks changed, so any row never successfully hashed — indexed before the column existed, in a format only supported by a later release, or declined once for a structural reason — keeps `""` and cannot be re-linked however many incremental scans run. A full scan arms every file `libs/audiohash` covers (the other eight never arm; see "Won't implement"). Revisit only if the manual full scan proves to be a real burden, in which case the fix is an opportunistic backfill — hash rows with an empty `audio_hash` even on an incremental scan, bounded and one time per file — designed in `docs/agents/scanning.md` under known scanner debt. Also noted in `planTrackContinuity`'s doc comment.
- [x] Genre ids churn on rename, orphaning covers
  Same root cause as the closed album item above. The artist half is now won't-implement — see "Stable artist identity across a rename (artist id churn)" — so what stays in scope here is genres plus the positional-id cover key, which is also the piece that shrinks the artist case's blast radius without needing rename detection.
  - [x] Genres: raw-`name` identity, DB-id cover with no fallback
    Genres: identity is `name`, not even normalised (`internal/store/genre.go:19`, unique index `internal/model/genre.go:9`), and the cover keys on the DB id with no fallback (`subsonic/genres.go:48`), so a genre rename always orphans it. Milder otherwise — there is no genre star type in the cleanup list, and `/genre/:name` routes by name, so links survive.
  - [x] Cheap partial win: stop keying covers on a positional id
    Cheap partial win, independent of rename detection: stop keying genre and unmatched-artist covers on a positional id. **Made more urgent by the drop-and-recreate policy, not less:** while there are no consumers, "drop the DB and rescan" is the accepted answer to every schema change, and a rescan reassigns autoincrement ids in whatever order it happens to insert — so a hand-uploaded genre or unmatched-artist cover comes back attached to a *different* entity, silently. The escape hatch is what triggers the bug. A non-positional key fixes the rename case and the rebuild case at once; scope them together. (Supersedes the old "DB rebuilds misattributing images" backlog item, which no longer exists separately.) This is also the only part of the won't-implement artist item that stays in scope: with a content-derived key, an artist rename still loses the star, `LastImageFetchAt` and the `/artist/:id` link, but no longer orphans or misattributes the cover.
  - [x] Three fix shapes already exist in the codebase
    Three fix shapes exist and the codebase already contains one of each: preserve the row (albums — `planAlbumContinuity`); migrate on re-key (radio — `subsonic/radio.go:230-243` computes old and new `RadioKey(streamURL)` and moves the cover so a URL edit does not orphan it); key on content (artist MBIDs). The remaining work is applying them, not inventing them.
- [x] Small code-health follow-ups noted during the album-continuity review
  - [x] `planAlbumContinuity` batch transaction vs per-album proof
    **Fixed.** `planAlbumContinuity` was running a whole batch in one transaction while its proof is per album, so one album's DB error rolled back every other album's retag in that batch. Restructured as snapshot → plan → apply: `readAlbumSnapshot` reads the batch state in one read transaction, `planAlbumRetags` is a pure function over it (no DB, so every leg of the proof — split, disagreeing batch, unchanged identity, merge survivor, ordering — is unit-testable), and `applyAlbumRetag` commits **one album per transaction**, re-proving the plan against the live rows inside the writing transaction. That re-check is what makes the read/write split safe: the counts *are* the proof, and acting on a stale one would rename an album that has since split. A per-album failure is logged and skipped, so it degrades only that album to a new id — matching `reconcile`'s own per-track loop. Also drops the write-transaction-held-across-a-whole-library, so a scan no longer blocks the API's writers for the duration. Documented in `docs/agents/scanning.md`.
- [ ] Big review of the whole import task

### Backend — Resource Leaks

- [ ] Nothing ever evicts from the image cache
  `Cache.Delete(kind, key)` exists (`internal/imagecache/imagecache.go:130`) and still has zero callers of any kind, production or test, so deleting an entity leaves its derivative directory behind forever. No prune task exists in `app/tasks` either. Superseded fingerprints of a still-live entry are already swept on rebuild (`Cache.sweep`), so this is only about entities that go away. Wire `Delete` in alongside the existing `assets.Delete` calls: `subsonic/artists.go:55,59`, `subsonic/genres.go:56`, `subsonic/playlists.go:348,423`, `subsonic/radio.go:223,226,228,275`, plus album deletion in the scanner's orphan cleanup (`store.DeleteOrphanedAggregates`, which has no assetstore counterpart today). One trap remains, reduced but not removed:
  - [ ] Artist cache eviction must handle both key slots
    The artist cache key uses the MBID-preferring derivation (`artistCoverKey` in `subsonic/artists.go`) and deliberately does not track which of the two slots the image came from (manual upload vs. auto-fetched). An artist later gaining an MBID therefore orphans its old derivatives keyed on the name-hash slot — a leak, not a misattribution, since no other artist can inherit them. The exception is documented in `subsonic-api.md`. Artist-specific eviction logic must handle both keys.
  - [ ] Editor thumbnails can't be swept by entity id (1.0-critical)
    Editor thumbnails (`kind: "editor"`) can't be swept this way at all — `pictureThumbKey` keys them by a hash of the file path or the image bytes (`metadata/pictures.go:419`), which is not derivable from an entity id. They need either a different key scheme or an age-based sweep, so a periodic prune task in `app/tasks` may be the better shape for the whole problem than per-deletion hooks. The editor-thumbnail half is the 1.0-critical part — those grow on every normal use of the metadata editor, not just on deletion. The per-entity wiring could slip to a later release if needed.

## Frontend

### Frontend - mobile

### Frontend — Metadata editor

- [ ] Surface field-level validation errors (RFC 9457 `errors[]`) in the editor forms — the backend now returns `422` `ValidationProblem` with `errors[]` (`{pointer, detail}`) and the SPA type carries it, but no view renders it; the UI still shows only the top-level `detail`/`title`.
- [ ] When identifying albums sometimes the track position is wrong — can we improve that?

# Future releases

## Frontend - Metadata editor

- [ ] Explore exposing Lyrics as an editable field
  Already read and stored (`tags.Metadata.Lyrics` → `track.Lyrics`, `scanner/reconcile.go:200`) but the editor can't set it, so a mis-tagged lyric is unfixable in-app. Only the write side is missing: `metadataedit.Track`/`Patch`/`BuildTagMap` (`taglib.Lyrics`), `managedTagKeys`, and the frontend `Track`/`PatchFields`/`MANAGED_TAG_KEYS` + `EditPanel`.
- [ ] Explore exposing Release type as an editable field
  Same shape as lyrics: read and stored (`tags.Metadata.ReleaseType` → `album.ReleaseType`, `scanner/reconcile.go:128`) but not editable, so album/EP/single/compilation classification can't be corrected in the UI. `taglib.ReleaseType` (`RELEASETYPE`).
- [ ] Explore sort keys as editable fields
  MusicBrainz-style sort tags — ArtistSort/AlbumSort/TitleSort/AlbumArtistSort/ComposerSort (`TSOP`/`TSOA`/`TSOT`/`TSO2`/`TSOC`) — control browse ordering, which is aether's whole job. Bigger slice than the two above: not read today, so it needs scanner read + a store column *and* the editor field, not just the write side.
- [ ] Improve track position when identifying albums
  Track position is sometimes wrong when identifying albums — can we improve it?

## Backend — Multi-user

- [ ] Multi-user — per-user scoping of queue, stars, playlists, and history
  Landed via session identity (owner-keyed schemas). PAT layer landed (replaces the interim cookie resolver and unblocks third-party clients). Remaining work: re-key the owner columns on `User.ID` instead of the login string, then re-enable rename. The caveat is now contained, not live: `owner` is still the LOGIN string (`patIdentityResolver` returns `info.LoginID`, `app/router/main.go:133`), so a rename would orphan queue/stars/playlists/history — but renaming is refused with 400 (`errRenameUnsupported`, `handlers/users/users.go`) and `UserDialog.vue` shows the login read-only, so nothing can trigger the orphaning. Lifting the refusal is the last step of this item, not a prerequisite.
- [ ] Favorites schema — superseded by `Owner` on `starred_items`
  Junction table keyed `(owner, item_type, item_id)` with unique index `idx_starred_item`. A per-type split (`album_stars`, `artist_stars`, `track_stars`) with `(user_id, item_id)` PKs and cascade deletes is optional later if there's a concrete benefit; the current schema works. **Decide now, not "later":** with no consumers, a schema change costs nothing but dropping the DB — that is the only window in which this is free, and after 1.0 it needs a real migration for a change that is optional by its own admission. So either commit to the split while it is cheap or close this as won't-do; "later" is the one answer that gets expensive.

## Backend — OpenSubsonic Completeness

- [ ] Transcoding for formats browsers can't play
  Identify formats browsers can't play natively and add FFmpeg transcoding. Not a 1.0 gate: browsers natively handle FLAC/MP3/OGG/Opus/AAC, so this is for exotic formats and bandwidth-limited remote listening, and it's a whole subsystem (ffmpeg dependency, cache, per-client profiles).
- [ ] CUE sheet support
  Single audio file + `.cue` sidecar (DJ mixes, EAC FLAC/APE rips). Exposed as regular per-track albums with seamless web-UI playback; virtual tracks (file + time region), scanner pairing, ffmpeg remux slicing for third-party clients, OpenSubsonic extension for region offsets. Full assessment: `docs/cue-playing.md`.
- [ ] setRating persistence — add rating column to tracks/albums when needed
  Confirmed unimplemented: `setRating` is a two-line `writeResponse(w, nil)` (`subsonic/annotation.go:103-105`) and no model carries a rating field. The handler answers OK for a write it drops — if this stays unimplemented through 1.0, that silent lie is the part worth revisiting.
- [ ] getArtistInfo / getAlbumInfo
  External metadata (MusicBrainz bios, similar artists).
- [ ] getTopSongs / getSimilarSongs
  Requires external data or play-history analysis.
- [ ] Podcasts, Jukebox — not in scope for this pass
  Internet Radio has since been implemented: full CRUD under `/rest/` + Radio UI.
- [ ] Bookmarks — not needed for resume
  `savePlayQueue`'s `position` already covers resume. Only worth adding for per-track offsets that survive a queue replacement (audiobooks, long sets). If added, the play queue stays the single source of truth for the current track's position — do not write both on the same tick.

## Backend — Cleanup

- [ ] Extract a shared helper for the three cover-art extension handlers
  `updateArtist` (`handlers/subsonic/artists.go`), `updateGenre` (`genres.go`) and `updateAlbum` (`albums.go`) differ in five places — the endpoint name in one error string, the `decodeID` kind, the store lookup, the `assetstore.Kind`, and the key derivation (`artistCoverKey`'s MBID-or-DB-id logic vs `strconv.FormatUint`) — while everything around them is character-identical: multipart guard, byte cap, `id` presence, kind check, 404, `readCoverFile`, the put/clear switch, `writeResponse`. Roughly 40 duplicated lines. Shape: a helper taking `(endpointName, idKind string, kind assetstore.Kind, resolve func(uint) (writeKey string, clearKeys []string, err error))`, where the artist's extra DB-id slot clear fits `clearKeys` naturally. `dupl` flagged the albums/genres pair when `albums.go` was added and stopped firing once `requireAdmin` diverged them, so nothing is currently suppressed — this is a real DRY item, not a lint workaround. While there: `maxRadioRequestBytes` / `radioMultipartMemory` / `radioCoverMaxBytes` are now imported by four non-radio handlers and deserve cover-neutral names.

## Backend — Library

- [ ] Add library statistics
  e.g. albums, artists, songs, genres, disk space used.
- [ ] Rework libraries: scan folders + metadata-composed
  Stop filtering filesystem concerns into the app. Keep a list of folders to scan (config- or DB-stored); compose libraries from track metadata rather than mapping each library 1:1 to a filesystem path.

## Frontend — Music Browsing & Features

- [ ] Artists tab — grouped page instead of card grid + drill-down
  Replace the grid-of-artist-cards + drill-down with a single scrollable page grouped by artist. One header per artist (alphabetical), followed by that artist's albums sorted by year; no per-artist navigation step. Partial, still accurate: Library has an Artists tab with grid and virtualized list views + alphabet rail — `components/library/ArtistGrid.vue`/`ArtistListView.vue`, wired at `LibraryView.vue:206-211` — but each card is a `router-link` to the `artist` route (`ArtistCard.vue:33`), so it's still artists-that-navigate, not the grouped artist-header + albums layout.
- [ ] Hover multi-select checkbox in song lists
  Spotify-style: on row hover, show a checkbox next to the duration for multi-select. A checkbox-in-the-index-cell pattern already exists in queue edit mode — `components/layout/QueueRow.vue:68-82` — but the browsing song lists (`components/library/AlbumTrackRow.vue`, `GenreTrackRow.vue`) select by plain/ctrl/shift click with no hover affordance; they only tint the row on `:hover`.
- [ ] Album cover drag and drop in the album view
- [ ] Album cover Remove can't tell if there's anything to remove
  `AlbumView`'s hero Remove clears aether's managed cover via `updateAlbum`'s `coverClear`, but most albums are served from folder art or embedded tags instead — so Remove → Save deletes a non-existent asset entry and the old cover reappears. Currently mitigated only by helper text spelling out the semantics; `HeroHeader` already has a `coverRemovable` prop for suppressing the affordance, and `ArtistView` drives it from an image-source query (`/api/v1/artists/{id}/image-source`, surfaced as `canRemoveImage`). The album equivalent needs `/rest` to report whether the served cover is aether-managed — an OpenSubsonic extension field or small endpoint, not an `/api/v1` route, since album covers are music functionality. Same gap exists for genres.
- [ ] Better genre handling — needs scoping before it can be planned
- [ ] Playlist edit is not a nice experience for now — needs scoping
  Name the specific interactions that are wrong (reorder? multi-remove? add-from-search?) before this can be estimated.
- [>] Add filter to artist / album etc

## Frontend — Player & Controls

- [ ] Implement radio mode queue => keep playing based on same type/taste
  - [ ] If I just listened to an album, put the next album of the same artist
    - [ ] If the artist has no more albums, jump to the next artist with similar tags
- [ ] Jukebox functionality — use the web UI only to control the audio
- [ ] Relay — like jukebox, but loading songs from another instance

## Frontend — Layout

## Metadata & External Integrations

- [ ] Last.fm scrobbling
  Forward each play to the user's Last.fm account so listening history lives off-box, plus the `track.love`/`artist.getInfo`/`album.getInfo` features that ride the same API key. Nothing outbound exists today (confirmed: no `lastfm`/`listenbrainz` reference anywhere in Go) — the local side only: the browser applies Last.fm's own 50%/4min/30s rule (`usePlayer.ts:53`), calls `/rest/scrobble`, and the server appends to `play_history` (`subsonic/annotation.go:62`). No client, credentials, config, or retry queue. Two prerequisites: the `submission` parameter is currently ignored, so now-playing pings are recorded as completed plays; and there is no user/settings table to hold a session key. Consider ListenBrainz first — same off-box history and stats, one unsigned POST with a token, no signed handshake or app registration.
- [ ] DLNA / UPnP endpoint
  Expose the library as a DLNA MediaServer so devices on the LAN (TVs, receivers, stock media players) can browse and stream without the Subsonic client.

# Backlog — needs investigation

- [ ] Support track comments
  COMMENT is a standard, ubiquitous tag (ID3 `COMM`, Vorbis `COMMENT`, MP4 `©cmt`), so exposing it is feasible — but unlike the Lyrics/Release type items it is not a clean promotion, because our tag library flattens comments. `go-taglib` (a binding over TagLib) reads every tag through TagLib's "property map", which represents a comment as plain text under a single `COMMENT` key. That drops the comment's language sub-field and its description/label, and cannot distinguish the several distinctly-labelled comments a file may hold — apps stash private data in labelled comment frames (e.g. iTunes' `iTunNORM`). So a naive single "Comment" text box could show/edit the wrong comment, silently overwrite or erase that hidden app data, or collapse multiple comments into one on save. This is a limitation of TagLib's property-map abstraction (upstream, inherited by the fork), not the file format and not aether: the library offers no per-frame API, only the flat map plus a delete-only "unsupported frames" list, so a faithful comment editor would need a different or extended library. A safe minimal design is to only ever read/write the unlabelled "main" comment and leave labelled frames untouched — but that is a deliberate policy call. Investigated 2026-08-23.
- [ ] Radio stations not saved in the play queue
  Radio stations aren't persisted in the play queue across sessions / devices. Investigated 2026-08-11, not fixed; needs a direction decision before any code.
  - [ ] Root cause: radio enters the queue as a synthetic non-`tr-` `Song`
    Root cause: a station enters the queue as a synthetic `Song` (`webui/src/utils/radioSong.ts:8` — `id: radio-<name>` plus a `streamUrl`, deliberately not the real `rs-<n>`). `useQueueSync.pushQueue` (`webui/src/composables/useQueueSync.ts:50`) sends `queue.map(s => s.id)` verbatim, and the backend's `decodeTrackIDs` (`subsonic/playlists.go:249`) silently drops every non-`tr-` id. `model.PlayQueueEntry.TrackID uint` is track-only anyway, and restore rebuilds entries from `model.Track` via `starredSongList`, which emits no `streamUrl` — so nothing radio-shaped can round-trip even if it were stored.
  - [ ] Two collateral bugs, worse than the missing station
    Two collateral bugs are worse than the missing station, and are worth fixing whichever direction wins: (a) the client's `currentIndex` counts the dropped entry, so a station before the playing track makes `currentIndex >= len(trackIDs)` and `savePlayQueueByIndex` answers error 10 (`subsonic/playqueue.go:73`) — discarding the whole save, tracks included; a station after it nominates the wrong current track. (b) A radio-only queue decodes to zero ids, which routes to `clearSavedQueue` (`playqueue.go:58`) and deletes the other device's saved session. Minor: `usePlayer.ts:74` scrobbles `radio-<name>`, which answers `invalid id` (log noise only).
  - [ ] OpenSubsonic has no mechanism for queuing radio
    OpenSubsonic has no mechanism for this: `savePlayQueue`'s `id`/`current` are defined strictly as song ids; `PlayQueue.entry` is an array of `Child` ("the list of songs in the queue") with a MUST that `current` be "a valid id in the list of songs"; `Child.type` ∈ `{music, podcast, audiobook, video}` and `mediaType` ∈ `{song, album, artist}` — no stream/radio value, no `streamUrl` field. Stations are a separate entity with their own CRUD: the spec's model is that a station is something you play, not something you queue. The extension registry (11 entries) has nothing for it — podcast episodes got a dedicated extension, radio-in-queue never did — so there is no upstream extension to adopt, only one to author.
  - [ ] Options: (A) follow spec, or (B) author a `radioQueue` extension
    Options: (A) follow the spec — radio doesn't persist cross-device, but the client strips non-track entries and recomputes `currentIndex` over the survivors so (a)/(b) become impossible. Pure bugfix, no spec deviation, no schema change. (B) author a `radioQueue` extension — polymorphic `PlayQueueEntry` (kind + ref), `rs-` ids accepted on the `ByIndex` variant only, spec-shaped `getPlayQueue` still filtered to songs. Schema drop, and deviate-first-upstream-later. **The "schema drop" half of B's cost is free right now** — with no consumers, dropping and recreating the DB is the accepted answer to any schema change, so B is not "spec-pure vs. expensive" but simply spec-pure vs. not; weigh it on the deviation alone, and decide before 1.0 makes the schema half real. Either way fix the two collateral bugs above first: they are wrong under both options.
  - [ ] Trap for (B): `stream` ignores the id kind (`rs-3` serves track 3)
    Trap for (B): `stream` discards the id kind (`_, id, err := decodeID(...)`, `subsonic/media.go:29`), so `stream?id=rs-3` today serves track 3's file. A latent bug a radio-in-queue design walks straight into — worth fixing on its own regardless.

# Won't implement

- [>] Recovering a move that straddles two scan runs (tombstones / soft delete)
  Move re-linking is a within-one-run proof: `planTrackContinuity` matches rows that vanished in *this* run against files new in *this* run, and by the time a later run sees the new path, `Cleanup` has hard-deleted the row. Spanning runs needs tombstones, and that cost is not in the scanner — every read path has to decide whether an absent row is visible, orphan cleanup must not reap aggregates holding only tombstones, and a purge flow becomes mandatory or the DB grows forever. A feature with a UI, not a fix, for a sequence the operator controls. **Edge case:** "reorganise, then scan" is safe; "move the files out, scan, move them back, scan again" silently loses that track's stars, playlists, history and queue position even though the files are byte-identical — same for a move between two libraries reconciled as separate runs. Mitigation: batch the moves and scan once, pausing the scheduled scan task while rearranging by hand. Full analysis, the objection list and the revisit trigger: [`docs/architecture/caveats.md#a-move-that-straddles-two-scan-runs`](docs/architecture/caveats.md#a-move-that-straddles-two-scan-runs).
- [>] Guarding a vanished or unreadable sub-tree inside a present library root
  Merges the former "Unreadable subtree swept silently — and can be re-linked" (Data Integrity & Scanning) and "Guard a vanished sub-tree inside a present library root" (Backend — Library): an unreadable subtree and an unattached one fail identically from the scanner's side. `makeWalkFn` swallows per-entry errors, so a hollow mountpoint's files stat ENOENT exactly as deleted ones do — the tracks are swept, and worse, `planTrackContinuity` can re-point such a row onto a byte-identical file elsewhere, handing its stars, playlist entries and history to the wrong file with no error anywhere. Accepted on an explicit assumption: mounts are library **roots** (`mount /music/library1`), never directories inside a library — and a dropped root trips `Scan`'s phase-1 guards, failing the run atomically. **Edge case:** a sub-mount, bind mount, junction or autofs subdirectory *under* a root that is not attached when a scan runs. Mitigation: mount at the library root, never inside it, and do not scan a half-mounted library. Full analysis, five candidate fixes with their objections (favoured: the portable volume tripwire) and the revisit trigger: [`docs/architecture/caveats.md#vanished-sub-trees-inside-a-present-library-root`](docs/architecture/caveats.md#vanished-sub-trees-inside-a-present-library-root).
- [>] Stable artist identity across a rename (artist id churn)
  An artist's identity is `name_norm` alone, so correcting a spelling creates a new row and orphan cleanup deletes the old one. The album fix does not port — `planAlbumContinuity` proves continuity from "every track this album holds is in this batch", which for an artist spanning many albums through two multi-valued associations is essentially never true on an incremental scan, so the guard would decline exactly when it is needed. A real fix needs its own signal and its own spec (MBID plus explicit rename detection): too much machinery for a rare manual correction. **Edge case:** a rename loses the star, the DB-id-keyed image cache, `LastImageFetchAt` (so the artist-image task re-hits the rate-limited providers) and `/artist/:id` links; the manual cover survives only *with* an MBID, so the covers that break belong to the unmatched artists most likely to hold a hand-uploaded image. Genres (same root cause) and the positional-id cover key stay scheduled under "Backend — Data Integrity & Scanning" — the latter shrinks this to the star, `LastImageFetchAt` and the link. Full analysis and the revisit trigger: [`docs/architecture/caveats.md#artist-id-churn-on-rename`](docs/architecture/caveats.md#artist-id-churn-on-rename).
- [>] Audio-hash coverage for the remaining eight audio formats
  `libs/audiohash` fingerprints the audio payload of FLAC, MP3, MP4/M4A/M4B, WAV, AIFF, Ogg Vorbis and Opus — eight of `walk.go`'s sixteen extensions — which is what lets `planTrackContinuity` keep a track's row, and with it its playlists, play history, stars and queue position, across a move that *also* retags the file (the common Picard/beets rename-from-tags case, where `file_size` and `title` both change so the `size+title` proof goes blind). The other eight — **WMA, APE, WavPack, raw AAC, Matroska/WebM (`.mka`, `.webm`), TTA, DSF** — will not be covered for now. Each needs a real per-format container parser (ASF objects, EBML, and four bespoke codec containers) for far worse coverage-per-line than the chunk-list and Ogg-page walkers already written, and they are rare in real libraries; the covered eight are what people actually store music in. The consequence is bounded and fail-safe: an uncovered format falls back to the `size+title` proof, which is exactly what every format had before the hash existed — nothing regressed, those moves just stay unprovable. Same for an Ogg file carrying a mapping other than Vorbis or Opus (Ogg FLAC is declined deliberately: it gives each metadata block its own packet, so a fixed header-packet skip would leave an embedded picture inside the digest and the hash would not survive an art edit), and for a chained, truncated or trailer-bearing Ogg, declined rather than hashed on a missing length component — a missing length component would collapse every such file into one collision class, and a false match merges two tracks' histories, which is worse than losing one's. Revisit only if a library shows up that is materially WMA or Matroska. **The user-visible half of this decision still needs writing down** — tracked under "Backend — Data Integrity & Scanning".
- [>] Sharing and Chat
  Sharing exists to hand out public unauthenticated links (`/share.php?id=…&secret=…` + an HTML landing page) that bypass auth by design; Chat is a global message wall with no rooms or delivery, vestigial in the ecosystem and pointless on a single-user server. Don't add them, and don't file them as gaps again.
- [>] `getUsers` (and Subsonic user CRUD: `createUser`/`updateUser`/`deleteUser`/`changePassword`)
  Admin-only user administration over `/rest`. It only duplicates the users CRUD that already lives on `/api/v1` — the intended admin surface per `CLAUDE.md`'s `/rest`-vs-`/api/v1` split — breaks no playback client (they only ever call `getUser` for their own record), and would add a second privileged write surface plus extra plumbing (the subsonic `Handler` holds only the `AdminChecker` closure, not a user lister). `getUser` (own record) IS implemented; this is the deliberate line where `/rest` stops. Don't file it as a gap again.
