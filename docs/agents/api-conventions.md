# API Conventions — `/api/v0` house rules: bounded URLs, POST-for-reads, one error shape

`/api/v0` is the internal, admin-only server-management API — see
[architecture.md](architecture.md)'s "two-API split" for what belongs here
versus `/rest`. This doc records the conventions the metadata picture/raw-tag
**header-safe redesign** established
(`docs/superpowers/specs/2026-08-22-metadata-picture-api-header-safe-redesign.md`)
so they apply to every `/api/v0` endpoint you add or touch, not only
metadata's. The worked example throughout is
`app/router/handlers/metadata/{metadata,pictures,raw}.go` plus the
machine-readable seed spec, `docs/openapi/aether-v0.yaml`.

## Bounded URLs: no list in a `GET`/`DELETE` URL

**Rule:** no `/api/v0` `GET` or `DELETE` operation may carry a
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
single already-resolved `file` (a folder-relative path, picked out of an
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
`docs/openapi/aether-v0.yaml` — the spec is a seed (see below), so a new
`/api/v0` `GET`/`DELETE` isn't linted at all until you add it there.

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
- `POST /metadata/pictures/inventory` (`ImagesHandler.inventory`) — reports
  which picture slots are populated for a track selection; body is
  `{scan_folder, paths[]}`.
- `POST /metadata/tracks/raw-tags` (`TagsHandler.rawTags`) — reads the
  complete, unfiltered tag map of a set of files; same `{scan_folder, paths[]}`
  body, decoded by the shared package-level `decodeSelection`
  (`app/router/handlers/metadata/selection.go`), which also enforces the
  selection cap (`maxSelectionPaths = 50`,
  `app/router/handlers/metadata/limits.go`) as defense-in-depth — the body
  already removes the 431 risk; the cap bounds the work a single request can
  demand.
- `POST /libraries/preview` (`Handler.preview`,
  `app/router/handlers/libraries/preview.go`) — reports the track/album
  counts a candidate filter set (`{filters[]}`, the same shape a library
  stores) would select, without storing anything; the filter builder calls it
  on every edit, and `filters[]` — up to `libraryfilter.MaxFilters` entries,
  each with up to `libraryfilter.MaxValues` values — is exactly the kind of
  variable-length list a bounded `GET` query string cannot carry.

**The same reasoning extends to selection-shaped mutations that would
otherwise be `DELETE`-with-body:** `POST /metadata/pictures/removals`
(`ImagesHandler.removals`) clears a picture cell across a selection. It is a named
batch-action `POST`, not `DELETE` with a body, so a client never has to
attach a payload to a verb that isn't specified to reliably carry one.

## One error shape: RFC 9457 `application/problem+json`, via a threaded `problemjson.Writer`

**Rule:** every `/api/v0` handler answers an error as
`application/problem+json` (RFC 9457 "Problem Details for HTTP APIs"),
written by calling a `*problemjson.Writer`
(`github.com/go-bumbu/http/problemjson`, v0.6.0) **threaded into the handler
struct as a `Problems *problemjson.Writer` field** — set once, per handler,
when `app/router/api_v0.go`'s `attachApiV0` constructs it from
`MainAppHandler`'s own unexported `problems` field. **This mechanism is
`/api/v0`-only** — `/rest` keeps its own OpenSubsonic envelope (numeric error
codes inside a 200 `subsonic-response`; see
[subsonic-api.md](subsonic-api.md)) and must never speak problem+json.

**`app/router/handlers/problems` owns what aether must own, and nothing
more.** It exposes a single constructor, `New(masked bool)
*problemjson.Writer` — no package-level write funcs, since that would make
the Writer a hidden global; it is built once in `router.New` and passed
explicitly from there. It configures only:
- the stable, **never-fetched** base URI every problem's `type` is built
  from, `https://aether.local/probs` — unchanged from the original ad hoc
  error package, so every `type` URI is byte-identical across the migration;
- the human titles for aether's own six slugs —
  `identify_unavailable`, `too_many_tokens`, `usertoken_unavailable`,
  `not_configured`, `last_admin`, `queue_full` — the
  generic slugs (`not_found`, `validation_error`, `internal`,
  `unauthorized`, `forbidden`, `conflict`, `rate_limited`, `unavailable`,
  `upstream_error`, `upstream_rate_limited`, `upstream_timeout`) ship as
  `problemjson` defaults and must not be redeclared here;
- the `Request-Id` extractor that fills a masked body's `reference` (see
  below).

