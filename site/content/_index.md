---
title: "Aether"
layout: landing
description: "A self-hosted music server with a web player and an OpenSubsonic API"
# Hero copy and screenshot (layouts/landing.html). Screenshots are dropped in
# assets/screenshots/ under the name given here — see site/README.md.
tagline: "A self-hosted music server with a web player and an OpenSubsonic API"
notice: "Under active development, with no compatibility guarantees between versions yet."
heroShot:
  name: "library"
  title: "The library: album grid, play queue and player bar"
---

## Try it

One program with the web player built in: start it, point it at your music,
open the browser.

```sh
docker run -d --name aether -p 8075:8075 \
  -e AETHER_AUTH_ADMINBOOTSTRAP_PW='change-me' \
  -v aether-data:/var/lib/aether \
  -v /path/to/music:/music:ro \
  ghcr.io/andresbott/aether:latest
```

Open <http://localhost:8075>, sign in as `admin` with the password above, and
run a scan from **Settings → Tasks**. For the Debian package or the plain binary
for Linux, Windows and macOS, see [Installation](/docs/getting-started/installation.md).

## Highlights

Aether streams your music folders to its own web player and to any Subsonic or
OpenSubsonic app. Beyond that, it does a few things most self-hosted music
servers do not:

{{< features >}}
{{< feature icon="sell" title="A tag editor built in" >}}
Fix tags and cover art in the browser and write them back into your files.
Identify a track, or a whole album at once, by AcoustID fingerprint and
MusicBrainz.
{{< /feature >}}
{{< feature icon="library-music" title="Libraries are filters, not folders" >}}
Lossless only, soundtracks, the audiobooks: views over the collection by folder,
format, release type or genre. An album can sit in several, and deleting one
never touches your music.
{{< /feature >}}
{{< feature icon="explore" title="A Discover feed that learns" >}}
Albums and playlists in one ranked feed, shaped by what you play, favorite and
add, with every fourth slot kept for something you have never played.
{{< /feature >}}
{{< feature icon="devices" title="One API for the web player and your apps" >}}
The web player uses nothing but [OpenSubsonic](https://opensubsonic.netlify.app/),
like Symfonium or DSub do. What the standard lacks is added as extensions any
app can adopt.
{{< /feature >}}
{{< feature icon="key" title="A password per app" >}}
Your own password never goes into an app: each gets its own credentials, revoked
on their own. Sign in with built-in users or through your reverse proxy
(Authelia, oauth2-proxy).
{{< /feature >}}
{{< feature icon="play-circle" title="Pick up where you left off" >}}
The play queue and position follow you from browser to browser, and to apps
that sync the queue: pause on the laptop, carry on from the phone.
{{< /feature >}}
{{< /features >}}
