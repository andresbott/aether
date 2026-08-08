# Authentication — decided model, do not re-design

Current state: **both authenticated modes are implemented.** Auth method
`native` (the builtin mode): JSON login + logout on `/api/v1/auth/*`, a cookie
session guard on the rest of `/api/v1`, and a full-app login gate in the SPA.
Auth method `proxy-header`: a reverse proxy (e.g. Authelia) authenticates and
injects identity headers, `headerGuard` (`app/router/proxy_auth.go`) validates
them (optionally against `TrustedProxies` CIDRs), provisions users on first
sight, and derives the role live from the groups header — see the mode section
below. Roles
are **enforced on `/api/v1`**: a public bootstrap set, then a session-scoped
tier (`/api/v1/auth/token`, `/api/v1/auth/tokens[/*]`) accepting any
authenticated role, and the rest defaulting to *admin* (403 otherwise) — the
whole surface is server administration. The SPA mirrors this via `role` in
`/me` (`useAuth().isAdmin` hides the Admin menu entry, the artist-image editor
and radio Discover, and redirects non-admins out of `/settings`). **`/rest`
authenticates exclusively via OpenSubsonic `apiKey` (Personal Access
Tokens)**: hash-only stored tokens (prefix `aether_`, `userauth`'s `pat`
service) verified by a dedicated handler that parses `apiKey` from query
params, with error codes 40 (no credentials), 43 (`apiKey` mixed with
`u`/`p`/`t`/`s`), 44 (invalid key), or 0 (verifier I/O failure). Every
per-user surface — queue, stars, playlists, history — is owner-scoped. The
SPA mints a 48h `spa`-scoped token via `POST /api/v1/auth/token` on boot
(session-scoped guard tier, token in memory only) and re-mints transparently
on expiry (one retry per subsonic call), recovering playback streams via the
audio element's error listener. Users create long-lived `client`-scoped tokens
in the settings UI (UserSettingsView, native mode only). The old Sec-Fetch-Site
CSRF mitigation is removed — tokens on `/rest` moot the concern. The plan
below is decided (TODO.md, top section) — implement it, don't invent
alternatives.

## The (Open)Subsonic auth methods (protocol recap)

Everything rides on query parameters; there is no standard header/Bearer/
cookie auth in the protocol:

1. **Plaintext** — `u` + `p=<password>` (or `p=enc:<hex>`, obfuscation not
   encryption). Legacy, deprecated since API 1.13.0, still widely used.
2. **Salted token** — `u` + `t=md5(secret+salt)` + `s=<client-chosen salt>`,
   fresh salt per request. The de-facto standard for current clients
   (Symfonium, DSub, Ultrasonic). Forces the server to hold a **recoverable**
   secret — hash-only storage cannot recompute the MD5; the algorithm is
   fixed, no negotiation.
3. **API key** — `apiKey=<key>` alone (OpenSubsonic 2024 extension,
   advertised as `apiKeyAuthentication`). Fail-closed: mixing with password
   params is error 43, invalid key error 44, no fallback. Server can store
   only `sha256(key)`, but client adoption is thin — an apiKey-only server
   locks out most current clients.

This is why the PAT plan defaults to **recoverable (encrypted-at-rest)**
tokens with an optional hash-only mode per token (TODO.md).

## The architecture in one paragraph

Two auth *modes* — **native** (builtin login + aether session cookie) and
**proxy-header** (Authelia in front) — differ only in **who establishes
identity for the SPA and `/api/v1`**. Everything downstream is shared and
mode-agnostic: the user table, the PAT system, the token-mint endpoint, and
the `/rest` verifier. **`/rest` is Subsonic-token-only in every mode** — no
cookies, no headers on that surface — so the SPA is just another Subsonic
client that happens to obtain its credential automatically. Auth handlers
compose with OR semantics via `go-bumbu/userauth` (`handlers/auth/chain`).

## Shared machinery (built once, used by both modes)

- **Multi-user table** (playlist `Owner` and the per-entity star direction
  are pre-wired — see architecture.md). Roles are **implemented** on top of
  `userauth` group memberships (`userdb` `user_groups` table): membership in
  the `admin` group (`users.AdminGroup`) makes a user an admin; a user with
  no groups is a regular user. The users CRUD exposes this as a `role`
  field (`"admin"`/`"user"`) and the bootstrapped initial admin is seeded
  into the group. The `/api/v1` guard enforces the admin role (see below).
  On `/rest`, the spec's admin-only endpoints (radio CRUD writes) are gated
  via `subsonic.WithAdminChecker`: the router injects `restAdminChecker`
  (owner login → `users.RoleOf`), handlers call `requireAdmin` (Subsonic
  error 50). nil checker (auth "none") passes everyone. Proxy mode mirrors
  the header-derived role into the DB groups on every `/api/v1` request
  (`resolveProxyIdentity`) so the DB-only `/rest` checker agrees with the
  IdP — `/rest` is proxy-bypassed and carries no identity headers.
