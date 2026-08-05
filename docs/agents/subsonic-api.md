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
  Fourteen extensions exist today — copy their shape.
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

- **`StarredItem` is keyed by `(owner, item_type, item_id)`** (unique index
  `idx_starred_item`). Every star operation (`Star`, `Unstar`, `GetStarred`,
  `StarredAt`) takes the owner first — never drop it. `Store.StarredAt(owner,
  itemType, ids)` is keyed by *owner*, type, *and* id: ids are only unique per
  type, so dropping `itemType` would leak an album's star onto the song with the
  same numeric id; and dropping `owner` would return another user's star state.
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
- Build the state with `starred.go`: `newStarLookup(h.store, owner, artistIDs,
  albumIDs, trackIDs)` runs **one** `Store.StarredAt` per non-empty id set, then
  `applyArtist`/`applyAlbum`/`applyTrack` decorate the already-built entity maps.
  For a flat song list use `starredSongList(h.store, owner, tracks)`. Never look
  a star up per row — a 500-album list would issue 500 queries.
- A star lookup failure is deliberately non-fatal — the response degrades to "no
  star state" rather than failing an entire browse over an annotation.
- **`getStarred2` is a full browse response, not a bare id list.** Its albums
  carry `songCount`/`duration` (via `AlbumTrackStats`) and its artists carry
  `coverArt`/`albumCount` (via `GetArtistAlbumCounts`) — the same fields
  `getAlbumList2` and `getArtists` emit, because the web UI's favorites filter
  renders this response with the *same* rows and cards as the full library and
  their count columns would otherwise be blank. If you add a field to those
  endpoints' entities, add it here too.
- **`getStarred2`'s albums and artists are `name_norm ASC`**, matching
  `getAlbumList2`'s `alphabeticalByName` (`store.starredAlbums`/`starredArtists`).
  This is load-bearing: the client derives the alphabet rail's letter buckets from
  the returned order, so an unordered response silently breaks the rail. Starred
  *playlists* keep their own order — star recency (`store/star.go`) — and are
  deliberately **not** library-scoped, since a playlist can span libraries.
- `getStarred2` is **unpaginated by the spec** — no `size`/`offset`. Don't add
  them, and don't design a client view around them.
- Coverage lives in `starred_test.go` (per-endpoint present/omitted assertions,
  plus the `getStarred2` enrichment/order and library-scoping cases) and
  `annotation_test.go` (the allowlist).

## Playlists

Playlists are **created with the session owner** (`createPlaylist` extracts the
owner from `requestOwner`). `Store.GetPlaylists(owner)` returns the user's own
playlists plus all public playlists. Visibility and write access:

- **Reads of invisible playlists answer Subsonic error 70** (not found), with no
  existence leak — a foreign private playlist behaves as if it doesn't exist.
- **Writes to foreign playlists answer error 50** (not authorized).
- Guards: `visiblePlaylist(h.store, owner, playlistID)` checks read access;
  `ownedPlaylist(h.store, owner, playlistID)` checks write access
  (`updatePlaylist`, `deletePlaylist`, `createPlaylistSongs`,
  `removePlaylistSongs`).

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
- **Owner is the session user** (`requestOwner`); auth "none" uses the fixed owner
  `"admin"`.
- `decodeTrackIDs` (shared with `createPlaylist`) **checks the id type**: without
  it `pl-1`/`al-1` would contribute their bare number and enqueue the *track*
  with that id. Regression: `TestSavePlayQueueIgnoresNonTrackIds`,
  `TestCreatePlaylistIgnoresNonTrackSongIds`.

Bookmarks are deliberately not implemented — see
[features.md](features.md); `position` here already covers resume-within-song,
and a bookmark must never become a second source of truth for it.

## Now Playing (`getNowPlaying`)

`Store.GetNowPlaying()` returns a **global** list of all recent playback across
all users (the endpoint deliberately has no owner filter). Each entry carries both
the playing track and the **real username** of the player (`NowPlayingEntry{Track,
Owner}`). The handler reports each entry's `username` field as the actual owner,
while the **star state (`starred`) is the viewer's own** — decorated via
`starredSongList(h.store, requestOwner(r), tracks)` so each user sees their own
favorites, not the playing user's.

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

## Search (`search3` + the `searchGenres` extension)

`search.go` searches artists, albums and songs on a normalized substring match
(`unidecode.Normalize` over the `*_norm` columns), each with its own
`count`/`offset` pair. Genres are the one non-spec result type:

- **`SearchResult3` has no `genre` field in the spec**, so genres are gated behind
  the advertised `searchGenres` extension and a `genreCount` parameter that
  **defaults to 0**. Without it the response is byte-identical to the standard
  shape — *the `genre` key is absent entirely, not an empty array* — so a
  third-party client neither pays for the query nor sees a field it cannot parse.
  An explicit `genreCount` always emits the array, empty or not, so a client that
  asked can tell "no matches" from "not supported". Regressions:
  `TestSearch3OmitsGenresWithoutGenreCount`,
  `TestSearch3EmitsEmptyGenreArrayWhenAsked`.
- Entries are standard spec `Genre` objects built by `genreToMap`
  (`browsing.go`), shared with `getGenres` so a genre looks identical wherever it
  surfaces — including the `genreCoverArt` extension's `coverArt` id.
- **`model.Genre` has no `name_norm` column, deliberately** — it is the one entity
  whose search matches and sorts in Go (`Store.SearchGenres` normalizes with
  `unidecode.Normalize` per row). Genres are few enough that `GetGenres` already
  loads the whole table on every genres-view load, and a column would have to be
  kept in step by every writer that renames a genre. Do not "optimize" this into a
  column without a backfill: SQLite cannot `ALTER TABLE ... ADD NOT NULL` on an
  existing table, and a nullable one would leave every existing genre
  unsearchable until a rescan.
- `song_count`/`album_count` come from `genreCountsSelect` (`store/genre.go`) and
  are **library-independent by design**: a `musicFolderId` filter decides *which
  genres match*, but a matched genre still reports its whole footprint, the same
  number the genre pages show. Regression: `TestSearchGenresFiltersByLibrary`.
- Genres are not starrable and carry no star state — see the favorites section.

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

In native mode (auth method `native`), **`/rest` now requires the session
cookie** (interim, until the PAT layer lands). `subsonic.Register` takes an
`IdentityResolver func(*http.Request) (owner string, ok bool)` that resolves
the cookie → login via `sessions.GetSessData` and `users.GetUser`
(deliberately no session renewal — `/me` renews); `nil` (auth "none") keeps
the fixed owner `"admin"` and `/rest` open. Without a session, endpoints
answer Subsonic error 40 (wrong credentials). Handlers read the resolved
identity via `requestOwner(r)`. Every per-user surface — play queue, stars,
playlists, history — is owner-scoped: data is keyed/filtered by the session
user's login. **Third-party Subsonic clients (Symfonium, DSub) are locked out
in native mode until PATs land** — they cannot obtain or send the session
cookie. Auth "none" (dev/trusted-LAN mode) remains unchanged: no resolver, no
validation, fixed owner `"admin"`. The decided plan (TODO.md, do not
re-design): per-user recoverable PATs verified via `u`/`t`/`s`/`apiKey`,
replacing the interim cookie resolver. Full model, including the Authelia
trusted-header deployment, in [authentication.md](authentication.md).

## Testing

Every handler file has a sibling `_test.go` using a real in-memory SQLite
store — see [testing.md](testing.md). When adding an endpoint, mirror an
existing test file (e.g. `radio_test.go`) rather than mocking the store.
