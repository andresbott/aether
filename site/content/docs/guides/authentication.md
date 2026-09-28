---
title: "Authentication"
weight: 30
---

# Authentication

`Auth.Method` in the config picks one of three modes:

| Method | Who signs you in |
|---|---|
| `none` | nobody — no users, no login. Trusted networks only. **The default.** |
| `native` | Aether: users live in its database and it shows its own login page |
| `proxy-header` | your reverse proxy (Authelia, Authentik, oauth2-proxy), which passes the user on in headers |

In both authenticated modes, users are either admins or regular users. Admins
get **Settings** — libraries, tasks, users — and the metadata editor; everyone
gets their own playlists, stars, play queue and history. Apps never use the
login password, see [Connecting apps](/docs/guides/connecting-apps.md).

## No authentication

With `none`, anyone who can reach the server has full access. To make that
harder to do by accident, the server then listens on `127.0.0.1` only unless
`Server.BindIp` names a specific address, and refuses to start with a wildcard
such as `0.0.0.0`.

## Built-in users

```yaml
Auth:
  Method: "native"
  AdminBootstrap:
    User: "admin"
    Pw: "@/etc/aether/admin.password"   # or a bcrypt hash, or plaintext
```

{{< screenshot name="sign-in" kind="panel" ratio="16 / 10" title="The sign-in page of the built-in users mode" >}}

`AdminBootstrap` creates the first admin, and only while there are no users yet
— changing it later has no effect. Generate a hash for `Pw` with
`aether user hash`, or keep the password in a file only root can read. Admins
manage further users under **Settings → Users**; everyone can change their own
password under **User settings → Account**.

{{< screenshot name="settings-users" title="Settings → Users, where admins add users and set their roles" >}}

After a few wrong passwords, each further login attempt for that user has to
wait a little longer, up to five minutes. There is no hard lockout.

The `aether user` command manages users with the server stopped — also the way
back in when no admin can sign in any more:

```sh
aether user list
aether user create alice --admin
aether user role alice admin
aether user reset-password alice
```

## Behind an authenticating proxy

With `proxy-header`, Aether never shows a login page. It trusts the user and
group headers your proxy adds, creates users the first time it sees them, and
makes members of `AdminGroup` admins — on every request, so group changes in
your identity provider apply immediately.

```yaml
Server:
  BindIp: "127.0.0.1"
Auth:
  Method: "proxy-header"
  ProxyHeader:
    UserHeader: "Remote-User"        # Authelia's defaults; oauth2-proxy uses
    GroupsHeader: "Remote-Groups"    # X-Forwarded-User / X-Forwarded-Groups
    AdminGroup: "aether-admin"
    TrustedProxies:
      - "127.0.0.1"
      - "::1"
```

> [!CAUTION]
> Anyone who can send these headers straight to Aether can sign in as anyone.
> - Aether must be reachable **only through the proxy** — bind it to
>   `127.0.0.1` or an internal network.
> - The proxy must **strip** incoming identity headers from every request.
> - List the proxy in `TrustedProxies`. On the same host, list both
>   `127.0.0.1` and `::1`: a proxy that connects to `localhost` may arrive
>   over IPv6, and its headers are then silently ignored.

**Let `/rest` bypass the proxy's sign-in.** Subsonic apps authenticate on every
request with their own credentials and cannot fill in a login page; without the
bypass they all break at the proxy. In Authelia that is a `bypass` rule for
`/rest` and `/rest/*`. Everything else, including the web player, stays behind
the proxy.
