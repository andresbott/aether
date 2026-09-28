# Developing Aether

The Go server lives in `server/`, the Vue SPA in `webui/` and the website
(landing page + user docs) in `site/`. Internal docs, including the
contributor notes on each subsystem, are in `docs/agents/`.

## Make targets

Tasks are driven by make, from the repo root:

```
make help     # list all targets
make test     # run the go tests
make lint     # run golangci-lint
make verify   # full suite (test, lint, license, benchmark, coverage, ui tests, spec lint)
make build    # snapshot binary via goreleaser
make site-serve  # the website in site/ (Hugo), http://localhost:1313/aether/
```

The Go targets live in `server/Makefile` and the root ones delegate to it;
`cd server && make verify` runs the Go checks alone.

## Backend and frontend separately

```
make run                  # backend on :8075
cd webui && npm install
cd webui && npm run dev   # Vite dev server, hot reload
```

Open the URL Vite prints; it proxies `/api` and `/rest` to the backend.
`make run-ui` serves the SPA embedded from `:8075`.

`make run` uses the dev config `server/zarf/localdata/config.yaml`, the full
annotated example of every setting.

## Website

See [`site/README.md`](site/README.md) for running, writing and updating the
site. It is published to GitHub Pages with every stable release.

## Release

GitHub Actions publishes a release when a version tag is pushed:

```
make tag version="v0.1.0"
```

Needs a clean `main`; run `make verify` first. See
[`docs/agents/releasing.md`](docs/agents/releasing.md) for what CI builds.
