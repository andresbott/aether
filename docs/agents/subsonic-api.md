# Subsonic API — compliance rules, ID scheme, response envelope

Everything music-shaped is served from `app/router/handlers/subsonic`
(mounted at `/rest` in `subsonic.Register`). This is the subsystem with the
strongest external contract: **third-party OpenSubsonic clients must be able
to consume it**, so compliance beats convenience.

## The compliance invariants

- **Never add ad-hoc endpoints, parameters, or response fields to `/rest`.**
  If the standard lacks something the web UI needs, implement it as an
  OpenSubsonic *extension*: a `/rest` endpoint (or field) advertised in
  `getOpenSubsonicExtensions` (`extensions.go`) so non-supporting clients
  ignore it. Prefer upstreaming the extension to the OpenSubsonic registry.
  Thirteen extensions exist today — copy their shape.
- **Never route music features through `/api/v1`** — that surface is admin
  only ([architecture.md](architecture.md), "two-API split").
- Every endpoint registers under both `/rest/<name>` and `/rest/<name>.view`
  (the `register` helper in `subsonic.go` does this — use it).
- **JSON only, for now**: a middleware rejects `f=xml` with a Subsonic error.
  XML support is an open TODO for third-party client compat; if you add it,
  remove the middleware rather than special-casing endpoints.

## Response envelope

All responses go through `writeResponse` / `writeError` (`response.go`),
which wrap payloads in `{"subsonic-response": {status, version: "1.16.1",
type: "Aether", serverVersion, openSubsonic: true, ...}}`. Do not write JSON
directly from a handler. Errors use Subsonic numeric codes via `writeError`.

## The ID scheme (load-bearing)

Subsonic IDs are strings; aether encodes the entity type into a prefix
(`subsonic.go`): `ar-` artist, `al-` album, `tr-` track, `pl-` playlist,
`rs-` radio station, `ge-` genre — `encode*ID(uint)` / `decodeID(string)`.
Endpoints that accept a generic `id` (e.g. `star`, `getCoverArt`) dispatch on
the decoded type. **Any new entity exposed over `/rest` needs its own prefix
here**, and the frontend types (`webui/src/types/subsonic.ts`) consume these
strings verbatim.

A decodable id is not automatically a *valid* id for an endpoint: `decodeID`
accepts every prefix, so an endpoint that only handles some types must check the
returned type itself. `starItems` (`annotation.go`) is the reference — see below.

## Favorites (`star` / `unstar` / `starred`)

- **Only four types are starrable**: artist, album and track from the spec's
  `id`/`albumId`/`artistId` parameters, plus playlist via the `playlistStar`
  extension. `starrableTypes` (`annotation.go`) is the allowlist; genre and radio
  ids decode fine but are dropped, because `getStarred2`, `getGenres` and
  `getInternetRadioStations` have no field to report them and a stored row would
  be permanently unreadable. Do not "just persist it" for a future UI.
- `albumId` and `artistId` are *typed* parameters, so an id of another kind is
  dropped rather than starring the row with that numeric id under the
  parameter's own type.
- **Every response carrying an artist, album, song or playlist emits `starred`**
  as an RFC3339 timestamp, and **omits the key entirely when unstarred** —
  clients test for presence, so never write `""` or `null`. `AlbumID3`,
  `ArtistID3` and `Child` all define the field in the spec; on `Playlist` it is
  the `playlistStar` extension.
- Build the state with `starred.go`: `newStarLookup(h.store, artistIDs, albumIDs,
  trackIDs)` runs **one** `Store.StarredAt` per non-empty id set, then
  `applyArtist`/`applyAlbum`/`applyTrack` decorate the already-built entity maps.
  For a flat song list use `starredSongList(h.store, tracks)`. Never look a star
  up per row — a 500-album list would issue 500 queries.
- `Store.StarredAt(itemType, ids)` is keyed by type *and* id: ids are only unique
  per type, so dropping `itemType` would leak an album's star onto the song with
  the same numeric id.
- A star lookup failure is deliberately non-fatal — the response degrades to "no
  star state" rather than failing an entire browse over an annotation.
- Coverage lives in `starred_test.go` (per-endpoint present/omitted assertions)
  and `annotation_test.go` (the allowlist).

## Play queue (`savePlayQueue` / `getPlayQueue` + the `indexBasedQueue` extension)

`playqueue.go` stores the cross-device playback session: the queue, which slot is
playing, and the offset within that track. Four endpoints, **one stored queue** —
the ByIndex pair is a different *view* of the same row, not a second queue.

