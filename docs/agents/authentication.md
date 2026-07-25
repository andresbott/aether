# Authentication — decided model, do not re-design

Current state: **no auth anywhere**. The SPA is same-origin and calls `/rest`
uncredentialed (`subsonicClient.initWithDefaults()`); the server never
validates Subsonic credentials. The plan below is decided (TODO.md, top
section) — implement it, don't invent alternatives.

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

Two auth *modes* — **builtin** (native login + aether session cookie) and
**proxy-header** (Authelia in front) — differ only in **who establishes
identity for the SPA and `/api/v1`**. Everything downstream is shared and
mode-agnostic: the user table, the PAT system, the token-mint endpoint, and
the `/rest` verifier. **`/rest` is Subsonic-token-only in every mode** — no
cookies, no headers on that surface — so the SPA is just another Subsonic
client that happens to obtain its credential automatically. Auth handlers
compose with OR semantics via `go-bumbu/userauth` (`handlers/auth/chain`).

## Shared machinery (built once, used by both modes)

- **Multi-user table** (playlist `Owner` and the per-entity star direction
  are pre-wired — see architecture.md) with a role column (admin gates radio
  CRUD writes and `/api/v1` admin routes).
- **PAT system** — per-user tokens verified by a thin Subsonic
  `AuthHandler` that parses `u`/`t`/`s`/`p`/`apiKey` on `/rest/*`. This is
  the *only* authentication on `/rest`.
- **Token-mint endpoint** — `POST /api/v1/session/token` (name TBD;
  `/api/v1` is the right home — it's not a music feature). Exchanges
  "whoever the `/api/v1` middleware says you are" for a Subsonic token
  bound to that user. Its authorizer is just the mode's chain — same
  handler, zero mode branching. It trusts **only** the middleware identity;
  no fallback auth of any kind (it's the most sensitive endpoint in the
  model).
- **`/api/v1/me`** — returns `{user, role, authMode}` so the SPA can show
  identity, gate admin UI, and pick the right 401 reaction without
  build-time config.
- **SPA token lifecycle** — on boot, mint a token; keep it in memory; speak
  **standard Subsonic auth** on `/rest` (the dormant `setCredentials` path
  in `webui/src/lib/api/subsonic.ts` already builds `u`/`t`/`s` params).
  Prefer `t`+`s` over raw `apiKey` in URLs so access-log contents are
  non-replayable. On token expiry, re-mint transparently; if the mint call
  itself 401s, the *session* is gone → mode-specific reaction (below).
  Surface "session expired" in the player instead of a generic error when
  a stream's next range request fails.

Token classes (may share a table, must differ in policy):

| | SPA-minted | User-created PAT |
|---|---|---|
| Created | automatically on SPA boot | manually in settings UI |
| Lifetime | short (hours–days), re-minted transparently | long-lived until revoked |
| Management UI | hidden or greyed out | listed, named ("Symfonium on phone"), revocable |
| Storage | may be hash-only (we control the client) | recoverable/encrypted by default |

Sweep expired SPA tokens so the table doesn't grow unbounded.

Why token-only on `/rest` instead of also chaining the session cookie there:
one auth path on the most compliance-sensitive surface in both modes (halves
the test matrix), and CSRF vanishes from `/rest` — Subsonic is full of
GET-with-side-effects (`star`, `deletePlaylist`), which a cookie-authenticated
API would have to defend; tokens moot it. Only `/api/v1` needs CSRF thought.

## Mode: builtin (native login)

Aether is its own identity provider — a built-in equivalent of the Authelia
path. `userauth` ships every piece: `handlers/login.JsonAuthHandler` (JSON
login endpoint, optional TOTP 2FA), `cookieauth.Manager` (gorilla-sessions
cookie; implements the chain `AuthHandler` interface; `LoginUser`/
`LogoutUser`; handlers read identity via `cookieauth.CtxGetUserData`), and
`userstore/dbusers` (GORM-backed user store on aether's SQLite).

| Surface | Protected by |
|---|---|
| `/` (SPA shell) | open (login view is part of the SPA) |
| `/api/v1/auth/login`, `/logout` | the login handler itself |
| `/api/v1` (rest of it) | `cookieauth` session cookie |
| `/rest` | Subsonic PAT/token verifier only |

Flow: SPA shows login page → `POST /api/v1/auth/login` sets the session
cookie → SPA mints its `/rest` token via the cookie-authorized mint endpoint.
On 401 from `/api/v1`: render the login view. Needs user-management UI
(create user, change password) in settings — admin-only.

## Mode: proxy-header (Authelia)

A reverse proxy (Traefik/nginx/Caddy) does forward-auth against Authelia;
authenticated requests reach aether with `Remote-User`, `Remote-Groups`
(comma-separated), `Remote-Name`, `Remote-Email` injected. Aether reads the
headers, resolves/provisions the user, maps groups to roles. Authelia
**replaces the login form, not the token layer**.

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
expired): full-page reload so the portal redirect kicks in. Configure
Authelia to answer non-HTML requests with 401 instead of 302 (it keys off
`Accept`) so the SPA sees a clean failure, not portal HTML.

Mode extras: JIT user provisioning keyed on `Remote-User`; a group-mapping
convention (e.g. `Remote-Groups` containing `aether-admin` → admin role);
an extended header handler — `userauth`'s `handlers/auth/headerauth` exists
but is minimal (single header, groups unimplemented, no context injection,
no proxy-trust check); extending it upstream is the natural path.

### Security invariants (non-negotiable in proxy mode)

1. **Aether must be unreachable except through the proxy** — anyone hitting
   it directly can send `Remote-User: admin` and become anyone. Bind to
   localhost / internal network; deployment docs must say this loudly.
2. **The proxy must strip inbound `Remote-*` headers on every request** —
   especially on the bypassed `/rest` path, where a malicious client could
   smuggle them.
3. `/api/v1` can additionally get a stricter Authelia policy (two_factor,
   group-restricted) for defense in depth.

## Config switch

`none` (current behavior — dev / trusted LAN) / `builtin` / `proxy-header`.
The switch only selects which handler guards `/api/v1` + the SPA shell and
whether login endpoints are mounted. `/rest` is configured identically in
all authenticated modes. `/api/v1/me` reports the active mode so the SPA
reacts correctly to 401s.