**The shapes** (old name → new name): `httperr.Problem` →
`problemjson.Details{type, title, status, detail, instance, reference}`,
`httperr.ValidationProblem` → `problemjson.ValidationDetails{Details,
errors[]FieldError}`, `httperr.FieldError` → `problemjson.FieldError{pointer,
detail}`, `httperr.Slug` → `problemjson.Slug` (still a package func, the
inverse of `Writer.TypeURI`). `type` is a stable, **never-fetched** URI
(`https://aether.local/probs/<slug>`); only its last path segment (the slug)
is meant to be read — `problemjson.Slug` extracts it back out;
`Writer.TitleFor` maps a known slug to its human title, falling back to the
slug itself for an unrecognised one. `instance` is always the request path,
so a client can tell which call failed without re-reading its own request.
`FieldError.Pointer` names the failing field with a JSON Pointer (RFC 6901)
— e.g. `/paths` or `/paths/0` — whether the request was JSON, a query
string, or multipart form: a caller only needs to know which field failed,
addressed the same way regardless of wire format.

**A 422 itemizes every problem, not just the first.** `WriteValidation`'s
`errors[]` carries one `FieldError` per thing wrong with the request, and each
`Pointer` follows the shape of the *request's own JSON*, not the stored
model — so a caller can walk the array and mark every offending field at
once, instead of fixing one, resubmitting, and discovering the next. The
libraries filter builder is the worked example:
`internal/libraryfilter.Validate` returns every issue across a `filters[]`
array in one pass, each pointer rooted at the offending element —
`/filters/1/values/0` for the first value of the second filter — and
`libraries.validateFilters` (`app/router/handlers/libraries/libraries.go`)
appends one more, `/show_artists`, when a hide-artists library's filters
resolve to none. `POST /libraries/preview` (above) answers the identical
`errors[]` shape for the same request-shaped reason: a candidate filter set
can be wrong in more than one place before it is ever saved.

**Status convention, confirmed across every handler:** `422` is
`Writer.WriteValidation` — hard-coded to `http.StatusUnprocessableEntity`,
always a `ValidationDetails`, for a request that is **well-formed but
invalid** (an unknown enum value, a selection over the size cap, an empty
list). Everything else goes through `Writer.Write` and is a plain,
non-itemized `Details` — including
`400` for a **malformed** request (a missing required field, invalid
JSON/multipart). Concretely, on `GET /metadata/pictures/image`: a missing
`slot` is `400` (plain — the request doesn't even name a slot to validate);
a present but unrecognised `slot` is `422` (itemizing `/slot` — the request
is well-formed, the value is wrong).

**Upstream failures:** `Writer.WriteUpstream` maps a failed third-party call
— now made through `github.com/go-bumbu/http/outbound` (see
[architecture.md](architecture.md)'s `outbound` section) — to `429` when the
`*outbound.Error`'s `Kind` is `KindRateLimited` (the provider is
rate-limiting Aether), `504` when it is `KindTimeout` (new in v0.6.0 — every
other upstream failure kind used to share `502`/`upstream_error` with
timeouts), otherwise `502`. `detail` is always `outbound`'s human
`UserMessage()` sentence or a fallback, **never a raw Go error**. In dev
mode only, when the error also exposes an `UpstreamReason() string` method
(`*outbound.Error` does, via its `Kind.String()`), `WriteUpstream` adds it as
a `reason` extension field alongside `detail` — a developer sees the exact
cause (`"timeout"`, `"unreachable"`, `"rejected"`, …) behind the coarse
status; production's masked mode strips `reason` like everything else (see
below).