- **The current track is stored as an INDEX, never a track id.** A queue may hold
  the same track in several slots, and an id cannot say which copy is playing.
  The spec's id-based `savePlayQueue` resolves `current` to the **first matching
  slot** at the handler boundary and rejects (code 10) an id that is not in the
  queue at all; clients that need the exact slot use `savePlayQueueByIndex`.
  Regression: `TestSavePlayQueueResolvesCurrentToFirstMatchingSlot`,
  `TestBothQueueVariantsShareOneStoredQueue`.
- **A save with no `id` clears the queue** (the spec's clear call), and
  `currentIndex` must then be absent — `savePlayQueueByIndex` errors 10 on an
  out-of-range index, so sending `-1` would fail. The SPA relies on this.
- `current`/`currentIndex` is required as soon as ids are present ("required
  unless `id` is empty"); a missing `position` means 0.
- **`getPlayQueue` omits the whole `playQueue` element when nothing is saved.**
  Clients test for presence, so an empty container would read as "a queue with no
  tracks". Same for `playQueueByIndex`.
- **The store heals a queue whose tracks were deleted by a rescan**
  (`Store.GetPlayQueue`): missing tracks drop out, `CurrentIndex` follows the
  survivors, and `PositionMs` **resets to 0 when the current track itself is
  gone** — resuming 90s into a different song would be worse than restarting.
  `DeleteOrphanedAggregates` also sweeps `play_queue_entries`.
- Entries are full `Child` objects built through `starredSongList`, so a restoring
  client rebuilds its queue from one request instead of re-fetching every track.
- Owner is hardcoded to `admin` (`playQueueOwner`) until auth lands, but the row
  is already keyed by owner — same pre-wiring as `Playlist.Owner`.
- `decodeTrackIDs` (shared with `createPlaylist`) **checks the id type**: without
  it `pl-1`/`al-1` would contribute their bare number and enqueue the *track*
  with that id. Regression: `TestSavePlayQueueIgnoresNonTrackIds`,
  `TestCreatePlaylistIgnoresNonTrackSongIds`.

Bookmarks are deliberately not implemented — see
[features.md](features.md); `position` here already covers resume-within-song,
and a bookmark must never become a second source of truth for it.

## Discovery feed (`getDiscovery`, the `discovery` extension)

`discovery.go` serves the ranked Discovery feed. It is the one endpoint whose
response shape was a deliberate design decision, so the reasoning is recorded here
(full spec: `docs/superpowers/specs/2026-07-31-discovery-ranked-feed-design.md`).

- **Per-type arrays plus a `rank` field, NOT a mixed `item[]` union.** Every
  container in the spec is per-type (`albumList2 { album[] }`,
  `starred2 { artist[], album[], song[] }`), so a heterogeneous array with a
  `type` discriminator is a shape no Subsonic client parses. Instead the response
  is `discovery { album[], playlist[] }` of otherwise-standard `AlbumID3` /
  `Playlist` objects, each carrying two additive fields: `rank` (absolute position
  in ONE cross-type ordering) and `reason` (why it surfaced). A client ignoring
  both still gets two valid lists; a client that understands `rank` merges them
  with a one-line sort. That keeps the ranking authoritative and server-owned.
- **The internal score is deliberately NOT exposed.** `rank` carries everything a
  client needs, and publishing scores invites clients to re-sort or re-weight.
- **Params:** `size` (default 48, cap 200), `offset`, `seed`, `musicFolderId`. A
  malformed `seed` falls back to a day-derived default rather than erroring — a bad
  seed should still yield a feed.
- **Scoring lives in `internal/discovery`, which has no DB access**, so the formula
  is unit-testable without SQLite; `Store.DiscoveryFeed` gathers signals and calls
  in. Do not move arithmetic into SQL.
- **The candidate pool size is a constant (`discoveryPoolSize`), never derived from
  `offset`/`size`, and no gathering query may use `ORDER BY RANDOM()`.** Ranking is
  a sort, so a pool that grew with offset would let a newcomer displace ranks an
  earlier page already served, and the user would watch items repeat or vanish
  while scrolling. Variety comes from the seeded jitter term instead. Regression
  tests: `TestDiscoveryFeedRanksStayStableAcrossOffsets`,
  `TestDiscoveryFeedCandidateGatheringIsDeterministic`.
- Albums render via a batched `GetAlbumsByIDs` + `AlbumTrackStats`; playlists reuse
  the same per-row pattern as `getPlaylists` (the pre-existing N+1 TODO.md tracks).

## Parameter conventions

Helpers in `subsonic.go` — use them instead of raw query reads:
`paramStr`, `paramInt(default)`, `paramStrSlice`, `paramBoolPtr` (nil =
absent, distinguishes "not provided" from `false`), and `paramLibraryID`
(`musicFolderId`; nil = cross-library, matching the store's `*uint` filter
convention).

## Media serving

`media.go`: `stream` serves the original file via `http.ServeFile` (range
requests work; no transcoding). `getCoverArt` resolves, in order: assetstore
image → folder cover from disk → embedded front cover → deterministic generated
cover (`internal/covergen`).

**No cover is ever served as its original bytes.** Every response is a
display-sized, re-encoded derivative from `internal/imagecache`, cached under
`<DataDir>/image-cache/<kind>/<key>/`. Details that matter when changing this:

- **Sizes are quantized** to `coverSizeBuckets` (48/96/160/256/512/1024) so the
  cache stays a handful of files per entity rather than one per size any client
  invents. A request with no `size` gets `maxCoverSize` (1024) — never the
  original, which is the traffic the cache exists to avoid.
- **Format is negotiated** on `Accept`: WebP when the client names it, JPEG
  otherwise (a bare `*/*` means JPEG — plenty of Subsonic clients send it with no
  WebP decoder). Responses carry `Vary: Accept`.
- **Derivatives are keyed by a source fingerprint** (path+size+mtime for files,
  seed+style for generated covers), so a changed cover produces a new derivative
  instead of serving the old one, and building one sweeps the entry's superseded
  derivatives. Cover URLs are stable while the image behind them is not, and the
  replacement is not always *newer*.
- **Sources are tried in order, degrading on failure**: a truncated cover file or
  a track re-tagged since the scan falls through to the next candidate rather
  than answering 500, which would leave a broken image in every grid cell.
- Aspect ratio is preserved (`size` is a bounding box) and sources smaller than
  the box are never upscaled.
- Responses carry `Cache-Control: no-cache` plus an ETag over the served file's
  path+size+mtime, with `Last-Modified` deliberately omitted so falling back to
  an *older* file still invalidates.

Embedded art used to be re-extracted from the audio file on every request; it is
now extracted once per (file, size, format) and served from the cache afterwards.
The editor's `/api/v1/metadata/pictures/image` takes an **optional** `size` with
the same meaning — omitting it serves the original, which the picture picker
relies on when copying an image into another slot.

**Only front-cover art may be served as a cover.** Files and folders routinely
hold several images (back cover, disc label, booklet, artist photo); the metadata
editor manages all of them, but the player has exactly one cover vertical.
Enforced at three points, all of which must stay type-aware:

- `tags.ReadFrontCover` (`internal/tags/cover.go`) returns the picture *typed*
  `Front Cover`, scanning every attached picture — never index 0, which is
  whatever the tagger happened to write first. `readEmbeddedCover` uses it.
- `tags.Metadata.HasCover` (both readers) is true only when a front cover is
  present, so `HasEmbeddedCover` / `GetCoverTrackPath` never nominate a
  back-cover-only track as the album's cover source. ffprobe exposes the type in
  the attached-picture stream's `comment` tag (`Cover (front)`, `Cover (back)`,
  `Other`), which is why the `-show_entries` list includes `stream_tags=comment`
  and `stream_disposition=attached_pic`.
- `scanner.BestCover` rejects filenames naming other artwork
  (`coverDisqualifyTokens`) even when they also contain a cover/front token, so
  `Back Cover.jpg` and `booklet-cover.jpg` never become `album.CoverPath`.
  `scanner.IsUsableCoverPath` re-checks a stored path on every scan, so an album
  left pointing at a back scan or a deleted file recovers on rescan.

A file whose only embedded picture is typed `Other` counts as having no cover —
deliberate: it falls through to folder art, then the generated cover.

gosec path-traversal findings on these handlers are suppressed in
`.golangci.yaml` **with a documented justification**: served paths come from
the trusted DB or are validated by `metadataedit.ResolveInLibrary`. If you
change where a served path comes from, that justification must still hold —
otherwise validate against the library roots first (also an open TODO).

## Authentication (current state)

None. The SPA is same-origin and calls `/rest` without credentials
(`subsonicClient.initWithDefaults()`); the client code can also build
Subsonic `u`/`t`/`s` token params for a remote server, but the server never
validates them. The decided plan (TODO.md, do not re-design): session-cookie
auth for the SPA, per-user recoverable PATs for third-party Subsonic clients,
chained with OR semantics via the `userauth` library. Full model, including
the Authelia trusted-header deployment, in
[authentication.md](authentication.md).

## Testing

Every handler file has a sibling `_test.go` using a real in-memory SQLite
store — see [testing.md](testing.md). When adding an endpoint, mirror an
existing test file (e.g. `radio_test.go`) rather than mocking the store.
