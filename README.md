# Aether

Aether is a self-hosted music server that indexes your music folders and streams
them over an OpenSubsonic-compatible API. It ships as a single Go binary with the
Vue 3 web player embedded.

> **Under active development.** Pre-release: no compatibility guarantees between
> versions. Authentication is implemented but **off by default** (`Auth.Method:
> "none"`) — set it to `native` or `proxy-header` before exposing a server.

## Features

- **OpenSubsonic API** — browsing, search, streaming, playlists and starring —
  any Subsonic client works.
- **Web player** — queue, near-gapless playback, library browse, playlists and
  internet radio.
- **Scanning** — incremental and full scans of multiple folders, on a cron
  schedule.
- **Artwork** — embedded and on-disk covers, plus Cover Art Archive and
  fanart.tv lookups.
- **Metadata editor** — edit tags in the browser, identify tracks via
  MusicBrainz.
- **Authentication** — optional: a built-in login with users and roles, or
  identity from an authenticating reverse proxy (Authelia, oauth2-proxy).
  Third-party clients authenticate with per-app tokens.
- **Operations** — SQLite, YAML/env config, Debian packages.

## Development

Tasks are driven by make:

```
make help     # list all targets
make test     # run the go tests
make lint     # run golangci-lint
make verify   # full suite (test, lint, license, benchmark, coverage)
make build    # snapshot binary via goreleaser
```

### Backend and frontend separately

```
make run                  # backend on :8075
cd webui && npm install
cd webui && npm run dev   # Vite dev server, hot reload
```

Open the URL Vite prints; it proxies `/api` and `/rest` to the backend.
`make run-ui` serves the SPA embedded from `:8075`.

## Release

GitHub Actions publishes a release when a version tag is pushed:

```
make tag version="v0.1.0"
```

Needs a clean `main`; run `make verify` first.
