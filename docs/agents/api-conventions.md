# API Conventions — `/api/v1` house rules: bounded URLs, POST-for-reads, one error shape

`/api/v1` is the internal, admin-only server-management API — see
[architecture.md](architecture.md)'s "two-API split" for what belongs here
versus `/rest`. This doc records the conventions the metadata picture/raw-tag
**header-safe redesign** established
(`docs/superpowers/specs/2026-08-22-metadata-picture-api-header-safe-redesign.md`)
so they apply to every `/api/v1` endpoint you add or touch, not only
metadata's. The worked example throughout is
`app/router/handlers/metadata/{metadata,pictures,raw}.go` plus the
machine-readable seed spec, `docs/openapi/aether-v1.yaml`.

## Bounded URLs: no list in a `GET`/`DELETE` URL

**Rule:** no `/api/v1` `GET` or `DELETE` operation may carry a
variable-length list in its URL — neither a repeated/array query parameter
nor an array-shaped path segment.

**Why:** production Aether runs behind **Caddy `forward_auth` + Authelia**.
Caddy copies the request URI into the `X-Forwarded-Uri` *header* on the
Authelia verify sub-request; Authelia's fasthttp read buffer
(`server.buffers.read`, default ~4 KB) answers oversized headers with **HTTP
431** *before the request reaches Aether*. The metadata editor used to encode
a multi-track selection as a repeated `?paths=` query param — one per
selected track — so a large multi-disc selection silently 431'd in
production, never reaching a handler that could log or explain it. Full
incident + design reasoning:
`docs/superpowers/specs/2026-08-22-metadata-picture-api-header-safe-redesign.md`.

**The fix, generalized:** a URL/header is a bounded channel; a body is not.
Anything that needs to carry a list — a track selection, a batch of ids —
travels in a request body, never the URL.

**The sanctioned exception shape:** a single, already-resolved scalar a
client got from a prior response — never a variable-length list — may ride
along as a `GET`/`DELETE` query param. `GET /metadata/pictures/image` is the
worked example: it stays a GET with a query because a browser can only `GET`
an image for an `<img src>`. It is still O(1) and header-safe: it carries a
single already-resolved `file` (a library-relative path, picked out of an
inventory response), never the selection that produced it. `GET
/radiobrowser/favicon?url=` and the two `candidate-info?url=` endpoints
(`/metadata/pictures/candidate-info`, `/metadata/artist-image/candidate-info`)
follow the same pattern — each takes one already-resolved URL a prior
response handed back, not a list. None of this loosens the array
prohibition above: a scalar picked from a prior response is still a scalar,
not a list, and a repeated/array parameter remains barred on every
`GET`/`DELETE` here.

**Enforcement:** `.spectral.yaml`'s `no-array-in-get-delete-url` and
`array-query-needs-maxitems` rules, run by `make spec-lint` (part of `make
verify`) and the `spec-lint` CI workflow. Both rules check parameters
declared at the operation level (`paths.<path>.get.parameters`) *and* at the
path-item level (a `parameters` array as a sibling of `get`/`delete`, which
applies to every operation on that path) — a shared param is just as capable
of causing a 431 as one declared under the operation. `make spec-lint` only
catches a violation in an endpoint that's actually described in
`docs/openapi/aether-v1.yaml` — the spec is a seed (see below), so a new
`/api/v1` `GET`/`DELETE` isn't linted at all until you add it there.

## A read that needs a list is a `POST` to a read sub-resource

**Rule:** when a read needs to carry a list (or any input too large for a
bounded URL), make it a `POST` to a named sub-resource that reads data — not
`GET`-with-a-body, and not the IETF `QUERY` method.

**Why not GET-with-body:** a body on a `GET` has no defined HTTP semantics —
proxies, caches and HTTP client libraries are free to drop it, and plenty do.

**Why not `QUERY`:** `QUERY` is the textbook answer (a safe, idempotent verb
that carries a body), but it is still a draft with no OpenAPI, codegen,
Spectral or browser `fetch` support today. Adopting it now would trade one
infrastructure gap (431s) for another (nothing in the toolchain understands
the verb).

**The pattern:** `POST` to a sub-path that names the read, document it as
safe/idempotent in its `description` even though the verb is `POST`, and keep
the response shape an ordinary read response, not a mutation result.

**Worked examples:**
- `POST /metadata/pictures/inventory` (`Handler.inventory`) — reports which
  picture slots are populated for a track selection; body is
  `{library_id, paths[]}`.
- `POST /metadata/tracks/raw-tags` (`Handler.rawTags`) — reads the complete,
  unfiltered tag map of a set of files; same `{library_id, paths[]}` body,
  decoded by the shared `Handler.decodeSelection`
  (`app/router/handlers/metadata/metadata.go`), which also enforces the
  selection cap (`maxSelectionPaths = 50`,
  `app/router/handlers/metadata/limits.go`) as defense-in-depth — the body
  already removes the 431 risk; the cap bounds the work a single request can
  demand.

