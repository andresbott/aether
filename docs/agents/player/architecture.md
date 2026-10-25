# Player architecture — layering, crates, design decisions

`player/` is a **Tauri 2 desktop music player** (Rust backend + the shared Vue
3 webui) that streams from an OpenSubsonic server — Aether first. Nothing in
the Rust side is Aether-specific, but the SPA, and so the app as shipped,
needs Aether: its server step accepts no other server. The webui is the whole
UI and, for now, also the player (its own `<audio>` element); the Rust
backend stores the server the user picked, carries the SPA's HTTP requests,
runs the tray, and holds a playback engine that stays idle until the playback
retrofit moves the music into it. Bootstrapped 2026-07-22 in its own
repository from the design spec
`docs/superpowers/specs/2026-07-22-aether-player-bootstrap-design.md` (the
decision record; where it says "follow-up", check [features.md](features.md)
for current status before assuming it landed), moved into this repo on
2026-10-09, trading its vendored copy of the webui for the real one, and given
server selection on 2026-10-10, whose design decisions D1–D8 are recorded
under [Key decisions](#server-selection-decisions) below, and API-key login on
2026-10-11 (D9 there).

Paths in `docs/agents/player/` are relative to `player/` unless they start with
`webui/`, `server/`, `docs/` or `.github/`.

Other player docs: [features.md](features.md) (what exists / what is
deliberately missing), [ipc.md](ipc.md) (command surface, events, errors,
protocol), [playback.md](playback.md) (engine/player concurrency contract —
read before touching audio), [testing.md](testing.md) (gates),
[releasing.md](releasing.md) (bundles, versioning). The server-side docs next
door still rule the UI: [../frontend.md](../frontend.md) for any `webui/`
change, [../subsonic-api.md](../subsonic-api.md) for what the server answers.

## Layering

    webui/  (the shared Vue 3 SPA — vite build, opened as the window; HTTP via `tauri-plugin-http` in the app, playback still in the webview)
       |
    src-tauri/  crate `aether-app` — thin Tauri shell
       |   commands/* (thin IPC handlers)   session.rs (settings + key + client, the guarantees)   protocol.rs (aether://)
       |   settings.rs (settings.json)   credentials.rs (credentials.json)   cookie_jar.rs (the plugin's jar, owner-only)
       |   events.rs   engine/* (per-OS PlaybackEngine impls)   tray.rs   tauri-plugin-http
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
  binary embeds. The SPA learns it runs inside the app, and reaches Rust, only
  through its host layer (`webui/src/lib/host/`, the only code that imports
  `@tauri-apps/*` or reads the page's origin — see
  [../frontend.md](../frontend.md#desktop-app-mode-libhost)): there the server
  becomes a user setting rather than the page's origin, and every `/api/v0`
  and `/rest` request rides `tauri-plugin-http`'s `fetch`. The few app-only
  controls (the login view's server step, *Change server*, About's App and
  Server rows) are gated on its `isDesktopApp()`. A UI change for the player
  is a `webui/` change under [../frontend.md](../frontend.md)'s rules: it must
  keep working in the browser and keep the SPA origin-agnostic. Do not
  restructure the webui as a side effect of backend work.
- **The SPA's requests leave from Rust, not from the webview.** The window's
  origin is `tauri://localhost` (Linux/macOS) or `http://tauri.localhost`
  (Windows), so a request from the webview to the server is cross-site: it
  would need CORS, and — fatal even with CORS — the server's session cookie
  is `SameSite=Lax` and not `Secure` (LAN installs run plain HTTP,
  `server/app/cmd/session.go`), so the webview never attaches it. The plugin
  sends each request over IPC to reqwest in the Rust process, which keeps its
  own cookie jar: no CORS, and the native login, `/me` and the token mint work
  unchanged, with no server change. Media is not CORS-gated, so `<audio>` and
  `<img>` still load straight from the server with the `apiKey` in their URL;
  the window CSP admits `http:`/`https:` for `img-src` and `media-src` (see
  [ipc.md](ipc.md#capabilities-and-csp)).
- **`player/` never imports from `server/`**, and the server knows nothing
  about it. The only contract between them is the OpenSubsonic API (plus the
  extensions `getOpenSubsonicExtensions` advertises, once the player uses any);
  the SPA's own `/api/v0` and `/rest` calls cross the HTTP plugin as opaque
  requests, which the Rust code neither parses nor shapes.

## Key decisions (dated 2026-07-22 unless marked; the bootstrap spec is linked above)

- **Bundle identifier is `dev.andresbott.aether.player` — exact, never
  change.** Tauri derives the app's config and cache directories from it
  (`app_config_dir()` holds `settings.json`, `app_cache_dir()` the HTTP
  plugin's cookie jar), so changing it silently forgets the saved server and
  the session — and a different identifier is a different app to the OS.
- **Everything UI↔backend goes through Tauri IPC** (no embedded HTTP server).
  Still true since server selection — the HTTP plugin *is* IPC — but the data
  path is now HTTP-shaped, not command-shaped: the SPA keeps its axios/`fetch`
  code and the server's own API shapes, and the plugin executes the requests
  in Rust (2026-10-10, [D2, D6](#server-selection-decisions)). So the
  per-endpoint browsing commands in `commands/library.rs` are unused by the
  SPA, and the bootstrap spec's follow-up #1 (the webui IPC retrofit) narrows
  to playback, queue, state events and the tray. Commands that do cross IPC
  stay *thin passthroughs of the Subsonic API shape* — do not invent new
  response shapes.
- **Mobile-ready, not mobile-initialized.** `crate-type = ["staticlib",
  "cdylib", "rlib"]`, android/ios engine stubs compile, identifier set — but
  no `tauri android/ios init` has been run (no `gen/android`, `gen/apple`;
  `src-tauri/gen/` is gitignored). Keep it that way until the mobile
  follow-up plans.
- **What the player stores (2026-10-10, [D4](#server-selection-decisions);
  amended 2026-10-11, [D9](#server-selection-decisions)):** `settings.json`
  `{ "serverUrl": …, "sessionKind": "cookie" | "apiKey" }` in `app_config_dir()`
  (`src-tauri/src/settings.rs`) — the normalised address of the server the user
  picked and how the SPA signs in to it; written to a temp file and renamed
  over the old one, so a save cut short leaves the old settings; a missing file
  (a first launch) quietly yields the defaults and a damaged one does too, with
  a line on stderr, so it can never keep the app from starting. In a key
  session the API key the user pasted — a revocable PAT, never a password —
  sits beside it in `credentials.json` (`src-tauri/src/credentials.rs`), written
  owner-only (`0600`) on Unix and re-chmodded on every overwrite, loaded at
  startup and deleted on logout, on a refused key, on a server change and on a
  switch back to password sign-in (`set_session_kind` to `cookie`). The cookie
  session is not the player's to store: it is the HTTP plugin's cookie jar
  (`app_cache_dir()/.cookies`, loaded at startup and rewritten whenever a
  response sets a cookie, and once more on exit, so a crash loses at most a
  save still in flight; only persistent cookies are written, and Aether's
  session cookie is one) plus the SPA's 48h `spa` token, pushed to Rust in
  memory (`set_session`) and dropped on logout, expiry or a server change. The
  plugin creates the jar with the default mode; `setup` makes it owner-only
  too (`src-tauri/src/cookie_jar.rs`) — the session cookie in it reaches
  `/api/v0` and can mint keys — and the plugin's in-place rewrites keep that
  mode. **No password is ever stored**: it only travels, in the login and
  change-password requests the plugin carries, and `normalize_server_url`
  refuses an address with credentials in it. The keyring is gone and stays
  gone; at startup `Session::load` (`src-tauri/src/session.rs`) reads the
  settings and the key and connects at once when both name a key session, else
  it starts disconnected until the SPA hands over its token.
- **Close-to-tray:** window close is intercepted (`prevent_close` + hide) so
  playback continues in the webview (the window is hidden, not destroyed);
  quit only via the tray menu. Unverified on macOS so far, where a hidden
  WKWebView may be throttled. The tray's Play / Pause and Next do not touch
  the Rust player (2026-10-10, [D3](#server-selection-decisions)): they emit
  `tray://command` and the SPA's `useTrayBridge` acts on its own player
  ([ipc.md](ipc.md#events-rust--ui)).
  Linux uses **ksni** (StatusNotifierItem) instead of Tauri's tray because
  Tauri goes through libappindicator, which is menu-only — no left-click
  activate callback on KDE/GNOME. Windows/macOS use Tauri's built-in tray.
  Both live in `src-tauri/src/tray.rs` behind `#[cfg]`.
- **State flow is one-way, full-state:** a 500 ms poller thread
  (`events.rs::spawn_state_emitter`) calls `player.poll()` (auto-advance)
  and emits the complete `PlayerState` on `player://state` when it changed.
  No deltas — the UI replaces state and cannot drift. Nothing in the SPA
  listens yet: with playback in the webview the Rust player stays idle, and
  this is the channel the playback retrofit will use.
- **The version is Tauri's, injected from the git tag in CI** — see
  [releasing.md](releasing.md). `app_version` reports it, not
  `CARGO_PKG_VERSION`.

### Server-selection decisions

Taken for server selection on 2026-10-10 (D1–D8) and for API-key login on
2026-10-11 (D9), and numbered because other docs cite them; this list is their
reference. D2–D4, D6 and D7 were confirmed with the maintainer before
implementation. The plan D1–D8 come from
(`docs/superpowers/plans/2026-10-09-player-server-selection.md`) is a local,
git-ignored historical record — nothing here depends on it.

- **D1 — The webview cannot call the server itself.** From the window's
  `tauri://localhost` / `http://tauri.localhost` origin every request is
  cross-site: it would need CORS the server does not send and, fatal even
  with CORS, never carries the session cookie — `SameSite=Lax` and not
  `Secure`, while `SameSite=None` would need `Secure`, i.e. the HTTPS a
  plain-HTTP LAN install lacks. The requests have to leave from somewhere else
  (see Layering above).
- **D2 — Transport: HTTP through Rust, with `tauri-plugin-http`.** Its `fetch`
  serialises the request (a `FormData` body keeps its multipart boundary),
  sends it over IPC to reqwest in the Rust process and hands back a real
  `Response`, with a cookie jar kept in `app_cache_dir()` (rewritten whenever a
  response sets a cookie and once more on exit): no CORS, the native login,
  `/me` and the token mint work unchanged, the session survives a restart, and
  the Go server needed no change. The SPA only takes its base URL from the host
  layer and its `fetch` from `httpFetch`.
- **D3 — Media and playback stay in the webview.** `<audio>` and `<img>` are
  not CORS-gated, so the SPA's own player keeps playing and covers keep
  loading straight from the server (the CSP admits `http:`/`https:` for
  `img-src` and `media-src`). The Rust `Player` stays idle until the playback
  retrofit, and the tray's Play / Pause and Next reach the webview player as
  events.
- **D4 — The player stores the server address, the sign-in kind and, in a key
  session, the pasted API key; never a password.** `settings.json` holds
  `{ "serverUrl", "sessionKind" }`, `credentials.json` the key, owner-only,
  like the cookie jar that already persists the session cookie (made owner-only
  at startup); the password is only posted through the transport, and
  `credentials.rs` is back in this narrower form while the `keyring`
  dependency stays removed (amended by D9).
- **D5 — The app-only controls, all gated on `isDesktopApp()`.** The login card
  hosts the server step — only the address and Connect until a server is
  known, then a muted `Server · host` line with a *Change* link above the
  credentials — the account menus offer *Change server*, and About adds
  *Server* and *App* rows. Nothing else in the UI knows about the app.
- **D6 — Browsing stays on HTTP, not per-endpoint IPC.** This supersedes part
  of the bootstrap spec's follow-up #1: what is left of the IPC retrofit is
  playback, queue, state events and the tray. The browsing passthroughs in
  `commands/library.rs` keep compiling over tested core methods, but nothing
  calls them; their fate is tracked in `TODO.md`, with the playback retrofit.
- **D7 — Superseded by D9 (2026-10-11).** A portal signs in through a browser
  redirect the app cannot follow and keeps `/api/v0` closed to anonymous
  callers, so the server step reads a `/api/v0/me` that does not answer as
  Aether plus a `/rest/ping` answered with Aether's own envelope
  (`"type": "Aether"`; `/rest` is open in every proxy setup) as a portal and
  saves nothing, and a `/me` reporting `proxy-header` without a user shows the
  same message. Portal login — the system browser plus a cookie hand-off into
  the plugin's jar — is a follow-up tracked in `TODO.md`.
- **D8 — Rejected alternatives:** CORS on the server with requests straight
  from the webview (dead on D1's cookie problem, and it would push
  token-shaped credentials into the login form); loading the UI from the
  server's origin (same-origin solves everything, but IPC capability URLs are
  static, the UI's version would follow the server's, and it rules out the
  offline cache); and the full IPC retrofit first (months of work before
  choosing a server did anything visible).
- **D9 — Portal servers are used with an API key, in a `/rest`-only session
  (2026-10-11).** `/rest` is PAT-only whenever auth is on and open in every
  proxy setup, so a key the user creates in the web UI (User settings →
  Connected apps) signs the app in where no password can: the server step saves
  a `portal` probe as a key session, the key card verifies the key with
  `getUser` before Rust stores it, identity and role come from that answer, and
  no request carrying the key ever goes to `/api/v0` — the SPA's `apiClient`
  refuses to send (`sessionKind` in `webui/src/lib/host/`); the password link
  and Connect re-probe `/api/v0/me` anonymously. An admin's key keeps the
  `/rest` admin actions (cover uploads, radio CRUD); everything riding `/api/v0`
  hides behind `useAuth().managementApi`. The option is offered on native
  servers too. A `/rest` error 40 or 44 means the key was revoked or has
  expired: the SPA forgets it and shows the key card again; a media element
  error is no verdict on the key. The browser hand-off D7 described stays a
  low-priority follow-up for administering a portal server from inside the app
  (TODO.md). Spec:
  `docs/superpowers/specs/2026-10-11-player-api-key-login-design.md`, a local,
  git-ignored historical record — nothing here depends on it.

## Domain types worth knowing

- `aether_core::subsonic::SubsonicClient` — cheap to clone (commands clone it
  out of the session's `connection: Arc<RwLock<Option<SubsonicClient>>>`).
  Subsonic API v1.16.1, client name `aether-player`. Two auth schemes, never
  mixed (the server answers a mix with error 43): `with_api_key` sends the
  OpenSubsonic `apiKey` — what `set_session` builds from the SPA's `spa` token
  and what `set_api_key` and `Session::load` build from the stored key,
  the only client the app creates — and `new` signs every request with a fresh
  salt + md5 token (used by the tests only now). Both constructors
  accept only an address `normalize_server_url` would (http/https with a
  host, no credentials, query or fragment); `base_url_str()` returns that
  normalised form. Error texts never carry a request URL, because a `/rest`
  URL carries the key: `without_url()` strips it wherever reqwest would print
  it — the send error in `get()`, the engine's `download`, `protocol.rs`.
- `Session` (`src-tauri/src/session.rs`) — what the settings, key and session
  commands share, held by `AppState` (`src-tauri/src/state.rs`) next to the
  `Player`: the config directory, `settings: RwLock<Settings>`,
  `api_key: RwLock<Option<String>>` (the key from `credentials.json`, mirrored
  in memory) and `connection`, with a lock order: **`settings` → `api_key` →
  `connection`**. `set_session` holds the settings read lock until the client
  is stored, and `set_server_url`/`set_session_kind`/`clear_server_url` hold
  the settings write lock, and the key lock after it, while they persist the
  files and rebuild the client from what is left, so a session can never be
  stored for a server that is no longer the configured one. `set_api_key` and
  `clear_api_key` take the same locks in the same order. It works on a plain
  directory, so its tests drive those guarantees without a Tauri app or an
  audio engine; the commands are one-line wrappers, the ones that write files
  `async` so the disk stays off the main thread.
- `aether_core::player::Player` — owns a `Mutex<Inner>` (queue, paused,
  stopped, volume) plus `Arc<dyn PlaybackEngine>` and
  `Arc<dyn StreamUrlResolver>`. See [playback.md](playback.md) for the lock
  contract.
- `ConnectionResolver` (`src-tauri/src/state.rs`) implements
  `StreamUrlResolver` over the shared connection — errors `NotConnected`
  when disconnected.
- `CoreError` (core) maps to `AppError { code, message }` (app layer) — see
  [ipc.md](ipc.md).

## Known debt (bootstrap final review 2026-07-22, server selection 2026-10-10, API-key login 2026-10-11)

- **Whole-file download before playback** — `RodioEngine::load` downloads the
  entire track, and `Player` holds its lock across `load`, so `state()` /
  `poll()` stall during download. Progressive streaming is the accepted
  follow-up fix; the blocking behavior is the accepted v0 design, not a bug.
- `eprintln!` used for logging throughout (poll loop, settings, tray) — no
  structured logging yet.
- Untested paths: `cover_art_url` with `size: None` and resolver-failure
  paths in `Player`. `build_url` with a path prefix is covered now
  (`requests_keep_the_server_path_prefix`), and so are the settings/session
  guarantees (`session.rs`'s tests).
- Registered but unused by the SPA (server selection, 2026-10-10): the
  browsing commands in `commands/library.rs`, the playback and queue
  commands, and `connection_status`. They still compile against tested core
  client and player methods; each command's fate is tracked in `TODO.md`,
  with the playback retrofit.
- The `TRAY_*` constants in `events.rs` are used only by the
  `#[cfg(desktop)]` tray module, so a mobile build would hit `dead_code`
  under `-D warnings` — gate them when the mobile follow-up lands.
- Follow-up plans (not started): the playback retrofit (the bootstrap spec's
  follow-up #1, narrowed by server selection to playback, queue, state events
  and the tray — the Rust engine plays and the webview's `<audio>` goes),
  administering a portal server from inside the app (D9 narrowed D7's browser
  hand-off to that), the metadata editor's pictures in the app (both in
  [features.md](features.md)), Android ExoPlayer/Media3 plugin, iOS AVPlayer
  plugin, offline cache behind `TrackCache` + `aether://` interception.
