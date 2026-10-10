# Player features — what exists, what is deliberately missing

Check here before adding a capability — most gaps are catalogued follow-ups
with a chosen direction in the bootstrap spec
(`docs/superpowers/specs/2026-07-22-aether-player-bootstrap-design.md`) or in
the server-selection decisions D1–D8
([architecture.md](architecture.md#server-selection-decisions)). See
[architecture.md](architecture.md) for layering; paths are relative to
`player/`.

| Feature | Status | Where |
|---|---|---|
| Server selection (login view's server step, Change server in the account menus) | Implemented (2026-10-10) | Rust: `get_settings` / `set_server_url` / `clear_server_url` in `src-tauri/src/commands/settings.rs` normalise and persist the address. SPA: the login view asks for it before anything else, probes it (`webui/src/lib/api/Server.ts`) and saves what answered; *Change server* in `UserMenu` and in the phone's `⋮` (`webui/src/composables/useChangeServer.ts`) signs out and returns to that step — see [../frontend.md](../frontend.md#desktop-app-mode-libhost) |
| Settings and session persistence (no password, no keyring) | Implemented (2026-10-10) | `settings.json` `{ "serverUrl": … }` in the app config dir (`src-tauri/src/settings.rs`), loaded in `lib.rs` setup; the session is `tauri-plugin-http`'s cookie jar (`app_cache_dir()/.cookies`, saved as cookies are set) — see [architecture.md](architecture.md) |
| Rust-side session (the SPA's `/rest` key) | Implemented (2026-10-10) | `set_session` / `clear_session` / `connection_status` in `src-tauri/src/commands/connection.rs`. The SPA pushes its `spa` token after every (re)mint and withdraws it on logout, expiry and a server switch (`webui/src/lib/subsonicSession.ts`); `set_session` builds a `SubsonicClient::with_api_key` for the saved server without a round trip. Only the idle engine and `aether://` consume it today |
| Webui on the app's transport (HTTP through Rust) | Implemented (2026-10-10) | Every `/api/v0` and `/rest` request rides `tauri-plugin-http`'s `fetch` (`webui/src/lib/host/transport.ts`); `<audio>` and `<img>` load straight from the server (CSP in [ipc.md](ipc.md#capabilities-and-csp)). First launch and the switch flows are unit-tested only; the real-window acceptance run is still to come |
| Webui driving playback over IPC (the Rust engine) | **Not implemented** | Follow-up #1, narrowed by server selection to playback, queue, `player://state` and the tray: the SPA plays through its own `<audio>` (`webui/src/composables/usePlayer.ts`) while the Rust `Player` stays idle. A `webui/` plan under [../frontend.md](../frontend.md)'s rules; do not bolt IPC calls into webui components ad hoc — they belong in `webui/src/lib/host/` |
| Proxy-header (portal) servers in the app | **Not supported** ([D7](architecture.md#server-selection-decisions)) | Signing in takes the portal's browser redirect, which the app cannot follow. The server step detects such a server — its `/api/v0/me` does not answer as Aether, but `/rest/ping` answers Aether's envelope — and says "This server signs you in through a web portal, which the desktop app cannot open yet. Open it in a browser, or connect to another server." without saving it; a saved server whose `/me` reports `proxy-header` and no user shows the same words with a *Change server* button (`needsPortal`). Follow-up: login in the system browser + a cookie hand-off into the plugin's jar |
| Metadata editor pictures in the app | **Broken in the app** | Picture previews and the artist image build `<img>` URLs from `apiClient.defaults.baseURL` (`webui/src/lib/api/Metadata.ts:187`, `webui/src/views/settings/metadata-editor/PicturesSection.vue:207`) and copy-picture calls a raw `fetch` (`Metadata.ts:128`); the `/api/v0` picture endpoints are cookie-authenticated, so neither loads from the webview. Fix: fetch through the plugin + object URLs (follow-up). The music UI is unaffected — its images come from `/rest/getCoverArt` |
| Library browsing (artists, albums, album, search) | Implemented, unused by the SPA | `src-tauri/src/commands/library.rs` → `SubsonicClient` (thin Subsonic passthrough). The SPA browses over HTTP through the plugin instead ([D6](architecture.md#server-selection-decisions)); these commands' fate is tracked in `TODO.md`, with the playback retrofit |
| Desktop playback (play/pause/resume/stop/seek/volume) | Implemented, idle | `src-tauri/src/commands/playback.rs` → `Player` → `RodioEngine` (`src-tauri/src/engine/desktop.rs`); nothing calls it while the SPA plays in the webview |
| Queue (set/next/prev/get) + auto-advance | Implemented, idle | `aether-core` `Queue` + `Player::poll()`; poller in `src-tauri/src/events.rs` |
| Player state events to UI | Implemented, no listener yet | `player://state` full-state event, 500 ms poll — see [ipc.md](ipc.md) |
| Cover art via `aether://cover/<id>` | Implemented, unused by the SPA | `src-tauri/src/protocol.rs` proxies Subsonic `getCoverArt`; future cache interception point. The SPA loads covers straight from the server |
| System tray (show/hide, play/pause, next, quit) | Implemented | `src-tauri/src/tray.rs` — Tauri tray on Win/macOS, ksni on Linux |
| Tray Play / Pause and Next drive the webview player | Implemented (2026-10-10) | `tray.rs` emits `tray://command` (`"play_pause"` / `"next"`); `webui/src/composables/useTrayBridge.ts`, installed once from `App.vue`, calls `usePlayer().togglePlayPause()` / `playNext()` — on every layout, the settings routes included |
| Close-to-tray (playback continues in the webview) | Implemented | `on_window_event` in `src-tauri/src/lib.rs` hides the window instead of destroying it, so the webview keeps playing; unverified on macOS, where a hidden WKWebView may be throttled |
| Version reporting | Implemented | `app_version` returns tauri's package version — the tag's in a release ([releasing.md](releasing.md)); About shows it as *App* inside the app |
| Release bundles (deb/AppImage, NSIS, dmg ×2) | Implemented | `.github/workflows/release.yml` `player` job, on every `v*.*.*` tag |
| Progressive streaming | **Not implemented** | v0 downloads the whole file in `RodioEngine::load` (accepted design); progressive streaming is the flagged follow-up — see [playback.md](playback.md) |
| Offline cache | Trait only | `aether_core::cache::TrackCache` + `NoopCache`. Disk impl + `aether://` interception is follow-up #4. Design already accounts for it — implement behind the trait |
| Android playback (ExoPlayer/Media3) | Stub by design | `src-tauri/src/engine/android.rs` compiles, `load`/`seek` return `Engine` errors. Real impl is a Kotlin plugin follow-up; needs `tauri android init` first |
| iOS playback (AVPlayer) | Stub by design | `src-tauri/src/engine/ios.rs`, same pattern; requires macOS |
| Minimize-to-tray refinement | Catalogued | Noted in the bootstrap's review round (its scratch notes were not migrated); today's behavior is close-to-tray via `on_window_event` |
| Code signing (macOS notarization, Windows Authenticode) | **Partial** | The macOS `.app` is ad-hoc signed (`bundle.macOS.signingIdentity: "-"` — no identity, no notarization; Gatekeeper still asks for Open Anyway once), the Windows installer is unsigned; see [releasing.md](releasing.md#cautions) |

## The webui's feature set (shared, out of scope for backend tasks)

The webui is Aether's full web client (albums, artists, genres, playlists,
radio, search, metadata editor, admin views — [../features.md](../features.md)
has its table). Inside the app it talks to the server the user picked, through
the plugin. That holds for `make player-run` too: its window loads the vite
dev server but is still the app, so vite's `/api` and `/rest` proxy is not in
the path — with `make run` serving, give the server step
`http://localhost:8075`. Its player (`webui/src/composables/usePlayer.ts`)
uses two HTML `<audio>` elements with preload/swap; it is the app's player
for now, and the playback retrofit *replaces* it rather than extending it.
Changes to `webui/` belong to webui-scoped plans, not backend plans.

## Do not

- Do not move browsing onto per-endpoint IPC commands: the SPA's data path is
  HTTP through the plugin ([D6](architecture.md#server-selection-decisions)).
  An IPC command that does carry server data keeps the Subsonic API shape — do
  not invent new response shapes.
- Do not add an HTTP server inside the app; IPC (the HTTP plugin included) is
  the only UI↔backend path.
- Do not call `fetch` (or XHR, `sendBeacon`, or an axios instance other than
  `apiClient`) outside the host transport: in the app the window's CSP
  confines `connect-src` to IPC, so a request the webview sends itself never
  reaches the server — and a cross-site one would not carry the session cookie
  anyway. Every request goes through `httpFetch` from `webui/src/lib/host/`
  (`apiClient` uses it as its fetch adapter in the app); the metadata editor's
  raw picture `fetch` is the known casualty.
- Do not import `@tauri-apps/*`, or read `location.origin`, outside
  `webui/src/lib/host/` — `webui/src/lib/host/__tests__/originAgnostic.spec.ts`
  fails the suite when anything does.
- Do not store a password on the Rust side, nor any credential of the
  player's own: it keeps the server address only, and the session lives in
  the plugin's cookie jar and, as the `spa` token, in memory.
- Do not initialize mobile targets (`tauri android/ios init`) outside a
  dedicated mobile plan.
- Do not implement caching outside the `TrackCache` trait; the `aether://`
  protocol is its interception point (the SPA does not use it yet — covers
  load straight from the server).
- Do not vendor, fork or copy the webui into `player/`; the app consumes
  `webui/dist` as built.
