---
title: "Connecting apps"
weight: 20
---

# Connecting apps

Aether speaks the [OpenSubsonic](https://opensubsonic.netlify.app/) API, so Subsonic and OpenSubsonic apps such as Symfonium can use it. In the app, enter your server's address, e.g. `https://music.example.com` or `http://192.168.1.10:8075`, and the credentials below.

> [!IMPORTANT]
> Aether answers in JSON only. Apps that understand only the original Subsonic XML responses, such as DSub, cannot connect yet.

## Credentials

**With authentication off** (`Auth.Method: none`), the API asks for nothing: any username and password work.

**With authentication on**, your own password never works in an app. Each app gets its own credentials instead, so you can revoke one without touching the others. Open **User settings → Connected apps** from your account menu:

- **Connect a music app** creates a username and password for the app's login form. This works with practically every Subsonic app.
- **Create an API key** creates a single key, for OpenSubsonic apps and your own scripts that support API-key sign-in.

Both are shown **once** — copy them into the app right away. The same page lists every app with access and revokes it, and also lists the browsers you are signed in to the web player from.

{{< screenshot name="connected-apps" title="User settings → Connected apps, with an app's new credentials shown once" >}}

> [!WARNING]
> Subsonic apps sign every request in a way that can be replayed: over plain HTTP, anyone who captures one request can keep using that app's access until you revoke it. Use HTTPS for anything beyond your home network.

## Behind an authenticating proxy

If a proxy such as Authelia protects Aether, it must let `/rest` through without its own sign-in: apps cannot fill in a login page. See [Authentication](/docs/guides/authentication.md#behind-an-authenticating-proxy).

## What apps cannot do yet

Apps offer a few features Aether does not serve yet. Most of them come up empty, or with an error:

- **Transcoding.** Files are streamed as they are, so an app's bitrate or format setting has no effect, and listening away from home uses the file's full bitrate.
- **Ratings.** An app can set them and is told it worked, but they are not saved.
- **Artist and album information** — biographies, similar artists, album notes — and **top or similar songs**, so mixes an app builds from them stay empty too.
- **Lyrics**, even when your files carry them.
- **Podcasts, jukebox mode, bookmarks, sharing and chat.** To resume, the play queue keeps your position in the current track instead of a bookmark.
- **Managing users and changing passwords.** Do that in the web player, see [Authentication](/docs/guides/authentication.md).

Two things also work differently from what an app may expect:

- **"Now playing" counts as a play.** Some apps report a track as it starts and again once it has been played; Aether records both, so their plays can count twice in your history and in the Discover feed.
- **Plays stay in Aether.** They are not passed on to Last.fm or ListenBrainz; if your app can scrobble to them itself, let it.
