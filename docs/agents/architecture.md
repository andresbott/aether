# Architecture — layering, package roles, design decisions

Aether (module `github.com/andresbott/aether`) is a self-hosted **music server**:
a single Go binary with an embedded Vue 3 SPA, SQLite persistence, an
OpenSubsonic-compatible `/rest` API for all music functionality, and a private
`/api/v1` API for server administration. It is pre-release software with **no
backwards-compatibility obligations** (see CLAUDE.md: no migration code, no
compat shims — change schemas freely; the user drops the DB).

Read next, per area: [subsonic-api.md](subsonic-api.md) ·
[api-conventions.md](api-conventions.md) · [scanning.md](scanning.md) ·
[frontend.md](frontend.md) · [features.md](features.md) ·
[authentication.md](authentication.md) · [testing.md](testing.md) ·
[releasing.md](releasing.md)

## Layering

    app/cmd (cobra CLI, config, wiring)          webui/ (Vue 3 SPA)
            |                                        | (built + embedded)
    app/router ──── /api/v1 handlers ── /rest subsonic ── app/spa (embed.FS)
            |            |                    |
    app/tasks (task defs)|                    |
            |            v                    v
    internal/* ── store (GORM/SQLite) ── scanner ── tags ── model
                  taskrunner · assetstore · artistimage · coverart ·
                  covergen · identify · albumidentify · metadataedit ·
                  radiobrowser · unidecode
    libs/ ── acoustid, fpcalc (standalone clients, no aether imports)

- **`app/cmd` is the only composition root.** `server.go` builds everything —
  DB (SQLite WAL, `busy_timeout=5000`, max 10 conns), store, scanner config,
  task runner + scheduler, optional identify/artist-image services — and hands
  it to `router.Cfg`. Optional dependencies stay `nil` and handlers degrade
  gracefully (e.g. `Identifier: nil` disables audio identification; no
  provider API keys means the fetch task reports "not configured").
- **`app/router` owns routing only.** Three surfaces on one gorilla/mux
  router, in order: `/api/v1` (admin), `/rest` (Subsonic), then the SPA
  catch-all on `/`. A second HTTP server (observability, default :9009)
  serves Prometheus `/metrics` via `handlers.Admin()` — opt-in via
  `Observability.Enabled`, and not started at all when false.
- **`internal/store` is the only DB gateway.** It wraps `*gorm.DB`; handlers
  never touch GORM directly except through it. `Store.Transaction(fn)` yields
  a tx-scoped `*Store`. Query filters are small structs in `filters.go`
  (`SearchFilter`, `ArtistsFilter`, `StarredFilter` — all `LibraryID *uint`,
  nil = cross-library).
- **`internal/model` is schema only.** GORM structs + `Migrate()`
  (AutoMigrate + the composite unique index `idx_album_identity` on
  `albums(name_norm, album_artist_norm, mb_release_id)` — album identity is
  name+albumartist+MB release, all three normalized/optional).
- **`libs/` holds packages with zero aether imports** (acoustid, fpcalc) —
  deliberate extraction candidates. Don't import `internal/` from `libs/`.

## The two-API split (do not blur it)

Settled in CLAUDE.md; restated because it decides where every new endpoint goes:

- **All music functionality goes through `/rest`** and must stay
  OpenSubsonic-compliant. Missing capability? Add a proper OpenSubsonic
  *extension* under `/rest` advertised via `getOpenSubsonicExtensions` —
  never a bespoke `/api/v1` music endpoint. See [subsonic-api.md](subsonic-api.md).
- **`/api/v1` is server management only**: libraries CRUD + folder browse,
  tasks/schedules/executions, metadata editor, artist MBID/MusicBrainz search,
  `GET /artists/{id}/image-source` (which of aether's store / the music folder /
  the generated avatar the artist's image comes from — a server filesystem
  detail, not a Subsonic field), artist image preview + pick from the image
  providers (`/artists/image-preview`, `/artists/{id}/image-from-search` — see
  [scanning.md](scanning.md)), radio-browser import proxy, health. TODO.md
  plans moving these under an `/admin` path eventually.

## Key domain types (internal/model)

- `Library` — a music folder root; per-library `HideArtists`, `DefaultView`,
  `Icon`, JSON-encoded `ExcludePatterns`. Deleting/changing a library path
  wipes its tracks. `Source` (`"db"` | `"config"`) records who owns the row —
  see [Config-provisioned libraries](#config-provisioned-libraries).
- `Track` — `FilePath` unique; `LastSeenAt` drives scan cleanup;
  `LibraryID` cascade-deletes. `Album`/`Artist`/`Genre` link via join tables
  (`AlbumArtist`, `TrackArtist`, `TrackGenre`, `AlbumGenre`).
- `*Norm` columns (e.g. `TitleNorm`, `NameNorm`) hold
  `internal/unidecode.Normalize` output (lowercased ASCII transliteration) —
  used for identity and alphabetical indexing. Always populate them when
  writing names.
- `StarredItem` — single junction table keyed `(owner, item_type, item_id)`
  (unique index `idx_starred_item`). `Owner` is the authenticated user's
  **login string**, the same convention as `Playlist`, `PlayQueue`,
  `PlaylistPlay` and `Scrobble` — so every per-user surface is owner-scoped
  today. Re-keying those columns on `User.ID` is the remaining multi-user step
  (TODO.md, Future releases); a per-type star split is optional, not planned —
  don't invent a different direction.

## Background work: taskrunner + scheduler

`internal/taskrunner` wraps `go-bumbu/tempo`'s queue runner; executions
persist in the DB, and each run's logs stream to a per-execution file under
`<DataDir>/task-logs` (read back via `FileTaskLogReader`). Cron scheduling is
`go-quartz`, with schedules stored in `ScheduleStore` and re-armed at startup.
Tasks are registered in `app/cmd/server.go` from `app/tasks`: `scan`,
`scan-full`, `fetch-artist-images`. **A scan does not auto-trigger the
artist-image fetch** — they are deliberately independent.

## Data directory layout

`DataDir` (default `./data`) holds: `aether.db` (SQLite), `metadata/`
(assetstore: `<kind>/<key>/<name>[.auto].<ext>` — a plain `<name>.<ext>` is a
manual upload the fetcher must never overwrite; `.auto.` marks fetched
images), `image-cache/` (imagecache), `task-logs/`.

`metadata/` and `image-cache/` are deliberately separate trees with opposite
guarantees. `metadata/` is **authoritative**: it holds the only copy of manually
uploaded art, so nothing may clear it. `image-cache/` is **pure cache** —
display-sized re-encodes of covers, generated art included, keyed
`<kind>/<key>/<name>.<source fingerprint>.<size>.<webp|jpg>`. Deleting it costs
only the CPU to rebuild, which makes it the safe thing to wipe when covers look
wrong. There is no `generated-covers/` tree any more; generated covers are
rendered once at full size and cached as ordinary derivatives.

## Configuration

`go-bumbu/config`: defaults → `config.yaml` (`-c` flag) → `.env` file → env
vars prefixed `AETHER_` (e.g. `AETHER_ENV_LOGLEVEL=debug`). Config values
starting with `@` load the referenced file's contents (used for gitignored
`*.api.key` provider keys).

**Optional bools in config need a presence check, not just a `*bool`.**
`go-bumbu/config`'s unmarshaller allocates every nil pointer field it walks
(`unmarshal.go`), so after loading, an omitted key is an allocated `false` —
indistinguishable from an explicit `false`. Any config bool whose default is
`true` must therefore be re-checked against the handler
(`normalizeLibraryBools` in `app/cmd/config.go` asks `handler.GetString` per
key and resets absent ones to nil). Declaring `*bool` alone silently flips the
default for everyone who didn't spell the key out.

### Config-provisioned libraries

Libraries come from **two additive sources**: the admin UI (`/api/v1/libraries`)
and a `Libraries:` list in the config file. `Library.Source` (`model.SourceDB` /
`model.SourceConfig`) records which.

`reconcileLibraries` (`app/cmd/libraries.go`) runs in `server.go` right after
`store.New`, before anything reads the table, and **materializes** each config
entry into a real `libraries` row. That is the whole design decision: config
libraries get a normal autoincrement ID, so every existing consumer — the
`tracks.library_id` FK, the scanner, `getMusicFolders`, the per-library SQL
joins in `internal/store` — keeps working untouched, and nothing needs to know
libraries have two origins.

Semantics (each covered by a test in `app/cmd/libraries_test.go`):

- Config entries are rewritten from the file on **every** startup, including
  fields the entry omits (those revert to their defaults). This is what makes
  them read-only over the API: an accepted edit would silently revert on the
  next restart, so `PUT`/`DELETE` answer **409 `config_managed`** and `POST`
  refuses a name or path a config library already claims.
- Matching is by **path** first (that is what gets scanned), then by **name**.
  A colliding UI-created row is **adopted** rather than duplicated, keeping its
  scanned tracks. A path change wipes tracks, exactly as the API does it.
- Name matching one row while the path matches a *different* one is
  unresolvable, so startup **fails** instead of guessing.
- Removing an entry from config **never deletes** the library: the row is handed
  back to the UI as an ordinary editable one, tracks intact. Deleting a library
  stays a deliberate UI action, so a commented-out entry can't wipe a scan.
- `LastScanStartedAt` is runtime state, not configuration, and stays on the row
  for both sources.

Config entries are validated with the **same exported validators** the API uses
(`libraries.ValidateName/ValidatePath/…`) so a config typo fails as loudly as a
bad request, with the same message. Don't add a second copy of those rules.

## External services (all optional)

- **MusicBrainz** (`internal/artistimage.MusicBrainzSearch`, pickers in the
  metadata editor) — artist/release MBIDs drive image fetching and album identity.
- **fanart.tv / TheAudioDB** (`internal/artistimage`, chained providers) —
  artist images; enabled per configured API key.
- **Cover Art Archive** (`internal/coverart`) — album covers by MBID.
- **AcoustID + Chromaprint** (`internal/identify`, `libs/acoustid`,
  `libs/fpcalc`) — audio fingerprint identification; requires the `fpcalc`
  binary on the host and a per-version app key baked into `app/metainfo`.
  Availability is decided once at startup; when either is missing the router
  gets a nil identifier plus a user-facing reason, which
  `GET /api/v1/metadata/capabilities` returns as `identify: false` +
  `identify_unavailable_reason` so the editor can grey out Identify and say
  what is missing rather than hiding it. What a confirmed match actually stages
  is the user's choice: both identify dialogs render the same
  `IdentifyFieldSelect` row (all fields on by default) and the view narrows each
  overlay through `pickOverlayFields` (`webui/src/lib/identifyFields.ts`), so a
  match can fill one field across a batch without touching the rest. Add a field
  to `candidateToOverlay` or `albumPickToOverlay` and you must add it to
  `IDENTIFY_FIELDS` too — an uncovered overlay key is silently dropped from every
  apply in both dialogs (a unit test guards this).
  **Identification is cached in three layers, and each covers a different
  cost.** `identify.Cache`
  (LRU, keyed path + size + mtime) sits on the single `*identify.Identifier` that
  BOTH endpoints resolve through, so per-track and album identify share one
  fingerprint pass: identifying songs and then identifying them as an album pays
  no second fpcalc run or AcoustID call. Only successful answers are stored — a
  rate-limited lookup must stay retryable — while an empty match is stored, being
  the most expensive answer to re-derive. A tag save moves mtime and so costs a
  re-fingerprint; that is deliberate, since no stat can tell a tag rewrite from a
  re-encode.
  `albumidentify.CachingReleaseLookup` covers the OTHER half of the album flow's
  cost and is the layer people forget: `Resolve` enriches up to
  `MaxEnrichedOptions` (8) options with their MusicBrainz tracklist, and that
  client is throttled to **one request per second**, so a repeat album identify
  waited ~8s even with every fingerprint cached. It wraps the `ReleaseLookup`
  *in front of* the throttle — a hit must not consume a rate-limiter token, or
  the wait (which is the whole cost) is still paid.
  Finally the frontend's `useIdentifyCache` holds the shaped responses so
  reopening a dialog is zero-network (the two response shapes are not
  interconvertible — an `AlbumOption`'s assignments carry no scores or
  alternative candidates — which is why cross-flow sharing lives on the server
  primitive, not in the browser).
  All three are in-memory: a backend restart clears the first two, a page reload
  the third, so the first run after either is legitimately slow.
- **Album identification** (`internal/albumidentify`) answers the album question
  the editor asks when tagging a rip: which single release explains this whole
  selection, and where does each file sit on it. It fingerprints every file
  through `internal/identify`, unions every release the AcoustID candidates
  mention, ranks the union (coverage of the selection first, then mean score,
  track-position contiguity, tracklist-size fit, agreement with the files'
  existing album tag, single-disc bonus, earliest year as tiebreak), enriches
  the best `MaxEnrichedOptions` (8) with their MusicBrainz tracklist, and places
  the files no fingerprint matched by duration + title similarity. The cap
  exists because a dozen-file selection routinely unions dozens of releases —
  every reissue and compilation a track ever appeared on — and MusicBrainz is
  throttled to a few requests per second; the options past the cap still appear,
  with an unknown track count and no gap-fill. Exposed as
  `POST /api/v1/metadata/identify-album` (management API, not `/rest`), nil-safe
  exactly like identify: no fingerprinting service means a 503 and a greyed-out
  button. A per-file fingerprint failure is reported on that file's row and a
  failed MusicBrainz lookup only degrades its own option — neither fails the
  request.
- **radio-browser.info** (`internal/radiobrowser`) — station search proxied
  server-side to dodge CORS; an admin import tool only.

All outbound clients send the `Aether/<version> (github.com/andresbott/aether)`
user agent — keep that convention for new clients.

### `internal/upstream` — the shared outbound HTTP policy

Every third-party client above goes through `upstream.Doer` (`upstream.New(service, userAgent, rps)`)
rather than its own `http.Client` + `rate.Limiter`. **Use it for new outbound
clients; don't hand-roll the retry/throttle again.** It provides:

- fair-use throttling (burst 1) applied *before* the request,
- a bounded retry (3 attempts, 500ms doubling) for transient failures —
  5xx, 429, timeouts, transport errors — and `Retry-After` compliance capped
  at 5s so a user-facing lookup can't hang,
- no retry on 4xx (a refusal won't change) — `upstream.IsRejected(err)` lets a
  caller treat "no data for this id" as an empty result,
- a typed `*upstream.Error` carrying the technical detail for logs *and*
  `UserMessage()`, a sentence naming the service for the UI.

Handlers map it with `upstream.HTTPStatus(err)` (502, or 429/504 when more
precise) and `upstream.UserMessage(err, fallback)` via each handler package's
`writeUpstreamErr`. **Never put a raw Go error in a user-facing body** — that
is what leaked `{"error":"...","code":"upstream_error"}` onto the screen.

Degrade rather than fail where a fallback exists: `coverart.List` tries the
release-group MBID when the release lookup fails, since both describe the same
album.

## The `/api/v1` error envelope

Every `/api/v1` failure answers RFC 9457 `application/problem+json` — a
`Problem{type, title, status, detail, instance}` body (see
[api-conventions.md](api-conventions.md)) — except the metadata package's
batch endpoints (`updateTracks`, `rawTags`), which answer a per-row
`{results: [...]}` envelope instead, forwarded as plain `application/json`
even on `updateTracks`' failing `500` (`rawTags` always answers `200` once the selection is valid); see
api-conventions.md's error-shape section for the detail. Most handler
packages (metadata, tokens, libraries, artists, radiobrowser, users) build a
Problem directly via
`app/router/handlers/httperr`; `tasks` calls it directly for its one JSON
error body (`queue_full`) and otherwise still answers bare `http.Error`.
Anything that answers a bare `http.Error`/`http.NotFound` under `/api/v1` —
`tasks`' remaining plain-text errors, the `sessionGuard`/`headerGuard` auth
gate's `401`/`403`, the `/api/v1` catch-all's `400`, or a stray
`http.NotFound` inside an otherwise-migrated handler (`pictureImage`'s "cell
not found") — is still guaranteed the same shape by
`app/router/errors.go`'s `jsonErrorEnvelope` middleware: a handler body that
is already a JSON object (an `httperr` Problem, or an ad hoc handler JSON
body) passes through untouched, and a plain-text body gets a Problem built
for it from the response status alone (`errorCodeFor` maps status → slug,
`httperr.TitleFor` maps slug → title, the plain-text body becomes `detail`
verbatim).

**`middleware.Cfg.JsonErrors` must stay `false`.** It wraps *every* error body
blindly, escaping our JSON into a string field — the client then receives a
document where it expects a sentence. The go-bumbu middleware is still wired for
logging and Prometheus. Successful responses are never buffered, so streaming
(audio, task logs) is unaffected.

**The `application/problem+json` rewrite is scoped to the admin API mount
only.** `jsonErrorEnvelope` is mounted on the root router (`main.go`), so it
technically also wraps `/rest` — but the middleware checks `r.URL.Path`
against `apiV1MountPrefix` ("/api/v1") before choosing a shape: only a path
under that prefix gets a `Problem`. Every other path — `/rest` foremost —
keeps the original, non-RFC-9457 `apiError{error, code}` envelope this
middleware has always answered with, byte-identical to before problem+json
existed here. In practice `/rest` almost never reaches either fallback branch
at all: Subsonic's own error path (`writeError`) always answers HTTP 200 with
its own `subsonic-response` JSON envelope, never reaching the buffering
branch (status < 400). The exceptions are a handful of `/rest` media-serving
edge cases (a cover/stream file vanishing mid-request, no cover source at
all; see `media.go`'s `http.Error`/`http.NotFound` calls) — these fall to the
legacy `apiError` shape, unchanged from before this middleware ever spoke
RFC 9457. `/rest` must never answer `application/problem+json`.

See [api-conventions.md](api-conventions.md) for the full house rules.

## Decision records and historical docs

- `docs/superpowers/specs/` + `docs/superpowers/plans/` — dated design specs
  and implementation plans (the *why* for most UI subsystems). The code wins
  when they disagree; they are records, not current truth.
- `docs/architecture/` — three living frontend convention registries
  (see [frontend.md](frontend.md)); unlike the specs these are maintained.
- `docs/cue-playing.md` (2026-07-22) and `docs/gapless-playback-web-audio.md`
  (2026-06-30) — assessed-but-deferred designs. Check them before designing
  CUE support or Web-Audio gapless playback from scratch.
- `gonic_features.md` — an analysis of the *gonic* server used as a feature
  reference; it describes gonic, **not** aether.

## Known debt

Catalogued with chosen directions in `TODO.md` (repo root) — check it before
designing anything security-, scan-, or favorites-shaped; the gap you found is
probably already listed there with a decided approach.
