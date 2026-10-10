# Player IPC — commands, events, errors, custom protocol

The Tauri IPC boundary is the app's only UI↔backend interface. Handlers live
in `src-tauri/src/commands/` and are registered in `src-tauri/src/lib.rs`
(`invoke_handler`). Keep handlers thin: logic and its tests belong in
`aether-core`. See [architecture.md](architecture.md). Paths are relative to
`player/`.

Two kinds of traffic cross it. The SPA's requests to the server ride
`tauri-plugin-http` (`.plugin(tauri_plugin_http::init())` in `lib.rs`): its
`fetch` is an IPC call that reqwest executes in Rust with the plugin's cookie
jar — none of our commands is involved, and the capability below scopes it.
Everything else is the commands and events on this page. The SPA reaches both
only through its host layer, `webui/src/lib/host/` (see
[../frontend.md](../frontend.md#desktop-app-mode-libhost)).

## Commands

All fallible commands return `Result<T, AppError>`. Names/args below are the
Rust snake_case; Tauri exposes args camelCased to JS (`api_key` → `apiKey`,
`position_ms` → `positionMs`).

| Group | Command | Notes |
|---|---|---|
| Misc | `app_version()` | returns tauri's package version (`app.package_info().version` — `tauri.conf.json`'s `version`, overridden with the git tag in a release, see [releasing.md](releasing.md)); deliberately not `CARGO_PKG_VERSION`. About shows it as *App* |
| Settings | `get_settings()` | the saved `Settings`, `{"serverUrl": string \| null}`; infallible |
| | `set_server_url(url)` | normalises the address (`aether_core::subsonic::normalize_server_url`: http/https with a host, no credentials, query or fragment; trailing slashes trimmed, port and path prefix kept), writes `settings.json` and answers the saved `Settings` — the SPA adopts that normalised URL. Drops the Rust session: a session belongs to the server it was minted on. `InvalidServerUrl` for a refused address, `Internal` when the file cannot be written (nothing changes then) |
| | `clear_server_url()` | writes and answers the defaults (`{"serverUrl":null}`) and drops the session too; `Internal` when the file cannot be written |
| Session | `set_session(api_key)` | builds `SubsonicClient::with_api_key` for the saved server and stores it in `AppState.connection` — no round trip, nothing validated. `NotConnected` when no server is saved (`InvalidServerUrl` only for a hand-edited address that no longer parses). The SPA calls it after every (re)mint of its `spa` token |
| | `clear_session()` | drops the client; infallible. The SPA calls it on logout, expiry and a server switch |
| | `connection_status()` | tagged enum: `{"status":"disconnected"}` or `{"status":"connected","serverUrl":…}` — tags are lowercase (fixed during the bootstrap; keep them lowercase), `serverUrl` is the normalised address, and there is no `username` (the session is a key, not a login). Unused by the SPA |
| Browsing | `get_artists()`, `get_albums(list_type, size, offset)`, `get_album(id)`, `search(query)` | thin passthrough of Subsonic shapes (`getArtists`, `getAlbumList2`, `getAlbum`, `search3`) — do not invent new shapes. Unused by the SPA, which browses over HTTP through the plugin |
| Playback | `play_track(id)`, `pause()`, `resume()`, `stop()`, `seek(position_ms)`, `set_volume(volume)` | unused while the SPA plays in the webview |
| Queue | `queue_set(ids)`, `queue_next()`, `queue_prev()`, `queue_get()` | `queue_get` returns full `PlayerState`; unused, as above |

The SPA calls six commands, all from `webui/src/lib/host/index.ts`:
`get_settings` (in `initHost`, before the app mounts), `set_server_url`,
`clear_server_url`, `set_session`, `clear_session` and `app_version`.
`connect(server_url, username, password)` and `disconnect()` were removed with
server selection (2026-10-10), together with the keyring and
`connection.json`: the player stores no password
([architecture.md](architecture.md)).

Invariant: **commands that can trigger `PlaybackEngine::load` (`play_track`,
`queue_set`, `queue_next`, `queue_prev`) run via
`tauri::async_runtime::spawn_blocking`** because `load` blocks on a full
track download (v0). Keep any new load-triggering path off the async runtime
threads — see [playback.md](playback.md).

## Events (Rust → UI)

Two events. `tray://command` and its payloads are pinned on both sides —
`events.rs`'s `event_names_and_tray_commands_are_the_ones_the_webview_listens_for`
test and `webui/src/lib/host/events.ts` — because nothing links the two at
build time; `player://state` only by the Rust test, since nothing in the SPA
listens to it yet. `core:default` already allows the webview to listen.

- **`player://state`** (`events::PLAYER_STATE_EVENT`), carrying the full
  `PlayerState` — no deltas, so the UI replaces state and cannot drift.
  Emitted by a dedicated 500 ms poller thread (`spawn_state_emitter`) that
  also drives auto-advance via `player.poll()`; it emits only when state
  changed since the last tick. Nothing in the SPA listens yet: the Rust player
  stays idle while the webview plays.
- **`tray://command`** (`events::TRAY_COMMAND_EVENT`), whose payload is one
  plain string: `"play_pause"` (`TRAY_PLAY_PAUSE`) or `"next"` (`TRAY_NEXT`).
  Emitted by the tray menu's Play / Pause and Next (`tray.rs`), which ask the
  webview to act instead of calling the idle Rust player. The SPA subscribes
  through the host layer's `onTrayCommand` (which drops any other payload),
  from `useTrayBridge` — installed once from `App.vue`, so it keeps working on
  the settings routes, where `PlayerLayout` is unmounted.

`PlayerState` serializes camelCase: `trackId`, `positionMs`, `paused`,
`stopped`, `volume`, `queue` (track-id strings), `queueIndex`.

## Errors

`AppError { code, message }` (`src-tauri/src/error.rs`). The UI is meant to
branch on `code`; codes are a stable contract:
`NotConnected`, `InvalidServerUrl`, `ServerUnreachable`, `AuthFailed`,
`ServerError`, `DecodeError`, `Internal`. `CoreError → AppError` mapping is
tested (`core_errors_map_to_stable_codes`,
`invalid_server_url_serializes_to_its_own_code`) — extend the tests when
adding codes; never rename existing ones. `AppError::internal` is for a
failure that is neither the server's nor the caller's, such as the settings
file not being writable.

Subsonic API error codes 40 (wrong or missing credentials) and 44 (invalid
`apiKey`) map to `CoreError::AuthFailed`; other API errors become
`CoreError::Api` → `ServerError`.

`message` is shown and logged, so it never carries a request URL: a `/rest`
URL holds the API key, and reqwest prints the URL in its errors.
`without_url()` strips it wherever reqwest would — the send error in the core
client's `get()`, `engine/desktop.rs::download`, `protocol.rs`. The HTTP
plugin's own rejections (the SPA's requests, a plain string over IPC) do
include the URL, so the SPA must never put their text on screen or in a log
(see [../frontend.md](../frontend.md#desktop-app-mode-libhost)).

## `aether://` custom protocol

`src-tauri/src/protocol.rs` handles `aether://cover/<id>[?size=N]` by
proxying Subsonic `getCoverArt`. Notes that matter:

- Registered as an *asynchronous* URI scheme protocol; the handler spawns on
  the async runtime and responds via `UriSchemeResponder`.
- Path parsing tolerates the platform difference where the first segment
  arrives as the URI *host* (e.g. `aether://cover/123` on some platforms) —
  keep that host+path chaining if you touch it.
- Response building is panic-free by design (`build_response` falls back to
  an empty 200 instead of unwrapping) — a bootstrap review fix; don't
  reintroduce `.unwrap()` on `Response::builder()`. The 502 body is the
  upstream error built `without_url()`, for the reason above.
- This handler is the designated interception point for the future offline
  cache (check `TrackCache` first, then proxy). The SPA does not use it
  today: covers load straight from the server.

## Capabilities and CSP

`src-tauri/capabilities/default.json` grants the `main` window `core:default`
and `http:default` — the HTTP plugin's fetch commands — with an `allow` scope
of `http://*`, `https://*`, `http://*:*` and `https://*:*`: the server is
whatever address the user types, so the scope cannot name it. The `*:*` pair
is the operative one: verified in the real app, `http://*` does not match an
address with an explicit port such as `localhost:8075`; the port-less pair is
kept as a harmless superset. Least privilege otherwise — adding a Tauri plugin
means adding its permission here, not widening globally. The schemas under
`src-tauri/gen/` regenerate on build and are gitignored.

The window CSP (`app.security.csp` in `src-tauri/tauri.conf.json`):

- `img-src 'self' aether: http://aether.localhost data: http: https:` and
  `media-src 'self' http: https:` — covers and audio load straight from the
  server the user picked, any host, hence the bare schemes; `aether:` and
  `http://aether.localhost` keep the custom protocol loadable.
- `connect-src 'self' ipc: http://ipc.localhost` — IPC only. The plugin's
  requests are IPC calls, so the webview never connects to the server itself,
  and a `fetch`, XHR or `sendBeacon` aimed at the server is blocked: that is
  why every SPA request goes through the host transport, the final play-queue
  save on `pagehide` included (`savePlayQueueBeacon` falls back to it in the
  app).
- A new protocol or resource kind needs a matching CSP change, or it silently
  fails to load.
