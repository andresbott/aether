# Aether Player

Desktop music player for Aether and other OpenSubsonic servers: a Tauri 2 app
whose Rust backend plays the music and whose window is the same Vue 3 SPA the
server embeds (`../webui`). Linux, macOS and Windows.

> **Pre-release.** The UI still talks HTTP to a server; driving it over Tauri
> IPC is the open follow-up — see `docs/agents/player/features.md`.

## Layout

- `crates/aether-core` — pure Rust core: Subsonic client, queue/player, engine
  trait. Never depends on Tauri.
- `src-tauri` — the Tauri shell: IPC commands, desktop audio engine (rodio),
  tray, credentials, events.
- `package.json` — pins the `@tauri-apps/cli` only; the frontend is `../webui`.

## Development

Run everything from the repo root (`make help` lists the targets):

    make player-prepare   # tauri CLI + webui dependencies
    make player-run       # tauri dev: vite dev server + the app window
    make player-lint      # cargo fmt --check + clippy -D warnings
    make player-test      # cargo test --workspace
    make player-build     # production bundles into player/target/release/bundle

`make -C player <target>` runs the same targets without the prefix (`prepare`,
`run`, `lint`, `test`, `fmt`, `build`, `check-deps`).

Linux build prerequisites (Debian/Ubuntu):

    sudo apt-get install -y libwebkit2gtk-4.1-dev build-essential curl wget file \
      libxdo-dev libssl-dev libayatana-appindicator3-dev librsvg2-dev \
      libasound2-dev libdbus-1-dev

`make -C player check-deps` names what is missing.

Before contributing (human or agent), read `docs/agents/player/architecture.md`.
