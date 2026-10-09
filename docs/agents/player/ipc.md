# Player IPC — commands, events, errors, custom protocol

The Tauri IPC boundary is the app's only UI↔backend interface. Handlers live
in `src-tauri/src/commands/` and are registered in `src-tauri/src/lib.rs`
(`invoke_handler`). Keep handlers thin: logic and its tests belong in
`aether-core`. See [architecture.md](architecture.md). Paths are relative to
`player/`.

## Commands

All fallible commands return `Result<T, AppError>`. Names/args below are the
Rust snake_case; Tauri exposes args camelCased to JS (`server_url` →
`serverUrl`).

| Group | Command | Notes |
|---|---|---|
| Misc | `app_version()` | returns tauri's package version (`app.package_info().version` — `tauri.conf.json`'s `version`, overridden with the git tag in a release, see [releasing.md](releasing.md)); deliberately not `CARGO_PKG_VERSION` |
| Connection | `connect(server_url, username, password)` | builds `SubsonicClient`, validates via `ping`, stores in `AppState.connection`, persists credentials |
| | `connection_status()` | tagged enum: `{"status":"disconnected"}` or `{"status":"connected","serverUrl":…,"username":…}` — tags are lowercase (fixed during the bootstrap; keep them lowercase) |
| | `disconnect()` | clears connection + stored credentials |
| Browsing | `get_artists()`, `get_albums(list_type, size, offset)`, `get_album(id)`, `search(query)` | thin passthrough of Subsonic shapes (`getArtists`, `getAlbumList2`, `getAlbum`, `search3`) — do not invent new shapes |
| Playback | `play_track(id)`, `pause()`, `resume()`, `stop()`, `seek(position_ms)`, `set_volume(volume)` | |
| Queue | `queue_set(ids)`, `queue_next()`, `queue_prev()`, `queue_get()` | `queue_get` returns full `PlayerState` |

Invariant: **commands that can trigger `PlaybackEngine::load` (`play_track`,
`queue_set`, `queue_next`, `queue_prev`) run via
`tauri::async_runtime::spawn_blocking`** because `load` blocks on a full
track download (v0). Keep any new load-triggering path off the async runtime
threads — see [playback.md](playback.md).

## Events (Rust → UI)

One event: **`player://state`** (`events::PLAYER_STATE_EVENT`), carrying the
full `PlayerState` — no deltas, so the UI replaces state and cannot drift.
Emitted by a dedicated 500 ms poller thread (`spawn_state_emitter`) that also
drives auto-advance via `player.poll()`; it emits only when state changed
since the last tick.

`PlayerState` serializes camelCase: `trackId`, `positionMs`, `paused`,
`stopped`, `volume`, `queue` (track-id strings), `queueIndex`.

## Errors

`AppError { code, message }` (`src-tauri/src/error.rs`). The UI is meant to
branch on `code`; codes are a stable contract:
`NotConnected`, `ServerUnreachable`, `AuthFailed`, `ServerError`,
`DecodeError`, `Internal`. `CoreError → AppError` mapping is tested
(`core_errors_map_to_stable_codes`) — extend the test when adding codes;
never rename existing ones.

Subsonic API error code 40 maps to `CoreError::AuthFailed`; other API errors
become `CoreError::Api` → `ServerError`.

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
  reintroduce `.unwrap()` on `Response::builder()`.
- This handler is the designated interception point for the future offline
  cache (check `TrackCache` first, then proxy).
- The CSP in `src-tauri/tauri.conf.json` allowlists `aether:` and
  `http://aether.localhost` for `img-src` — a new protocol or resource kind
  needs a matching CSP change or images silently fail to load.

## Capabilities

`src-tauri/capabilities/default.json` grants only `core:default` to the
`main` window — least privilege. Adding a Tauri plugin means adding its
permission here, not widening globally.
