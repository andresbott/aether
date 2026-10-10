# Player testing — gates, layout, conventions

Run everything through the root Makefile; `make verify` includes the two player
gates after the server and webui ones. CI (`.github/workflows/player.yml`, on
pushes to `main` and PRs) runs the same steps on Ubuntu — if it isn't green
locally, it won't be green in CI. See [architecture.md](architecture.md) for
layering. Paths are relative to `player/`.

## Gates

| Command | What it runs |
|---|---|
| `make player-lint` | `cargo fmt --all -- --check` + `cargo clippy --workspace -- -D warnings` |
| `make player-test` | `cargo test --workspace` |
| `make player-prepare` | `npm install` in `player/` (the tauri CLI) and in `webui/` |
| `make player-run` | `npx tauri dev` (vite dev server on :5173 + the app window; `make run` the server for data, then give the window's server step `http://localhost:8075` — the dev window is still the app, so vite's proxy is not in its path) |
| `make player-build` | `npx tauri build` (production bundles, see [releasing.md](releasing.md)) |
| `make -C player check-deps` | names the missing native libraries (Linux) and the apt line that installs them |

The same targets exist in `player/Makefile` without the `player-` prefix.

Policy: **fmt-clean and clippy-clean (`-D warnings`) at every commit** — this
was a global constraint of the bootstrap plan, not aspiration. Run
`make player-lint && make player-test` before calling any change done. Note
clippy runs only on the host target; mobile-stub code paths
(`#[cfg(target_os = "android"/"ios")]`) are *not* lint-checked locally — keep
stubs trivially clean (e.g. the `Default` impls exist for this reason).

Both gates compile `src-tauri`, which embeds `webui/dist` at compile time:
`player/Makefile` builds it (`make build-ui`) when `webui/dist/index.html` is
missing, so a fresh checkout works, but it does not refresh a stale one — that
only matters for `make player-build`, which rebuilds it anyway.

## System prerequisites (Linux)

Building tauri / rodio needs (Debian/Ubuntu): `libwebkit2gtk-4.1-dev
build-essential curl wget file libxdo-dev libssl-dev
libayatana-appindicator3-dev librsvg2-dev libasound2-dev libdbus-1-dev`
(`patchelf` too for AppImage bundles). The runtime libraries alone are not
enough — `make -C player check-deps` probes with pkg-config, which is what the
`*-sys` build scripts do (`webkit2gtk-sys`, `libappindicator-sys`, `alsa-sys`,
`libdbus-sys`). macOS needs Xcode's command line tools; Windows the MSVC build
tools and WebView2 (preinstalled on Windows 10+).

## Where tests live

- **`aether-core`** — the bulk of coverage; fast, no Tauri, no audio device.
  - Subsonic client: `wiremock` mock server; tests assert the auth query
    params of both schemes (`u`, `t`, `s` for password auth; `apiKey` alone,
    never mixed with them, for key auth; `v`, `c`, `f=json` on both), error
    codes 40 and 44 → `AuthFailed`, an unreachable server's error text without
    the key, requests under a path prefix, response parsing, and the
    `normalize_server_url` table (trailing slashes, port and path prefix kept,
    other schemes, a missing scheme or host, query, fragment, credentials).
  - Player/queue: table-of-behavior unit tests with `FakeEngine` /
    `FakeResolver` (see [playback.md](playback.md)) — extend the fakes, don't
    add real-engine deps to core.
- **`src-tauri`** — deliberately thin, mostly contract tests:
  error-code mapping/serialization (`error.rs`, `InvalidServerUrl` and
  `Internal` included), the connection-status JSON shape without `username`
  (`commands/connection.rs`), the settings JSON shape and its file round trip
  — missing or damaged file → defaults (`settings.rs`), and the event names and
  tray payloads (`events.rs`; of those, the webview listens only for
  `tray://command` so far). One exception:
  `engine/desktop.rs` carries three `download` tests — two against a one-shot
  loopback stub (plain `std::net`, no mock crate, no audio device), one
  against a closed port — pinning that the body comes back and that neither a
  failed send nor a bad status leaks the key into the error text. If a handler
  needs real logic tests, that logic probably belongs in core.
- **`webui`** — the shared SPA's suite is the root's `make ui-test`, not a
  player gate; a player-motivated UI change is tested there, under
  [../frontend.md](../frontend.md)'s and [../testing.md](../testing.md)'s
  conventions.

## Conventions

- Serialization shapes are contracts: when a struct crosses the IPC boundary,
  add a serde round-trip/shape test (see the `ConnectionStatus`, `Settings`
  and `AppError` tests) — the webui reads these exact shapes (`Settings` and
  `AppError` today, [ipc.md](ipc.md)), and nothing links the two sides at
  build time. The same goes for event names and payloads: `events.rs`'s test
  pins them, and `webui/src/lib/host/events.ts` mirrors `tray://command` and
  its payloads (`player://state` has no SPA listener yet).
- No audio device in CI: nothing in `cargo test --workspace` may require one.
  Real-audio verification is manual (user acceptance), historically deferred
  explicitly in the bootstrap's task reports. The same holds for the app
  window: the SPA's desktop mode (server step, switch, tray bridge) is
  unit-tested in `webui/` against a faked host layer or `@tauri-apps/*`, and
  the real-window run is user acceptance.
- Known untested paths (catalogued, fine to add tests for): `cover_art_url`
  with `size: None`, resolver-failure paths in `Player`, and the
  settings/session command guarantees — `set_session` answering
  `NotConnected` without a server, `set_server_url` / `clear_server_url`
  dropping the session, a failed settings write leaving both untouched.