**Masked bodies in production, and the `reference` correlation id.**
`problems.New(masked bool)` picks the Writer's mode:
`router.Cfg.Production` (sourced from config's `Env.Production`) selects
`masked=true` → `problemjson.ModeMasked`. A masked response collapses
`type`/`title` to a generic `.../probs/error` / "An error occurred" identity
and drops `detail`, the validation `errors[]`, and the upstream `reason` —
only `status`, `instance`, and a new `reference` field remain, so a
production client's error body never leaks internal specifics. `reference`
is filled from the same `Request-Id` every response now carries:
`app/router/requestid.go`'s `requestID` middleware — first in the chain —
honors an inbound `Request-Id` header (e.g. forwarded by Caddy) or mints an
8-byte hex id, sets it on the request (so the logging middleware and
`problems.New`'s `RequestID` extractor read the same value) and echoes it on
the response. That is what lets a masked, detail-free client error still be
traced 1:1 to the full-detail server log line for the same request. Dev mode
(`Production` false, the default) keeps `ModeDev` — full detail, unchanged
from before this option existed.

**Uniform across all of `/api/v0`.** Eight handler packages call
`h.Problems.Write`/`.WriteValidation`/`.WriteUpstream` directly for every
error today: `auth`, `metadata` (its three handler structs — tags, pictures,
identify — each carry their own `Problems` field), `tokens`, `libraries`,
`artists`, `radiobrowser`, `users`, `tasks`. The per-package
`writeError`/`writeErr` shims that once wrapped the old `httperr` package are
gone; every call site names `h.Problems.<Method>` directly (or takes the
`*problemjson.Writer` as an explicit parameter, conventionally named `pw`,
in a package-level helper function). The front-door session/role gate
(`sessionGuard`/`headerGuard` in `app/router/api_v0.go` and
`app/router/proxy_auth.go`, which answer `401`/`403`/`500`) calls
`h.problems.Write` directly too — `MainAppHandler`'s own unexported
`problems` field, the same `*problemjson.Writer` instance every handler's
`Problems` field was threaded from — rather than relying on the router
fallback. The router-level `jsonErrorEnvelope` middleware
(`app/router/errors.go`, wired in `app/router/main.go`) still guarantees the
same `problemjson.Details` shape for any bare plain-text error that reaches
it **on a path under the admin API mount** (`apiV0MountPrefix`,
`"/api/v0"`) — `errorCodeFor` maps the response status to a slug (`401` →
`unauthorized`, `403` → `forbidden`, …), the same threaded Writer's
`TitleFor` maps that slug to its human title, and the plain-text body
becomes `detail` verbatim. That fallback still matters for what's left on
that path: the `/api/v0` catch-all (`api_v0.go`'s `PathPrefix("")`, a bare
`400`) and a stray `http.NotFound` inside an otherwise-migrated handler
(`pictureImage`'s "cell not found" `404`, below). A body that is already a
JSON object (a handler's own `problemjson.Details`, or an ad hoc handler
JSON body) is passed through untouched, so the two mechanisms never
double-wrap each other. `jsonErrorEnvelope` isn't a lesser, non-RFC-9457
fallback to work around — together with the handler packages calling the
threaded Writer directly, it is *how* `/api/v0` stays uniform: every error
response under this mount, handler-authored or not, ends up
`application/problem+json`, with no exceptions — the batch endpoints
described below report their per-row outcomes on a `200`, so they never
author an error response of their own. (Outside the mount — chiefly
`/rest` — the same middleware answers the legacy, pre-RFC-9457
`apiError{error,code}` shape instead; see
[architecture.md](architecture.md)'s error-envelope section — `/rest` must
never speak problem+json.) `middleware.Cfg.JsonErrors` stays `false`
regardless — that flag is go-bumbu's own blind wrapper, unrelated to
`jsonErrorEnvelope`.

`pictureImage`'s "cell not found" `404` (inside the `metadata` package)
takes the router-fallback path rather than calling `h.Problems` directly:
the handler answers Go's bare `http.NotFound` because that endpoint is an
image stream, not a JSON one. The envelope still turns it into the same
`problemjson.Details{type: .../probs/not_found, title: "Not found", detail:
"404 page not found", ...}` shape as everywhere else — `detail` is just Go's
stock message rather than a handler-authored sentence.
`docs/openapi/aether-v0.yaml` documents it as `application/problem+json` like
every other response on that path.

**Batch endpoints: status describes the request, the body describes the
work.** `updateTracks` (`PUT /metadata/tracks`) and its read-only sibling
`rawTags` (`POST /metadata/tracks/raw-tags`), both in
`app/router/handlers/metadata`, act on a list of files and report one
outcome per row — `{results: [{path, ok, error}, ...]}` for `updateTracks`,
`{results: [{path, tags, unsupported, error}, ...]}` for `rawTags` — so a
client can tell which files failed and why. Both follow one rule, and it is
a layering rule rather than a formatting one:

- Everything the *request* needs in order to be processable is checked
  before any row is attempted, and a failure there is an ordinary
  problem+json rejection: malformed JSON or an invalid field combination
  (`400`), a missing `scan_folder` (`400`), a selection over
  `maxSelectionPaths` (`422`), a scan folder that is not configured (`404`).
  Those are the only non-2xx responses these endpoints produce.
- Once the request is accepted the response is **always `200`**, whatever
  happened to the rows — one failed, some failed, or every one of them. A
  per-file failure (unreadable, unwritable, outside the scan folder root) is
  that row's `error`; it never escalates to a transport status, not even
  when the whole batch failed. `updateTracks` writes files incrementally,
  so "N of M written" is the true state of the system after the call, and a
  `200` carrying that ledger is the only honest response — a `5xx` would
  tell a proxy or retry wrapper "nothing happened, safe to retry", which is
  false and would re-write (and re-index) the files that did land.
- A failure of something *every* row depends on — a dependency outage, not
  a bad file — belongs to the first bullet, not the second: hoist it to a
  pre-row problem+json rejection rather than letting the aggregate row
  result bend the status. `identifyAlbum` is the worked example:
  `albumidentify.upstreamFailure` answers `429`/`502` problem+json only when
  every input failed *and* the failures are typed `*outbound.Error` from
  AcoustID — positive evidence that the service, not the files, is down.

Both build the `200` body with the package's own `writeJSON`
(`Content-Type: application/json`). The SPA (`useUpdateTracks` in
`webui/src/composables/useMetadataEditor.ts`) reports "N of M saved, K
failed" from `results[]` alone; its `onError` path is reserved for genuine
request-level and transport failures, which is what `apiErrorMessage` is
built to read.

**Enforcement/reference:** `docs/openapi/aether-v0.yaml`'s
`components.schemas.{Problem,ValidationProblem,FieldError}` and
`components.responses.{BadRequest,NotFound,UnprocessableEntity,TooManyRequests,UpstreamError}`
— all typed `application/problem+json`, with no exception left: every
non-2xx response documented under `/api/v0`, batch endpoints included, is a
Problem.

## Mount-relative paths — model the base through `servers:`

**Rule:** the OpenAPI document models `/api/v0` exclusively via
`servers: [{ url: /api/v0 }]`; every `paths:` key is mount-relative
(`/metadata/pictures/inventory`, never
`/api/v0/metadata/pictures/inventory`). Handlers already do the equivalent
thing structurally: routes are registered on the subrouter returned by
`app.router.PathPrefix("/api/v0").Subrouter()` (`app/router/main.go`), never
with `/api/v0` baked into an individual route string.

**Why:** keeping the base path out of every individual path key means a change
to the mount prefix is a one-line `servers[0].url` edit (plus the matching
one-line mount-prefix change in `app/router/main.go`), not a mechanical rewrite
of every path and `operationId` in the spec. The `/api/v0` version prefix — and
any future bump — lives only there.

**No automated check for this today** — confirmed by hand (grepping the spec)
that no `paths:` key hard-codes `/api/v0` (only `servers[0].url` and prose
may). A Spectral rule forbidding a literal `/api/v0` substring inside any
`paths` key would be a natural follow-up; none exists yet.

## The seed OpenAPI spec — what's specced, what isn't

`docs/openapi/aether-v0.yaml` is the machine-readable contract for
`/api/v0`, but it is a **seed**: today it covers only the metadata editor's
picture/raw-tag surface (the endpoints this doc uses as its worked examples)
plus the shared `Problem`/`ValidationProblem`/`FieldError` components. Not
yet specced — real, mounted endpoints per `app/router/api_v0.go`, just not
described here: `artists`, `users`, `tasks`, `tokens`, `radiobrowser`,
`auth`, `libraries`, `health`/`version`/`me`, and the rest of `/metadata/*`
(`capabilities`, `identify`, `identify-album`, the structured editor's
`PUT /metadata/tracks`). Extending the spec to those groups is the standing
"align v1 with OpenAPI" initiative (`TODO.md`) — when you touch one of them,
consider adding its spec coverage rather than letting the gap widen.

Validate any spec edit with `make spec-lint` (`cd webui && npm run
spec-lint`, which runs `spectral lint ../docs/openapi/aether-v0.yaml -r
../.spectral.yaml --fail-severity=error` — `spectral` is only installed as a
`webui` devDependency, so it must run from there); it runs as part of `make
verify` and in CI (`.github/workflows/spec-lint.yml`). The negative fixtures
that prove the two bounded-URL rules actually fire live under
`docs/openapi/testdata/` and are never linted by `make spec-lint` itself,
which only targets `aether-v0.yaml` — lint them directly (from `webui/`:
`npx spectral lint ../docs/openapi/testdata/<file>.yaml -r
../.spectral.yaml`) if you change either rule and need to re-prove it still
catches a violation.
