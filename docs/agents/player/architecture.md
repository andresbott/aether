# Player architecture — layering, crates, design decisions

`player/` is a **Tauri 2 desktop music player** (Rust backend + the shared Vue
3 webui) that streams from an OpenSubsonic server — Aether first, but nothing
in it is Aether-specific. The webui controls the app; the Rust backend plays
the music. Bootstrapped 2026-07-22 in its own repository from the design spec
`docs/superpowers/specs/2026-07-22-aether-player-bootstrap-design.md` (the
decision record; where it says "follow-up", check [features.md](features.md)
for current status before assuming it landed), and moved into this repo on
2026-10-09, trading its vendored copy of the webui for the real one.

Paths in `docs/agents/player/` are relative to `player/` unless they start with
`webui/`, `docs/` or `.github/`.

Other player docs: [features.md](features.md) (what exists / what is
deliberately missing), [ipc.md](ipc.md) (command surface, events, errors,
protocol), [playback.md](playback.md) (engine/player concurrency contract —
read before touching audio), [testing.md](testing.md) (gates),
[releasing.md](releasing.md) (bundles, versioning). The server-side docs next
door still rule the UI: [../frontend.md](../frontend.md) for any `webui/`
change, [../subsonic-api.md](../subsonic-api.md) for what the server answers.

## Layering

    webui/  (the shared Vue 3 SPA — vite build, opened as the window; still HTTP-only, NOT yet on IPC)
       |
    src-tauri/  crate `aether-app` — thin Tauri shell
       |   commands/* (IPC handlers)   protocol.rs (aether://)   events.rs
       |   engine/* (per-OS PlaybackEngine impls)   tray.rs   credentials.rs
       v
    crates/aether-core/  pure Rust, NO Tauri dependency
        subsonic/ (HTTP client)   player/ (Player, Queue, engine trait)   cache/ (trait only)

Rules each layer enforces:

- **`aether-core` must never depend on `tauri` or any `tauri-*` crate.**
  `cargo test -p aether-core` runs without linking a webview. It holds the
  Subsonic client, `Player` + `Queue` state machines, the `PlaybackEngine`
  and `StreamUrlResolver` traits, and the `TrackCache` trait.
- **`src-tauri` (crate `aether-app`, lib `aether_app_lib`) is a thin shell.**
  Command handlers translate IPC into core calls; logic and its tests belong
  in core. Engines are selected per platform with `#[cfg(target_os)]` in
  `src-tauri/src/engine/mod.rs` — desktop is real (rodio actor), android/ios
  are compile-clean stubs.
- **The webui is the shared one, not a copy.** `src-tauri/tauri.conf.json`
  points `frontendDist` at `../../webui/dist` (relative to `src-tauri/`) and
  its hooks run the webui's own scripts from `player/`, the directory the
  tauri CLI runs in: `beforeDevCommand: npm --prefix ../webui run dev` (vite
  on :5173, the `devUrl`) and `beforeBuildCommand: npm --prefix ../webui run
  build`. `make build-ui` at the root produces the identical files the Go
  binary embeds. Nothing in `webui/` is player-specific yet: it still talks
  HTTP (axios) to a same-origin server, so inside the app window (a
  `tauri://localhost` / `http://tauri.localhost` origin) it has no server to
  talk to — the `AetherClient` transport-abstraction retrofit is follow-up #1,
  not started. A UI change for the player is a `webui/` change under
  [../frontend.md](../frontend.md)'s rules: it must keep working in the
  browser and keep the SPA origin-agnostic. Do not restructure the webui as a
  side effect of backend work.
- **`player/` never imports from `server/`**, and the server knows nothing
  about it. The only contract between them is the OpenSubsonic API (plus the
  extensions `getOpenSubsonicExtensions` advertises, once the player uses any).

## Key decisions (dated 2026-07-22, spec linked above)

- **Bundle identifier is `dev.andresbott.aether.player` — exact, never
  change.** It doubles as the keyring service name in
  `src-tauri/src/credentials.rs`; changing it silently orphans stored
  passwords.
- **Everything UI↔backend goes through Tauri IPC** (no embedded HTTP server).
  Browsing commands are *thin passthroughs of the Subsonic API shape* — do
  not invent new response shapes; the future webui retrofit must map ~1:1
  from its existing axios calls.
- **Mobile-ready, not mobile-initialized.** `crate-type = ["staticlib",
  "cdylib", "rlib"]`, android/ios engine stubs compile, identifier set — but
  no `tauri android/ios init` has been run (no `gen/android`, `gen/apple`;
  `src-tauri/gen/` is gitignored). Keep it that way until the mobile
  follow-up plans.
- **Credentials:** server URL + username in `connection.json` in the app
  config dir; password only in the OS keyring (`keyring` crate). Never in
  localStorage or plain config files. Connection is restored at startup in
  `lib.rs::run()` setup.
- **Close-to-tray:** window close is intercepted (`prevent_close` + hide) so
  playback continues in the backend; quit only via the tray menu. Linux uses
  **ksni** (StatusNotifierItem) instead of Tauri's tray because Tauri goes
  through libappindicator, which is menu-only — no left-click activate
  callback on KDE/GNOME. Windows/macOS use Tauri's built-in tray. Both live
  in `src-tauri/src/tray.rs` behind `#[cfg]`.
- **State flow is one-way, full-state:** a 500 ms poller thread
  (`events.rs::spawn_state_emitter`) calls `player.poll()` (auto-advance)
  and emits the complete `PlayerState` on `player://state` when it changed.
  No deltas — the UI replaces state and cannot drift.
- **The version is Tauri's, injected from the git tag in CI** — see
  [releasing.md](releasing.md). `app_version` reports it, not
  `CARGO_PKG_VERSION`.

## Domain types worth knowing

- `aether_core::subsonic::SubsonicClient` — cheap to clone (commands clone it
  out of `AppState.connection: Arc<RwLock<Option<SubsonicClient>>>`). Auth is
  per-request salt+token (md5), Subsonic API v1.16.1, client name
  `aether-player`.
- `aether_core::player::Player` — owns a `Mutex<Inner>` (queue, paused,
  stopped, volume) plus `Arc<dyn PlaybackEngine>` and
  `Arc<dyn StreamUrlResolver>`. See [playback.md](playback.md) for the lock
  contract.
- `ConnectionResolver` (`src-tauri/src/state.rs`) implements
  `StreamUrlResolver` over the shared connection — errors `NotConnected`
  when disconnected.
- `CoreError` (core) maps to `AppError { code, message }` (app layer) — see
  [ipc.md](ipc.md).

## Known debt (from the bootstrap final review, 2026-07-22)

- **Whole-file download before playback** — `RodioEngine::load` downloads the
  entire track, and `Player` holds its lock across `load`, so `state()` /
  `poll()` stall during download. Progressive streaming is the accepted
  follow-up fix; the blocking behavior is the accepted v0 design, not a bug.
- `eprintln!` used for logging throughout (poll loop, keyring, tray) — no
  structured logging yet.
- Untested paths: `build_url` with a base-URL path prefix, `cover_art_url`
  with `size: None`, resolver-failure paths in `Player`.
- Follow-up plans (in the spec, not started): webui IPC retrofit, Android
  ExoPlayer/Media3 plugin, iOS AVPlayer plugin, offline cache behind
  `TrackCache` + `aether://` interception.