**The same reasoning extends to selection-shaped mutations that would
otherwise be `DELETE`-with-body:** `POST /metadata/pictures/removals`
(`Handler.removals`) clears a picture cell across a selection. It is a named
batch-action `POST`, not `DELETE` with a body, so a client never has to
attach a payload to a verb that isn't specified to reliably carry one.

## One error shape: RFC 9457 `application/problem+json`, via `httperr`

**Rule:** every `/api/v1` handler answers an error as
`application/problem+json` (RFC 9457 "Problem Details for HTTP APIs"), built
with the shared `app/router/handlers/httperr` package. **This package is
`/api/v1`-only** — `/rest` keeps its own OpenSubsonic envelope (numeric error
codes inside a 200 `subsonic-response`; see
[subsonic-api.md](subsonic-api.md)) and must never use it.

**The shapes** (`httperr.go`):
- `Problem{type, title, status, detail, instance}` — a plain error. `type` is
  a stable, **never-fetched** URI (`https://aether.local/probs/<slug>`); only
  its last path segment (the slug) is meant to be read — the old ad hoc
  `"code"` string reborn as a URI suffix (`httperr.Slug` extracts it back
  out; `httperr.TitleFor` maps a known slug to its human title). `instance`
  is always the request path, so a client can tell which call failed without
  re-reading its own request.
- `ValidationProblem{Problem, errors[]FieldError}` — adds itemized
  field-level failures. `FieldError.Pointer` names the failing field with a
  JSON Pointer (RFC 6901) — e.g. `/paths` or `/paths/0` — whether the request
  was JSON, a query string, or multipart form: a caller only needs to know
  which field failed, addressed the same way regardless of wire format.

