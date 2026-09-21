# Aether

Aether is a self-hosted music server that indexes your music folders and streams
them over an OpenSubsonic-compatible API. It ships as a single Go binary with the
Vue 3 web player embedded.

> **Under active development.** Pre-release: no compatibility guarantees between
> versions. Authentication is implemented but **off by default** (`Auth.Method:
> "none"`) — set it to `native` or `proxy-header` before exposing a server.

## Features

- **OpenSubsonic API** — browsing, search, streaming, playlists and starring —
  any Subsonic client works.
- **Web player** — queue, near-gapless playback, library browse, playlists and
  internet radio.
- **Scanning** — incremental and full scans of multiple folders, on a cron
  schedule.
- **Libraries** — named views over the scanned catalog, filtered by scan folder,
  folder path, file format, release type, compilation or genre; served to
  Subsonic clients as music folders.
- **Artwork** — embedded and on-disk covers, plus Cover Art Archive and
  fanart.tv lookups.
- **Metadata editor** — edit tags in the browser, identify tracks via
  MusicBrainz.
- **Authentication** — optional: a built-in login with users and roles, or
  identity from an authenticating reverse proxy (Authelia, oauth2-proxy).
  Third-party clients authenticate with per-app tokens.
- **Operations** — SQLite, YAML/env config, Debian packages.

## Configure what gets scanned

List the directories to scan under `ScanFolders:` in `config.yaml` — there is
no UI for this, only the config file:

```yaml
ScanFolders:
  - Name: "Music"
    Path: "/srv/music"
    ExcludePatterns:
      - ".*/covers/.*"   # optional, Go regexes
    FollowSymlinks: true # optional, default true
```

`Name` identifies the folder (tracks remember it); `Path` is the directory to
scan. Restart the server after editing, then run a scan. Removing an entry
removes its tracks at the next scan, along with their stars, playlist
entries and play history. Changing a `Path` keeps its tracks only if the old
location is gone by the next scan (a move); if the old copy still exists,
new rows are created and the old ones are swept instead. Content reached
through a symbolic link is stored under the link's target (with
`FollowSymlinks`, the default); if that target lies outside every listed
`Path`, those tracks are listed but do not play —
list the target directory as a scan folder of its own (it must not contain
another scan folder: roots cannot nest). With no scan folder configured at
all nothing is scanned and no on-disk media is served: a populated index is
still listed, but nothing plays.

See `zarf/localdata/config.yaml` (the dev config `make run` uses) for the full
annotated example and every other setting.

## Libraries are filters

A **library** is what you browse — a named, saved filter over everything that
was scanned, not a directory. Create one under **Settings → Libraries → Add
library**: a name, an icon, the view it opens on, whether it has an Artists
tab, and up to 20 filters:

| Filter | Matches tracks… |
|---|---|
| Scan folder | indexed under one of the chosen `ScanFolders` entries |
| Folder path | stored under one of the chosen directories |
| File format | with one of the chosen extensions (`flac`, `mp3`, …) |
| Release type | on an album of one of the chosen types; "(none)" means untyped albums |
| Compilation | on a compilation album (Yes) or on any other album (No) |
| Genre | tagged with one of the chosen genres |

A track must pass **every** filter; within one filter **any** of its values is
enough. A library without filters is the whole catalog. While you edit, the
dialog shows how many tracks and albums the filters match.

Worth knowing:

- **Scan first.** Formats, genres and release types are picked from what the
  catalog already holds, so before the first scan the pick-lists are empty
  (release type offers only "(none)").
- **Libraries overlap, and deleting one deletes no music.** The same album can
  sit in several libraries; removing a library removes only the view.
- **Lists narrow, detail pages do not.** A "Lossless" library lists only albums
  that have a FLAC track — but an album page shows all of its tracks, MP3
  included, and an artist page shows all of the artist's albums.
- **Renaming a scan folder orphans the filters that name it.** Settings →
  Libraries marks such a library "Needs attention" right away. The filter
  keeps matching the old name's tracks until the next scan re-labels them,
  then matches nothing — and the dialog cannot save the library until that
  value is removed or replaced.
- **A value that leaves the catalog shrinks a library silently.** A genre,
  format or release-type filter keeps the value it was saved with; when no
  track carries it any more — after a retag, say — the library just matches
  less, and the edit dialog marks the value "(not in the catalog)".
- **Folder paths are matched as stored.** Pick directories with *Browse…*. A
  typed path counts only after Enter, and spaces around it are dropped. A
  directory that is a symbolic link, or sits below one, matches nothing:
  tracks reached through a link are stored under the link's target. A path
  filter does not follow a scan folder whose `Path` you change — edit it
  afterwards.

## Development

Tasks are driven by make:

```
make help     # list all targets
make test     # run the go tests
make lint     # run golangci-lint
make verify   # full suite (test, lint, license, benchmark, coverage)
make build    # snapshot binary via goreleaser
```

### Backend and frontend separately

```
make run                  # backend on :8075
cd webui && npm install
cd webui && npm run dev   # Vite dev server, hot reload
```

Open the URL Vite prints; it proxies `/api` and `/rest` to the backend.
`make run-ui` serves the SPA embedded from `:8075`.

## Release

GitHub Actions publishes a release when a version tag is pushed:

```
make tag version="v0.1.0"
```

Needs a clean `main`; run `make verify` first.
