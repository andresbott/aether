---
title: "Getting started"
weight: 1
bookToc: false
---

# Getting started

From nothing to music playing in your browser:

1. **Install Aether** — as a Debian package, a container or a plain binary, see
   [Installation](/docs/getting-started/installation.md).
2. **Tell it what to scan.** List your music directories under `ScanFolders`
   in the config file and restart, see
   [Configuration](/docs/getting-started/configuration.md). There is no UI for
   this on purpose: the config file is the only place scan folders are defined.
3. **Run a scan.** Open the web player at `http://<your-server>:8075`, go to
   **Settings → Tasks** and run the catalog scan (the **Scan now** button on
   **Settings → Libraries** does the same). Scans can also run on a schedule
   from the same page.
4. **Play.** Everything scanned shows up under **Library**.

{{< screenshot name="settings-tasks" title="Settings → Tasks, where the catalog scan runs now or on a schedule" >}}

Then, depending on how you listen:

- Split the collection into [libraries](/docs/guides/libraries.md) — lossless
  only, one genre, the audiobooks folder.
- [Connect a phone or desktop app](/docs/guides/connecting-apps.md) through the
  OpenSubsonic API.
- Before anyone else can reach the server, turn on
  [authentication](/docs/guides/authentication.md).
