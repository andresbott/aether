# Aether Player

Desktop music player for Aether: a Tauri 2 app whose window is the same Vue 3
SPA the server embeds (`../webui`) and whose Rust backend remembers the server,
carries the SPA's requests and runs the tray. Linux, macOS and Windows.

> **Pre-release.** The window asks for the server's address first, then signs
> in as the browser does, and plays in the webview for now: moving playback
> into the Rust engine is the open follow-up, and servers behind an auth portal
> (proxy-header) are not supported yet — see `docs/agents/player/features.md`.

## Layout

- `crates/aether-core` — pure Rust core: Subsonic client, queue/player, engine
  trait. Never depends on Tauri.
- `src-tauri` — the Tauri shell: IPC commands, the saved server setting, the
  HTTP plugin the SPA's requests ride, desktop audio engine (rodio), tray,
  events.
- `package.json` — pins the `@tauri-apps/cli` only; the frontend is `../webui`.

## Development

Run everything from the repo root (`make help` lists the targets):

    make player-prepare   # tauri CLI + webui dependencies
    make player-run       # tauri dev: vite dev server + the app window
    make player-lint      # cargo fmt --check + clippy -D warnings
    make player-test      # cargo test --workspace
    make player-build     # production bundles into player/target/release/bundle

`make -C player <target>` runs the same targets without the prefix (`prepare`,
`run`, `lint`, `test`, `fmt`, `build`, `check-deps`). With the Aether server
running locally (`make run`, on :8075), give the window's server step
`http://localhost:8075`.

Linux build prerequisites (Debian/Ubuntu):

    sudo apt-get install -y libwebkit2gtk-4.1-dev build-essential curl wget file \
      libxdo-dev libssl-dev libayatana-appindicator3-dev librsvg2-dev \
      libasound2-dev libdbus-1-dev

`make -C player check-deps` names what is missing.

## Installing the macOS build

The dmg is ad-hoc signed, not notarized (there is no Apple Developer
certificate), so macOS blocks the first launch: "Apple could not verify
aether-player is free of malware". Open the app once, close that dialog, then
in System Settings → Privacy & Security click **Open Anyway** — or run
`xattr -cr /Applications/aether-player.app` once. Windows shows the SmartScreen
warning for the same reason.

Before contributing (human or agent), read `docs/agents/player/architecture.md`.