**Status convention, confirmed across every migrated handler:** `422` is
`httperr.WriteValidation` — hard-coded to `http.StatusUnprocessableEntity`,
always a `ValidationProblem`, for a request that is **well-formed but
invalid** (an unknown enum value, a selection over the size cap, an empty
list). Everything else goes through `httperr.Write` and is a plain,
non-itemized `Problem` — including
`400` for a **malformed** request (a missing required field, invalid
JSON/multipart). Concretely, on `GET /metadata/pictures/image`: a missing
`slot` is `400` (plain — the request doesn't even name a slot to validate);
a present but unrecognised `slot` is `422` (itemizing `/slot` — the request
is well-formed, the value is wrong).

**Upstream failures:** `httperr.WriteUpstream` maps a failed third-party call
through `internal/upstream`'s classification (see
[architecture.md](architecture.md)'s `internal/upstream` section) to `429`
when the provider is rate-limiting Aether, otherwise `502`/`504` — `detail`
is always `upstream`'s human sentence or a fallback, **never a raw Go
error**.

**Status — uniform across all of `/api/v1`.** All seven handler packages call
`httperr.Write` directly for every error today: `metadata`, `tokens`,
`libraries`, `artists`, `radiobrowser`, `users`, `tasks` (enumerated in the
`titles` var's own comment, `app/router/handlers/httperr/httperr.go` — not the
package doc above it, which only names a few as examples). The per-package
`writeError`/`writeErr` shims that once wrapped it were removed; every call
site now names `httperr.Write`. The front-door session/role
gate (`sessionGuard`/`headerGuard` in `app/router/api_v1.go` and
`app/router/proxy_auth.go`, which answer `401`/`403`/`500`) calls
`httperr.Write` directly too, rather than relying on the router fallback.
The router-level `jsonErrorEnvelope` middleware (`app/router/errors.go`,
wired in `app/router/main.go`) still guarantees the same `Problem` for any
bare plain-text error that reaches it **on a path under the admin API
mount** (`apiV1MountPrefix`, `"/api/v1"`) — `errorCodeFor` maps the response
status to a slug (`401` → `unauthorized`, `403` → `forbidden`, …),
`httperr.TitleFor` maps that slug to its human title, and the plain-text
body becomes `detail` verbatim. That fallback still matters for what's left
on that path: the `/api/v1` catch-all (`api_v1.go`'s `PathPrefix("")`, a
bare `400`) and a stray `http.NotFound` inside an otherwise-migrated handler
(`pictureImage`'s "cell not found" `404`, below). A body that is already a
JSON object (an `httperr` Problem, or an ad hoc handler JSON body) is passed
through untouched, so the two mechanisms never double-wrap each other.
`jsonErrorEnvelope` isn't a lesser, non-RFC-9457 fallback to work around —
together with the handler packages calling `httperr` directly, it is *how*
`/api/v1` stays uniform: every error response under this mount,
handler-authored or not, ends up `application/problem+json` — with one
deliberate exception, the batch endpoints described below. (Outside the
mount — chiefly
`/rest` — the same middleware answers the legacy, pre-RFC-9457
`apiError{error,code}` shape instead; see
[architecture.md](architecture.md)'s error-envelope section — `/rest` must
never speak problem+json.) `middleware.Cfg.JsonErrors` stays `false`
regardless — that flag is go-bumbu's own blind wrapper, unrelated to
`jsonErrorEnvelope`.

`pictureImage`'s "cell not found" `404` (inside the already-migrated
`metadata` package) takes the router-fallback path rather than calling
`httperr` directly: the handler answers Go's bare `http.NotFound` because that
endpoint is an image stream, not a JSON one. The envelope still turns it into
the same `Problem{type: .../probs/not_found, title: "Not found", detail:
"404 page not found", ...}` shape as everywhere else — `detail` is just Go's
stock message rather than a handler-authored sentence.
`docs/openapi/aether-v1.yaml` documents it as `application/problem+json` like
every other response on that path.

**The batch endpoints are the one deliberate exception.** `updateTracks`
(`PUT /metadata/tracks`) and its read-only sibling `rawTags` (`POST
/metadata/tracks/raw-tags`), both in `app/router/handlers/metadata`, each act
on a list of files and answer a per-row envelope instead of a single
`Problem` — `{results: [{path, ok, error}, ...]}` for `updateTracks`,
`{results: [{path, tags, unsupported, error}, ...]}` for `rawTags` — so a
client can tell which files in the batch failed and why, rather than losing
that detail behind one top-level error. `rawTags` always answers `200` once the selection is valid: an
unreadable file is just that row's `error`, never a failed batch.
`updateTracks` answers `200` when at least one file in the batch was written
and `500` only when every row failed. Both build the body with the
package's own `writeJSON`, which sets `Content-Type: application/json`
directly; `jsonErrorEnvelope` sees an already-a-JSON-object body
(`isJSONObject`) and forwards it untouched, so `updateTracks`' `500` is the
one `/api/v1` failure status that leaves the server as plain
`application/json`, never `application/problem+json`.

**Enforcement/reference:** `docs/openapi/aether-v1.yaml`'s
`components.schemas.{Problem,ValidationProblem,FieldError}` and
`components.responses.{BadRequest,NotFound,UnprocessableEntity,TooManyRequests,UpstreamError}`
— all typed `application/problem+json`, with no exception left among
single-resource error responses; the batch endpoints above remain
`/api/v1`'s one deliberate departure from that shape.

## Mount-relative paths — model the base through `servers:`

**Rule:** the OpenAPI document models `/api/v1` exclusively via
`servers: [{ url: /api/v1 }]`; every `paths:` key is mount-relative
(`/metadata/pictures/inventory`, never
`/api/v1/metadata/pictures/inventory`). Handlers already do the equivalent
thing structurally: routes are registered on the subrouter returned by
`app.router.PathPrefix("/api/v1").Subrouter()` (`app/router/main.go`), never
with `/api/v1` baked into an individual route string.

**Why:** `TODO.md` tracks a planned reorg — moving admin-only groups like
libraries and tasks under `/api/admin/...` — as a deliberate breaking URL
change, free to do now under the no-backwards-compatibility rule and
expensive once anything depends on the current paths. Keeping the base path
out of every individual path key means that reorg is a one-line
`servers[0].url` edit (plus the matching one-line mount-prefix change in
`app/router/main.go`), not a mechanical rewrite of every path and
`operationId` in the spec.

**No automated check for this today** — confirmed by hand (grepping the spec)
that no `paths:` key hard-codes `/api/v1` (only `servers[0].url` and prose
may). A Spectral rule forbidding a literal `/api/v1` substring inside any
`paths` key would be a natural follow-up; none exists yet.

## The seed OpenAPI spec — what's specced, what isn't

`docs/openapi/aether-v1.yaml` is the machine-readable contract for
`/api/v1`, but it is a **seed**: today it covers only the metadata editor's
picture/raw-tag surface (the endpoints this doc uses as its worked examples)
plus the shared `Problem`/`ValidationProblem`/`FieldError` components. Not
yet specced — real, mounted endpoints per `app/router/api_v1.go`, just not
described here: `artists`, `users`, `tasks`, `tokens`, `radiobrowser`,
`auth`, `libraries`, `health`/`version`/`me`, and the rest of `/metadata/*`
(`capabilities`, `identify`, `identify-album`, the structured editor's
`PUT /metadata/tracks`). Extending the spec to those groups is the standing
"align v1 with OpenAPI" initiative (`TODO.md`) — when you touch one of them,
consider adding its spec coverage rather than letting the gap widen.

Validate any spec edit with `make spec-lint` (`cd webui && npm run
spec-lint`, which runs `spectral lint ../docs/openapi/aether-v1.yaml -r
../.spectral.yaml --fail-severity=error` — `spectral` is only installed as a
`webui` devDependency, so it must run from there); it runs as part of `make
verify` and in CI (`.github/workflows/spec-lint.yml`). The negative fixtures
that prove the two bounded-URL rules actually fire live under
`docs/openapi/testdata/` and are never linted by `make spec-lint` itself,
which only targets `aether-v1.yaml` — lint them directly (from `webui/`:
`npx spectral lint ../docs/openapi/testdata/<file>.yaml -r
../.spectral.yaml`) if you change either rule and need to re-prove it still
catches a violation.
