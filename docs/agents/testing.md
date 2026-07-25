# Testing — gates, commands, conventions

The full gate is `make verify` — run it before calling any work done. It runs,
in order: `test` → `ui-test` → `license-check` → `lint` → `benchmark` →
`coverage`, collects all failures, and fails if any target failed. CI runs the
same pieces (`.github/workflows/test.yml`: `make test`, `make coverage`,
`make ui-test`, `make package-ui`; separate golangci-lint and license-check
workflows).

| Target | What it runs |
|---|---|
| `make test` | `go test ./... -cover` — the fast Go suite |
| `make ui-test` | `cd webui && npm test` = `vue-tsc --noEmit` + `vitest run` |
| `make lint` | `golangci-lint run` (v2 config; CI pins v2.12.2 to avoid gosec ruleset skew) |
| `make coverage` | **per-package** total over `./internal/...`, threshold **70%** each |
| `make benchmark` | `go test -bench=. ./...` |
| `make license-check` | `go-licence-detector` against `allowedLicenses.json` / `overrideLicenses.json` |
| `make verify` | all of the above; the definition of done |

- The coverage gate is per `internal/` package, not a repo total. A new
  `internal/` package below 70% fails `verify` — write the tests, don't lower
  `COVERAGE_THRESHOLD`. `app/`, `libs/`, and `webui` are outside this gate.
- Lint policy (`.golangci.yaml`): standard set + `nolintlint`, `gocyclo`
  (min-complexity 20), `nestif`, `gosec`, `dupl`. `nolint` directives must
  name the linter and carry an explanation (`require-explanation: true`).
  The gosec path-traversal suppressions for the media handlers are documented
  in the config with their justification — keep it true (see
  [subsonic-api.md](subsonic-api.md)).

## Go test conventions

- **Real SQLite, not mocks.** Store, handler, and scanner tests construct a
  real `store.Store` over `glebarez/sqlite` (pure-Go, no cgo) with an
  in-memory/temp DB and run `model.Migrate`. New handler tests should mirror
  a sibling `_test.go` (e.g. `subsonic/radio_test.go`) rather than
  introducing a store interface/mock.
- Tests live next to the code; external test packages (`package foo_test`)
  are the norm, with `_internal_test.go` for white-box cases.
- Fixtures: real audio files under `internal/tags/testdata`,
  `internal/metadataedit/testdata`, `internal/covergen/testdata`.
- Test files are excluded from `nestif`, `dupl`, `gosec` — table-driven tests
  with some duplication are fine.
- Optional external binaries (ffprobe, fpcalc) must not break the suite:
  tests that need them skip when unavailable, matching the runtime
  "optional dependency" design ([architecture.md](architecture.md)).

## Frontend test conventions

- Vitest + @vue/test-utils, jsdom; specs in `__tests__/` beside the code.
- `npm test` type-checks first — a TS error fails the suite even with green
  runtime tests.
- View specs follow the checklist in
  `docs/architecture/main-content-view-layout.md` §8 (title, pluralized
  summary absent at zero, actions in header, empty/loading states);
  `RadioView.spec.ts` and `LibraryView.spec.ts` are the templates.
- Stable hook classes (`.edit-action-save`, `.hero-action-play`, …) exist for
  tests — target those, not structural selectors.

## Running the app locally

- `make run` — Go server with debug logging (`AETHER_ENV_LOGLEVEL=debug`),
  default config; serves whatever UI build is currently embedded.
- `make run-ui` — rebuilds the SPA, copies it into `app/spa/files/ui`, then
  runs the server.
- `cd webui && npm run dev` — Vite dev server for UI work (set
  `VITE_SERVER_URL_V1` if not proxying).

See [releasing.md](releasing.md) for the release-time gates.
