# Player features — what exists, what is deliberately missing

Check here before adding a capability — most gaps are catalogued follow-ups
with a chosen direction in the bootstrap spec
(`docs/superpowers/specs/2026-07-22-aether-player-bootstrap-design.md`). See
[architecture.md](architecture.md) for layering; paths are relative to
`player/`.

| Feature | Status | Where |
|---|---|---|
| Connect to OpenSubsonic server (ping-validated) | Implemented | `connect` / `connection_status` / `disconnect` in `src-tauri/src/commands/connection.rs` |
| Credential persistence (config file + OS keyring) | Implemented | `src-tauri/src/credentials.rs`; restored at startup in `lib.rs` setup |
| Library browsing (artists, albums, album, search) | Implemented | `src-tauri/src/commands/library.rs` → `SubsonicClient` (thin Subsonic passthrough) |
| Desktop playback (play/pause/resume/stop/seek/volume) | Implemented | `src-tauri/src/commands/playback.rs` → `Player` → `RodioEngine` (`src-tauri/src/engine/desktop.rs`) |
| Queue (set/next/prev/get) + auto-advance | Implemented | `aether-core` `Queue` + `Player::poll()`; poller in `src-tauri/src/events.rs` |
| Player state events to UI | Implemented | `player://state` full-state event, 500 ms poll — see [ipc.md](ipc.md) |
| Cover art via `aether://cover/<id>` | Implemented | `src-tauri/src/protocol.rs` proxies Subsonic `getCoverArt`; future cache interception point |
| System tray (show/hide, play/pause, next, quit) | Implemented | `src-tauri/src/tray.rs` — Tauri tray on Win/macOS, ksni on Linux |
| Close-to-tray (playback continues) | Implemented | `on_window_event` in `src-tauri/src/lib.rs` |
| Version reporting | Implemented | `app_version` returns tauri's package version — the tag's in a release ([releasing.md](releasing.md)) |
| Release bundles (deb/AppImage, NSIS, dmg ×2) | Implemented | `.github/workflows/release.yml` `player` job, on every `v*.*.*` tag |
| Webui driving the app over IPC | **Not implemented** | The window opens the shared `webui/` build, which is still HTTP/axios-only and finds no server at a `tauri://` origin — so the shipped bundles do not play music yet. Follow-up #1: `AetherClient` interface + IPC adapter + `coverArtUrl()` + event abstraction, as a `webui/` plan under [../frontend.md](../frontend.md)'s rules. Do not bolt IPC calls into webui components ad hoc |
| Progressive streaming | **Not implemented** | v0 downloads the whole file in `RodioEngine::load` (accepted design); progressive streaming is the flagged follow-up — see [playback.md](playback.md) |
| Offline cache | Trait only | `aether_core::cache::TrackCache` + `NoopCache`. Disk impl + `aether://` interception is follow-up #4. Design already accounts for it — implement behind the trait |
| Android playback (ExoPlayer/Media3) | Stub by design | `src-tauri/src/engine/android.rs` compiles, `load`/`seek` return `Engine` errors. Real impl is a Kotlin plugin follow-up; needs `tauri android init` first |
| iOS playback (AVPlayer) | Stub by design | `src-tauri/src/engine/ios.rs`, same pattern; requires macOS |
| Minimize-to-tray refinement | Catalogued | Noted in the bootstrap's review round (its scratch notes were not migrated); today's behavior is close-to-tray via `on_window_event` |
| Code signing (macOS notarization, Windows Authenticode) | **Partial** | The macOS `.app` is ad-hoc signed (`bundle.macOS.signingIdentity: "-"` — no identity, no notarization; Gatekeeper still asks for Open Anyway once), the Windows installer is unsigned; see [releasing.md](releasing.md#cautions) |

## The webui's feature set (shared, out of scope for backend tasks)

The webui is Aether's full web client (albums, artists, genres, playlists,
radio, search, metadata editor, admin views — [../features.md](../features.md)
has its table). It runs against an HTTP server; in dev the vite dev server
proxies `/api` and `/rest` to `localhost:8075`, which is what `make player-run`
opens, so the window shows the live UI when `make run` is serving — the same
flow the browser dev loop uses. Its player
(`webui/src/composables/usePlayer.ts`) uses two HTML `<audio>` elements with
preload/swap — this will be *replaced*, not extended, by the IPC retrofit.
Changes to `webui/` belong to webui-scoped plans, not backend plans.

## Do not

- Do not invent new IPC response shapes for browsing — keep the Subsonic API
  shape so the retrofit maps 1:1.
- Do not add an HTTP server inside the app; IPC is the only UI↔backend path.
- Do not initialize mobile targets (`tauri android/ios init`) outside a
  dedicated mobile plan.
- Do not implement caching outside the `TrackCache` trait or bypass the
  `aether://` protocol for cover art.
- Do not vendor, fork or copy the webui into `player/`; the app consumes
  `webui/dist` as built.
