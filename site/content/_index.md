---
title: "Aether"
layout: landing
description: "A self-hosted music server with a web player and an OpenSubsonic API"
# Hero copy and screenshot (layouts/landing.html). Screenshots are dropped in
# assets/screenshots/ under the name given here — see site/README.md. The
# notice is markdown, next to a "Pre-release" pill.
tagline: "Self-host your music. Listen anywhere."
notice: "Under active development. [More details](/docs/about/pre-release.md)"
heroShot:
  name: "library"
  title: "The library: album grid, play queue and player bar"
---

## Highlights

Aether is a self-hosted music server. It comes with its own web app, and it speaks the [OpenSubsonic](https://opensubsonic.netlify.app/) API, so you can also listen on your phone with apps like Symfonium. On top of that, it does a few things differently:

{{< features >}}
{{< feature icon="sell" title="A built-in metadata editor" >}}
Fix tags and cover art in the browser, and Aether writes the changes straight back to your files. It can also identify songs by their audio fingerprint and look them up on MusicBrainz, one track or a whole album at a time.
{{< /feature >}}
{{< feature icon="devices" title="OpenSubsonic through and through" >}}
The web app browses and plays your music through the same API any other app would use, with no special treatment. When the standard is missing something, Aether adds it as an extension that any app can pick up.
{{< /feature >}}
{{< feature icon="key" title="Apps never see your password" >}}
Each app gets its own login instead, which you can revoke without touching the others. For the web app, sign in with Aether's own accounts or through your reverse proxy (Authelia, Authentik, oauth2-proxy).
{{< /feature >}}
{{< feature icon="library-music" title="Make your own libraries" >}}
Set up a library for lossless only, one for soundtracks, one for the audiobooks. Each one is a saved filter rather than a folder, so an album can show up in several, and deleting a library never touches your files.
{{< /feature >}}
{{< /features >}}