- **PAT system** — per-user tokens verified by an `IdentityResolver` the
  router injects into `subsonic.Register`
  (`MainAppHandler.patIdentityResolver`, `app/router/main.go`). It parses
  **only** `apiKey` on `/rest/*`; a request that also carries any of
  `u`/`t`/`s`/`p` is answered with Subsonic error 43 (conflicting auth
  mechanisms) rather than falling back to password auth. This is the *only*
  authentication on `/rest`.
- **Token-mint endpoint** — `POST /api/v1/auth/token` (**implemented**,
  `handlers/tokens`; `/api/v1` is the right home — it's not a music
  feature). Exchanges
  "whoever the `/api/v1` middleware says you are" for a Subsonic token
  bound to that user. Its identity comes from an injected `Caller` resolver
  (the seam): native wires `sessionCaller` (cookie), proxy-header wires
  `proxyCaller` (the guard's context identity) — same handler, zero mode
  branching. It trusts **only** the middleware identity; no fallback auth of
  any kind (it's the most sensitive endpoint in the model).
- **`/api/v1/me`** — **implemented** (`handlers.MeHandler`): returns
  `{authMethod, user, features}` so the SPA can show identity, gate
  feature UI, and pick the right 401 reaction without build-time config.
  `user` carries the session's identity (`{login, role}`), null when
  anonymous; `role` is the derived vertical (`"admin"`/`"user"`, see
  handlers/users `RoleOf`) the SPA gates admin UI on;
  `features.userManagement` reports whether the users CRUD is mounted
  (auth method "native"). The endpoint is deliberately public — the SPA
  bootstraps on it before any login — and it renews the rolling session
  expiry of remember-me sessions.
- **SPA token lifecycle** — on boot, mint a 48h `spa`-scoped token via
  `POST /api/v1/auth/token` (sweeps ALL of the caller's spa tokens first —
  expired and live: the SPA holds exactly one and this mint supersedes it,
  bounding spa tokens at ~1/user so repeated boots cannot hit the cap); keep
  it in memory only (never localStorage). Speak standard Subsonic auth on
  `/rest` via `apiKey=<token>`. Hash-only storage means `apiKey` is the only
  possible transport until recoverable (encrypted-at-rest) tokens land for
  `t`+`s` clients (TODO). On token expiry (subsonic error 40/44),
  re-mint transparently (single-flight, one retry per call); if the mint call
  itself 401s, the *session* is gone → mode-specific reaction (below).
  Surface "session expired" in the player instead of a generic error when
  a stream's next range request fails. Generation counter discards mints
  resolving after logout.

Token classes — implemented in `userauth`'s `pat` service with a `scope` tag
distinguishing behavior:

| | SPA-minted (`spa` scope) | User-created PAT (`client` scope) |
|---|---|---|
| Created | automatically on SPA boot | manually via POST /api/v1/auth/tokens |
| Lifetime | 48h, re-minted transparently | long-lived until revoked |
| Management UI | hidden from list | listed, named, revocable in UserSettingsView |
| Storage | hash-only (we control the client) | hash-only today; recoverable/encrypted TODO for `t`+`s` clients |

Mint-time sweep purges every one of the caller's spa tokens (expired and live);
`GET /api/v1/auth/tokens` excludes `spa` scope from the list. A boot-mint that
keeps failing for a non-401 reason surfaces the login gate, whose purge refetches
`/me` and re-runs the mint watcher — so the SPA caps consecutive failed mints
(`MAX_MINT_ATTEMPTS` in `webui/src/lib/subsonicSession.ts`) and then leaves the
gate up instead of looping. A successful login or mint re-arms the budget.

Why token-only on `/rest` instead of also chaining the session cookie there:
one auth path on the most compliance-sensitive surface in both modes (halves
the test matrix), and CSRF vanishes from `/rest` — Subsonic is full of
GET-with-side-effects (`star`, `deletePlaylist`), which a cookie-authenticated
API would have to defend; tokens moot it. Only `/api/v1` needs CSRF thought.

## Mode: native (builtin login) — session layer implemented

Aether is its own identity provider — a built-in equivalent of the Authelia
path. `userauth` ships every piece: `flow/login` + `flow/login/handlers.JSON`
(the JSON login transport over a password-only policy; TOTP 2FA can be added
to the same flow later), `cookieauth.Manager` (gorilla-sessions cookie;
implements the chain `AuthHandler` interface; `LoginUser`/`LogoutUser`;
handlers read identity via `cookieauth.CtxGetUserData`), and
`userstore/userdb` (GORM-backed user store on aether's SQLite).

| Surface | Protected by |
|---|---|
| `/` (SPA shell) | open (login view is part of the SPA) |
| `/api/v1/auth/login`, `/logout` | the login handler itself |
| `/api/v1` (rest of it) | `cookieauth` session cookie, three tiers: public bootstrap (`/me`, `/health`, `/version`), session-scoped (`/api/v1/auth/token`, `/api/v1/auth/tokens[/*]` — any authenticated role), and admin default (`sessionGuard` in `app/router/api_v1.go`) |
| `/rest` | Subsonic PAT verifier (`apiKey` query param) — error 40/43/44 |

Implementation notes (`app/router/handlers/auth`, `app/cmd/session.go`):

- `POST /api/v1/auth/login` takes `{username, password, sessionRenew}` and
  answers `{done:true}` with the cookie set, or a uniform 401 for every
  credential-shaped failure. `sessionRenew` is the "remember me" bit: it opts
  the session into rolling renewal (24h window renewed on activity, 30-day
  hard cap) instead of the fixed 24h window.
- Cookie keys are generated once and persisted in `<DataDir>/session.keys`
  so sessions survive restarts; the cookie is `SameSite=Lax`, not `Secure`,
  because aether commonly runs plain-HTTP on a LAN.
- The SPA gate lives in `App.vue` + `useAuth()`: `/me` bootstraps, the login
  view replaces the whole app (not a route) while `authMethod` is `native`
  and `user` is null, and an axios interceptor flips a shared
  `sessionExpired` flag on any `/api/v1` 401 so an expired session re-opens
  the gate mid-flight. Logout and session expiry both purge the device
  (`purgeLocalSession` in `useAuth`): stop playback (queue sync unbinds first
  so the emptied queue is not pushed to the server), clear localStorage, and
  reset the query cache.
- The purge's cache reset refetches queries that now 401, so `sessionExpired`
  ends up set after a *deliberate* logout too. `useAuth` therefore also sets
  `explicitLogout` (in `lib/authState.ts`) when the logout starts, and the
  login view shows its "session has expired" note on the derived
  `sessionLostUnexpectedly` — expiry only, never a logout the user asked for.
  Both flags clear on the next successful login.

- `sessionGuard` re-reads the user row on every request and 403s a **disabled**
  user, so the `Enabled` kill-switch closes sessions that are already open — not
  just future logins. Without it a disabled admin kept its cookie and could
  re-enable itself through the same API, and could still mint `/rest` tokens
  (the check sits before the session-scoped tier for exactly that reason).
  `/me` instead reports a disabled user as anonymous: it is public-tier, and
  reporting no identity is what makes the SPA fall back to the login view.
  `pat.Verify` enforces the same flag on `/rest`, so a disabled user is closed
  out on every surface. Matches `headerGuard` in proxy mode.

### Users CRUD guards (native only)

Two refusals in `app/router/handlers/users/users.go` exist because of how
identity is keyed and bootstrapped. Both are load-bearing, not defensive
politeness:

- **The last enabled admin cannot be demoted, disabled or deleted** (409,
  code `last_admin`). This CRUD is the only path that grants the admin role and
  `bootstrapAdmin` re-seeds only while the store is EMPTY, so removing the final
  admin is unrecoverable without editing the DB by hand — a restart does not fix
  it and the `aether user` CLI has no role subcommand. Disabled admins do not
  count towards the quorum: they cannot log in, so leaving one behind is the
  same lockout with extra steps (`isLastEnabledAdmin`).
- **Renaming is refused** (400): owner-keyed data (play queue, stars, playlists,
  history) is keyed on the login STRING, not on `User.ID` — `patIdentityResolver`
  returns `info.LoginID` as the owner — so a rename orphans every owner-keyed row
  under the old key. The data survives in the DB but becomes invisible to its
  owner. `update` accepts a login field equal to the current one (the edit dialog
  submits the field it displays) and rejects any actual change; `UserDialog.vue`
  renders it read-only so the UI does not offer a doomed edit. Lifting this needs
  the owner columns to key on the UUID first (TODO.md, Multi-user).

Validation in `update` happens entirely before the first store write: the
mutations are separate store calls rather than one transaction, so a late
rejection would leave the update half-applied.

Still missing from this mode: change-own-password UI.

## Mode: proxy-header (Authelia) — implemented

A reverse proxy (Traefik/nginx/Caddy) does forward-auth against Authelia;
authenticated requests reach aether with `Remote-User` and `Remote-Groups`
(comma-separated) injected — header names are config
(`Auth.ProxyHeader.UserHeader`/`GroupsHeader`, so oauth2-proxy's
`X-Forwarded-User` is just configuration). Aether reads the headers,
resolves/provisions the user, maps groups to roles. Authelia **replaces the
login form, not the token layer**.

| Surface | Authelia ACL | Aether validates |
|---|---|---|
| `/` (SPA shell) | `one_factor` | nothing — protection exists to trigger the portal redirect |
| `/api/v1` | `one_factor` (+ group rule for admin routes) | trusts injected `Remote-*` headers |
| `/rest` | **`bypass`** | Subsonic PAT/token verifier only |

`/rest` must be bypassed: Subsonic clients authenticate via query params on
every request, cannot perform a portal login, hold no Authelia session, and
mostly cannot attach headers. Without the bypass every third-party client
dies at the proxy. Consequence: on `/rest` Authelia injects nothing and
aether must never consult identity headers there.

Flow: user hits the domain → Authelia portal handles login/2FA → SPA loads
with headers flowing on `/api/v1` → SPA mints its `/rest` token via the
header-authorized mint endpoint. On 401 from `/api/v1` (Authelia session
expired): full-page reload so the portal redirect kicks in (the expiry
watcher in `useAuth` branches on `authMethod === 'proxy-header'`). Configure
Authelia to answer non-HTML requests with 401 instead of 302 (it keys off
`Accept`) so the SPA sees a clean failure, not portal HTML.

Implementation (`app/router/proxy_auth.go`, `app/cmd/users.go
setupProxyAuth`): `userauth`'s `auth/headerauth` handler validates the
headers (configurable names, group parsing, `TrustedProxies` CIDR check) and
`headerGuard` enforces the same three `/api/v1` tiers as native. Users are
**JIT-provisioned** into the same `userdb` on first sight of a new login —
the row exists so PATs have an owner `pat.Verify` can check (and its
`Enabled` flag doubles as aether's kill-switch: a disabled user is 403'd on
`/api/v1` and rejected on `/rest` even while the proxy still authenticates
them); the row's password is a random throwaway. The **role is derived live
from the groups header** (`Auth.ProxyHeader.AdminGroup`, default
`aether-admin`) — DB groups are never consulted, the IdP is authoritative.
Login/logout endpoints and the users CRUD are not mounted;
`features.userManagement` reports false and the SPA hides the logout entry
(`authRequired` is native-only) while token management stays available.

### Security invariants (non-negotiable in proxy mode)

1. **Aether must be unreachable except through the proxy** — anyone hitting
   it directly can send `Remote-User: admin` and become anyone. Bind to
   localhost / internal network; deployment docs must say this loudly.
   `Auth.ProxyHeader.TrustedProxies` (CIDRs) adds defense in depth: when set,
   identity headers are honored only from those peers; when empty, a loud
   startup warning reminds that the deployment alone carries the guarantee.
2. **The proxy must strip inbound `Remote-*` headers on every request** —
   especially on the bypassed `/rest` path, where a malicious client could
   smuggle them. (Aether never consults identity headers on `/rest` — the
   guard is installed on the `/api/v1` subrouter only — but strip them
   anyway.)
3. `/api/v1` can additionally get a stricter Authelia policy (two_factor,
   group-restricted) for defense in depth.

## Config switch

`Auth.Method`: `none` (dev / trusted LAN) / `native` / `proxy-header`
(`app/cmd/config.go`). The switch only selects which handler guards
`/api/v1` + the SPA shell and whether login endpoints are mounted. `/rest`
is configured identically in all authenticated modes. `/api/v1/me` reports
the active mode so the SPA reacts correctly to 401s.

Native extras under `Auth.AdminBootstrap`: `User` / `Pw` seed the initial
admin while the user store is empty (idempotent — `bootstrapAdmin` in
`app/cmd/users.go`). `Pw` may be plaintext or a bcrypt hash (`$2` prefix,
from `aether user hash`). Ignored in the other modes.

Proxy-header extras under `Auth.ProxyHeader`: `UserHeader` (default
`Remote-User`), `GroupsHeader` (default `Remote-Groups`), `AdminGroup`
(default `aether-admin`), `TrustedProxies` (CIDR list, empty = trust every
peer).

`TrustedProxies` must list **both loopback forms** (`127.0.0.1` and `::1`)
when the proxy is co-located: a proxy configured with the hostname
`localhost` dials `::1` first, so a list with only `127.0.0.1` rejects its
headers. The failure is silent and reads as a broken frontend — `/me`
answers `200` with `"user": null`, and in proxy mode the SPA has no login
view to fall back to, so `subsonicReady` never flips and the page stays
blank. When debugging a blank SPA in proxy mode, curl `/api/v1/me` over
`127.0.0.1` and `[::1]` separately: differing answers mean this.
