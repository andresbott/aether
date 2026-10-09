---
title: "Pre-release status"
weight: 10
# Its first address, linked from the README that ships with v0.12 releases.
aliases:
  - /docs/getting-started/pre-release/
---

# Pre-release status

Aether has no stable release yet. Until it does, any new version may change the configuration file or the database, with no migration from what an older version left behind:

- **Configuration.** Options can be renamed, moved or removed. After an upgrade, compare your config with the [packaged config](https://github.com/andresbott/aether/blob/main/server/zarf/packaging/config.yaml) of the new version.
- **Database.** A new version may not work with an older version's database. The way out is an empty data directory and a fresh scan.
- **Scans.** Keeping your database across an upgrade can call for one Full Catalog Scan, see [After an upgrade](/docs/guides/scanning.md#after-an-upgrade).

## Your files are the source of truth

Aether is built so that losing its database costs you a rescan, not your collection. What it knows about your music comes from the files — tags, embedded and folder covers — and the metadata editor writes its changes back into them. A scan reads all of it again.

## Starting over

Stop Aether, delete everything in its data directory (`DataDir` in the config), start it again and run a scan from **Settings → Tasks**.

```sh
# Debian package
sudo systemctl stop aether
sudo rm -rf /var/lib/aether/*
sudo systemctl start aether
```

```sh
# Container: drop the volume, then start it with the same docker run as before
docker rm -f aether
docker volume rm aether-data
```

With built-in users, sign in as the bootstrap admin again: `AdminBootstrap` creates it whenever there are no users, so the password from the config (or `AETHER_AUTH_ADMINBOOTSTRAP_PW`) works again.

## What does not come back

> [!WARNING]
> Covers and images you uploaded or picked on an album, artist or genre page, and your internet radio stations, exist only in the data directory. Starting over loses them. Covers set in the metadata editor are written into your files and survive.

Everything else you set up in Aether goes with them:

- playlists, favorites and play history — the Discover feed starts learning from scratch;
- users, app passwords and API keys — connected apps need new credentials;
- libraries, the main library setting and task schedules.
