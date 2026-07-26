# Architecture — layering, package roles, design decisions

Aether (module `github.com/andresbott/aether`) is a self-hosted **music server**:
a single Go binary with an embedded Vue 3 SPA, SQLite persistence, an
OpenSubsonic-compatible `/rest` API for all music functionality, and a private
`/api/v1` API for server administration. It is pre-release software with **no
backwards-compatibility obligations** (see CLAUDE.md: no migration code, no
compat shims — change schemas freely; the user drops the DB).

Read next, per area: [subsonic-api.md](subsonic-api.md) ·
[scanning.md](scanning.md) · [frontend.md](frontend.md) ·
[features.md](features.md) · [authentication.md](authentication.md) ·
[testing.md](testing.md) · [releasing.md](releasing.md)

## Layering

    app/cmd (cobra CLI, config, wiring)          webui/ (Vue 3 SPA)
            |                                        | (built + embedded)
    app/router ──── /api/v1 handlers ── /rest subsonic ── app/spa (embed.FS)
            |            |                    |
    app/tasks (task defs)|                    |
            |            v                    v
    internal/* ── store (GORM/SQLite) ── scanner ── tags ── model
                  taskrunner · assetstore · artistimage · coverart ·
                  covergen · identify · metadataedit · radiobrowser · unidecode
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
  detail, not a Subsonic field), radio-browser import proxy, health. TODO.md
  plans moving these under an `/admin` path eventually.

## Key domain types (internal/model)

- `Library` — a music folder root; per-library `HideArtists`, `DefaultView`,
  `Icon`, JSON-encoded `ExcludePatterns`. Deleting/changing a library path
  wipes its tracks.
- `Track` — `FilePath` unique; `LastSeenAt` drives scan cleanup;
  `LibraryID` cascade-deletes. `Album`/`Artist`/`Genre` link via join tables
  (`AlbumArtist`, `TrackArtist`, `TrackGenre`, `AlbumGenre`).
- `*Norm` columns (e.g. `TitleNorm`, `NameNorm`) hold
  `internal/unidecode.Normalize` output (lowercased ASCII transliteration) —
  used for identity and alphabetical indexing. Always populate them when
  writing names.
- `StarredItem` — single `(item_type, item_id)` table, **no user column**:
  the app is single-user today. Playlist has a pre-wired `Owner` column.
  TODO.md records the chosen multi-user direction (per-entity star tables
  with `user_id`) — don't invent a different one.

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
images), `generated-covers/` (covergen cache), `task-logs/`.

## Configuration

`go-bumbu/config`: defaults → `config.yaml` (`-c` flag) → `.env` file → env
vars prefixed `AETHER_` (e.g. `AETHER_ENV_LOGLEVEL=debug`). Config values
starting with `@` load the referenced file's contents (used for gitignored
`*.api.key` provider keys).

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
  what is missing rather than hiding it.
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

Every `/api/v1` failure answers `{"error": "<sentence>", "code": "<slug>"}`.
`app/router/errors.go` (`jsonErrorEnvelope`) guarantees it: a handler body that
is already a JSON object passes through untouched, and plain-text errors
(`http.Error`, `http.NotFound`) get an envelope built for them.

**`middleware.Cfg.JsonErrors` must stay `false`.** It wraps *every* error body
blindly, escaping our JSON into the `error` string — the client then receives a
document where it expects a sentence. The go-bumbu middleware is still wired for
logging and Prometheus. Successful responses are never buffered, so streaming
(audio, task logs) is unaffected; `/rest` reports its own errors inside a 200
Subsonic envelope and is untouched.

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
