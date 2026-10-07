---
title: "Getting started"
weight: 1
bookToc: false
---

# Getting started

## Set it up

Aether is a single program with the web player built in: point it at your music, start it, open the browser. Pick Docker or the plain binary below; for the Debian package, and more on each, see [Installation](/docs/getting-started/installation.md).

{{< tabs >}}
{{% tab "Docker" %}}
1. Start the container, replacing `/path/to/music` with your music folder:

   ```sh
   docker run -d --name aether -p 8075:8075 \
     -e AETHER_AUTH_ADMINBOOTSTRAP_PW='change-me' \
     -v aether-data:/var/lib/aether \
     -v /path/to/music:/music:ro \
     ghcr.io/andresbott/aether:latest
   ```

   The image scans whatever you mount at `/music`.

2. Open <http://localhost:8075> and sign in as `admin` with the password above.
{{% /tab %}}
{{% tab "Binary" %}}
1. Download the zip for your system from the [latest release](https://github.com/andresbott/aether/releases/latest) and unzip it.

2. Next to the binary, create a `config.yaml` that lists your music folder:

   ```yaml
   ScanFolders:
     - Name: "Music"
       Path: "/path/to/music"
   ```

3. Start it from that folder:

   ```sh
   ./aether start
   ```

   Its database, cached artwork and logs go in a `data` folder there.

4. Open <http://localhost:8075>. There is no sign-in: authentication is off, so the server only listens on this machine.

On Windows, run `.\aether.exe start`, and put the path in single quotes (`'C:\Music'`): in double quotes, a backslash starts an escape.
{{% /tab %}}
{{< /tabs >}}

Then run the first scan from **Settings → Tasks** (the **Scan now** button on **Settings → Libraries** does the same). Everything it finds shows up under **Library**. The Tasks page can also run scans on a schedule.

{{< screenshot name="settings-tasks" title="Settings → Tasks, where the catalog scan runs now or on a schedule" >}}

To scan more folders later, list them under `ScanFolders` in the config file and restart, see [Configuration](/docs/getting-started/configuration.md). There is no UI for this on purpose: the config file is the only place scan folders are defined.

From here, depending on how you listen:

- Split the collection into [libraries](/docs/guides/libraries.md) — lossless only, one genre, the audiobooks folder.
- [Connect a phone or desktop app](/docs/guides/connecting-apps.md) through the OpenSubsonic API.
- Before anyone else can reach the server, turn on [authentication](/docs/guides/authentication.md).
- Before you move or retag your files, read what a [scan](/docs/guides/scanning.md) keeps — and skim the [known limitations](/docs/getting-started/limitations.md).
