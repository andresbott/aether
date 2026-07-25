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
  Eight extensions exist today — copy their shape.
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

## Parameter conventions

Helpers in `subsonic.go` — use them instead of raw query reads:
`paramStr`, `paramInt(default)`, `paramStrSlice`, `paramBoolPtr` (nil =
absent, distinguishes "not provided" from `false`), and `paramLibraryID`
(`musicFolderId`; nil = cross-library, matching the store's `*uint` filter
convention).

## Media serving

`media.go`: `stream` serves the original file via `http.ServeFile` (range
requests work; no transcoding). `getCoverArt` resolves, in order: assetstore
image → embedded/folder cover from disk → deterministic generated cover
(`internal/covergen`, cached under `<DataDir>/generated-covers`). Responses
carry `Cache-Control: no-cache`; there is no ETag yet (known TODO — stale
covers after retags are a catalogued bug in TODO.md with root-cause notes).

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
