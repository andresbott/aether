# Developing Aether

The Go server lives in `server/`, the Vue SPA in `webui/`, the desktop player
(Tauri 2, Rust) in `player/` and the website (landing page + user docs) in
`site/`. Internal docs, including the contributor notes on each subsystem, are
in `docs/agents/` (`docs/agents/player/` for the player).

## Make targets

Tasks are driven by make, from the repo root:

```
make help     # list all targets
make test     # run the go tests
make lint     # run golangci-lint
make verify   # full suite (test, lint, license, benchmark, coverage, ui tests, spec lint, player lint + tests)
make build    # snapshot binary via goreleaser
make player-build  # desktop player bundles for this OS (see below)
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
annotated example of every setting. It scans `server/zarf/locallibrary`
(git-ignored); `make sample-library` fills it with ~1 GB of freely licensed
albums from the Internet Archive.

## Desktop player

`player/` is a Tauri 2 app whose window is the same SPA; the Rust side plays
the audio. It needs a Rust toolchain (rustup, stable) and, on Linux, the
webkit2gtk / appindicator / alsa / dbus development packages —
`make -C player check-deps` names what is missing and the apt line that
installs it. `make verify` runs its lint and tests too, so it needs them as
well.

```
make player-prepare   # tauri CLI + webui dependencies
make run              # the server, for data
make player-run       # vite dev server + the app window, hot reload
make player-build     # bundles for this OS into player/target/release/bundle
```

See [`player/README.md`](player/README.md) and `docs/agents/player/`.

## Website

See [`site/README.md`](site/README.md) for running, writing and updating the
site. It is published to GitHub Pages with every stable release.

## Release

GitHub Actions publishes a release when a version tag is pushed:

```
make tag version="v0.1.0"
```

Needs a clean `main`; run `make verify` first. CI builds the server binaries,
packages and container image, then the desktop player bundles for Linux,
macOS and Windows and attaches them to the same release. See
[`docs/agents/releasing.md`](docs/agents/releasing.md) for what CI builds.
