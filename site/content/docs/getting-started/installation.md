---
title: "Installation"
weight: 10
---

# Installation

Every [release](https://github.com/andresbott/aether/releases/latest) ships:

| Download | For |
|---|---|
| `aether_<version>_linux_amd64_v1.deb`, `…_amd64_v3.deb`, `…_linux_arm64.deb` | Debian, Ubuntu and derivatives |
| `ghcr.io/andresbott/aether` | Docker / Podman, amd64 and arm64 |
| `aether_Linux_*.zip`, `aether_Windows_*.zip`, `aether_Darwin_*.zip` | the plain binary, any other system |
| `aether-spa.zip` | the web UI on its own (rarely needed, see below) |

**`v1` or `v3`?** The amd64 builds come in two flavours. `v3` is a little faster and needs a CPU from 2013 (Intel Haswell) or 2017 (AMD Zen) onwards; `v1` runs on any 64-bit x86 CPU. A `v3` build on an older CPU exits immediately with an "illegal instruction" error — pick `v1` when unsure.

## Debian package

```sh
sudo apt install ./aether_<version>_linux_amd64_v1.deb
```

The package installs:

| Path | |
|---|---|
| `/usr/bin/aether` | the server |
| `/etc/aether/config.yaml` | the configuration — your edits survive upgrades |
| `/lib/systemd/system/aether.service` | the service, enabled and started on first install |
| `/var/lib/aether` | the database, cached artwork and task logs |

It also pulls in two optional helpers (apt installs recommended packages by default): `ffmpeg`, whose `ffprobe` reads tags from formats the built-in reader does not handle, and `libchromaprint-tools`, whose `fpcalc` powers track identification in the metadata editor.

> [!IMPORTANT]
> The packaged config starts with authentication **off**, so the server only listens on `127.0.0.1:8075`. To reach it from other machines, either put a reverse proxy in front of it or [turn on authentication](/docs/guides/authentication.md).

The service runs in a systemd sandbox that can read your music anywhere but write only to `/var/lib/aether`. The metadata editor writes tags back into your files, so to use it, allow writes to your music directories:

```sh
sudo systemctl edit aether.service
```

```ini
[Service]
ReadWritePaths=/srv/music
```

Uninstalling with `apt purge` keeps `/var/lib/aether`, so your catalog database survives; delete it by hand if you want it gone.

## Container image

```sh
docker run -d --name aether -p 8075:8075 \
  -e AETHER_AUTH_ADMINBOOTSTRAP_PW='change-me' \
  -v aether-data:/var/lib/aether \
  -v /path/to/music:/music:ro \
  ghcr.io/andresbott/aether:latest
```

The image scans `/music` and has authentication on: sign in as `admin` with the password from `AETHER_AUTH_ADMINBOOTSTRAP_PW`, which is read on the first start only. `ffmpeg` and `chromaprint` are included.

- Drop `:ro` if you use the metadata editor — it writes tags into the files.
- For more than one music folder, or sign-in through a reverse proxy, mount your own config over `/etc/aether/config.yaml`.
- Tags: `latest`, `<major>.<minor>` and the exact version. Pre-releases get only their exact version.

## Plain binary

Unzip the archive for your system, put a `config.yaml` next to the binary (start from the [packaged one](https://github.com/andresbott/aether/blob/main/server/zarf/packaging/config.yaml)) and start it:

```sh
./aether start
```

On Windows, run `aether.exe start`. See [Configuration](/docs/getting-started/configuration.md) for where the config is looked up. For `ffprobe` and `fpcalc`, install ffmpeg and Chromaprint before starting the server; both are optional.

## The web UI archive

The server already serves the web player, so most setups never need `aether-spa.zip`. It is the same web UI as a standalone bundle, for hosting it on a separate web server: it must be served from the root of a domain, fall back to `index.html` for unknown paths, and have Aether's `/rest` and `/api/v0` on the same address (for example through a reverse proxy).
