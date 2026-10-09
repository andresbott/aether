# Releasing — tagging, goreleaser, the embedded-UI trap

Releases are goreleaser builds triggered by pushing a `v*.*.*` tag
(`.github/workflows/release.yml`). Aether ships as a **single binary with the
SPA embedded**, so the UI build step is part of every release path. The same
tag also publishes the **desktop player** bundles (`player/`, Tauri) for Linux,
Windows and macOS — see [The desktop player bundles](#the-desktop-player-bundles).

## Cutting a release

    make tag version="v1.2.3"

`make tag` refuses unless you are on `main` (`check-branch`) with a clean
working tree (`check-git-clean`), then (re)creates and pushes the annotated
tag. Note: unlike CI, `make tag` does **not** run `make verify` — run it
yourself first.

## What CI does on a tag

- **Linux + Windows job** (ubuntu-latest): `npm ci` + `make package-ui` (build
  the SPA and copy it into `server/app/spa/files/ui/` — the `//go:embed files/ui/*`
  in `app/spa/spa.go` picks it up), then goreleaser with `.goreleaser.yaml`
  (its builds set `dir: server`, the Go module root)
  (binary name `aether`, `CGO_ENABLED=0`, deb packages, GitHub release,
  container image, the `aether-spa.zip` web UI archive — see below). QEMU + Buildx + a ghcr.io login (the
  built-in `GITHUB_TOKEN`, `packages: write`) run before goreleaser for the
  image.
- **macOS job**: same UI build, goreleaser with `.goreleaser-darwin.yaml`.
- **Player job** (a matrix: `ubuntu-22.04`, `windows-latest`, `macos-latest`
  once per Mac arch), after the Linux job: `npm ci` in `webui/` and `player/`,
  `npx tauri build` with the tag's version, then `gh release upload` of the
  bundles onto the release goreleaser created — see below.
- **Site job**: after the Linux job, for stable tags only (no `-` in the tag),
  builds `site/` with Hugo and deploys it to GitHub Pages — see below.

## Build artifacts

| Build id | Targets | Ships as |
|---|---|---|
| `linux` | linux/amd64 **v1**, linux/arm64 | zip **and** deb |
| `linux-opt` | linux/amd64 **v3** | zip **and** deb |
| `windows` | windows/amd64 v1, v3 | zip |
| `darwin` | darwin/amd64, darwin/arm64 | zip (separate config) |
| — (archive `spa`) | the built web UI, no binary | `aether-spa.zip` |
| player (`release.yml` matrix, not goreleaser) | linux/amd64 | `aether-player_<ver>_amd64.deb`, `…_amd64.AppImage` |
| | windows/x64 | `aether-player_<ver>_x64-setup.exe` (NSIS) |
| | darwin/arm64, darwin/x64 | `aether-player_<ver>_aarch64.dmg`, `…_x64.dmg` |

### GOAMD64: only v1 and v3

Go picks the microarchitecture level at **compile time with no runtime
fallback** — running a binary on a CPU below its level is `SIGILL` at startup,
not graceful degradation. Hence two levels only:

- **v1** — SSE2 baseline, runs on any x86-64 CPU. The safe default.
- **v3** — AVX2/BMI2/FMA: Intel Haswell (2013+), AMD Zen (2017+).

v2 (Nehalem 2008+) is skipped: it adds little over v1 and almost nothing still
running is v2-but-not-v3. **v4 is deliberately excluded** — it requires AVX-512,
which Intel *fuses off* on 12th–15th gen consumer CPUs, so a v4 build would fail
to start on brand-new desktops while working on a 2017 Xeon. Confusing, and the
gain for Go code (single-digit percent; aether's hot paths are SQLite, wasm
taglib and IO) does not justify it.

Both amd64 debs declare `Architecture: amd64` — the field cannot express a
micro-level — and are distinguished by filename via `nfpms[].file_name_template`
(`aether_<ver>_linux_amd64_v1.deb` / `…_amd64_v3.deb`). That is fine for direct
download from a GitHub release. **If these are ever served from an apt
repository, drop the v3 deb or put it in its own suite**, since apt would see
two candidates for the same package+architecture.

## The Debian package

Contents and lifecycle (assets in `server/zarf/packaging/`):

| Path | Notes |
|---|---|
| `/usr/bin/aether` | static binary (v1 or v3, same package name) |
| `/etc/aether/config.yaml` | `config|noreplace` conffile — admin edits survive upgrades; `root:aether 0640` |
| `/lib/systemd/system/aether.service` | runs as `aether:aether`, hardened sandbox |
| `/var/lib/aether` | `DataDir` — DB, generated covers, task logs; `aether:aether 0750` |

- `preinstall` creates the `aether` system user/group (idempotent).
- `postinstall` fixes ownership/modes, then `systemctl enable --now` on **first
  install** or `restart` on upgrade *only if it was already active* — the
  admin's enable/disable choice is respected across upgrades.
- `preremove` stops/disables **only** when `$1 = remove`. It must not stop on
  upgrade: replacing the binary under a running process is safe on Linux, and
  stopping here would make `postinstall`'s `is-active` check see a dead service
  and leave it down after every upgrade.
- `postremove` on `purge` drops the config and the system user, and
  deliberately **keeps `/var/lib/aether`** (prints a notice instead) so a purge
  never destroys someone's catalog database.

Dependency policy: no libc dependency (static binary). Both optional runtime
dependencies are **Recommends**, since apt installs recommends by default and a
plain `apt install aether` should get the full feature set:

- `ffmpeg` provides `ffprobe`, the fallback tag reader
  (`internal/tags/ffprobe.go`) for formats taglib cannot handle; without it the
  scanner degrades to taglib-only.
- `libchromaprint-tools` provides `fpcalc` for AcoustID identification, which
  disables itself when absent (`app/cmd/server.go`). The detection runs **once
  at startup**, so installing fpcalc later needs a restart; until then the
  metadata editor shows Identify greyed out with the reason from
  `/api/v0/metadata/capabilities`.

## The container image

`ghcr.io/andresbott/aether`, linux/amd64 + linux/arm64, pushed by goreleaser's
`dockers_v2` from the release job. Assets in `server/zarf/docker/`: the `Dockerfile`
copies the prebuilt binary from `$TARGETPLATFORM/aether` in goreleaser's build
context (nothing compiles in the image), and `config.yaml` is the image's
`/etc/aether/config.yaml`.

- Built from the **`linux` build id only** (GOAMD64 v1): `linux-opt` is also
  linux/amd64 and would collide on the platform, and an image must run on any
  amd64 host.
- Tags: exact `{{.Version}}` always; `MAJOR.MINOR` and `latest` only for
  non-prerelease tags.
- Base `alpine` + `ffmpeg` + `chromaprint` (the same two optional tools as the
  deb's Recommends), runs as uid/gid 1000 `aether`, `DataDir` is the
  `/var/lib/aether` volume, music is expected at `/music` (one scan folder in
  the image config; more need a mounted config — `ScanFolders` is a list and
  cannot be expressed as flat `AETHER_*` vars).
- Binds `0.0.0.0`, so the image defaults to `Auth.Method: native`: the server
  refuses `none` on a wildcard bind. The admin password comes from
  `AETHER_AUTH_ADMINBOOTSTRAP_PW` on first start.
- The first push creates a **private** ghcr package; making it public is a
  one-time manual step in the package settings.
- Test locally without publishing: `goreleaser release --snapshot --clean`
  builds per-arch images tagged `…-snapshot-amd64` / `-arm64`.

## The web UI archive

`aether-spa.zip` is `webui/dist` plus `LICENSE`, flat (`index.html` at the
root), for serving the SPA separately or embedding it in another shell. It is a
goreleaser `meta` archive in `.goreleaser.yaml` only — the darwin config
appends to the same release and would upload it twice. It is listed in
`checksums.txt`.

- **It is the same build the binary embeds**, so it assumes the embedded
  setup: served at the domain root (Vite `base: '/'`, history-mode router — the
  host needs an `index.html` fallback for unknown paths) with `/rest` and
  `/api/v0` on the same origin. The API origin is a build-time Vite env var
  (`VITE_SERVER_URL_V0`, `VITE_SUBSONIC_SERVER_URL`), unset in releases.
- `webui/dist` comes from the `make package-ui` step; if it is missing the
  release fails ("globbing failed for pattern webui/dist") instead of
  publishing an empty zip.

## The desktop player bundles

The `player` job of `release.yml` builds `player/` with Tauri on GitHub's
runners and attaches the bundles to the release the Linux job created
(`gh release upload --clobber`, so a rerun replaces rather than fails). They
are **not** in `checksums.txt` — that file is goreleaser's. The player-side
view is [player/releasing.md](player/releasing.md); what is easy to get wrong:

- **The version comes from the tag**: `v1.2.3` → `1.2.3`, passed as
  `npx tauri build --config '{"version":"1.2.3"}'` over the `0.1.0` in
  `player/src-tauri/tauri.conf.json`. The app's `app_version` command reads
  that same package version, so the UI and the installer agree.
- **One format per OS, chosen so a prerelease tag still builds**
  (`v1.2.3-rc1` → `1.2.3-rc1`): Linux gets a deb and an AppImage but no rpm (an
  RPM `Version` cannot contain `-`); Windows gets the NSIS installer but no MSI
  (WiX versions are numeric, so tauri rejects `rc1` there); macOS gets one dmg
  per architecture — no universal binary, two smaller downloads, matching the
  server's per-arch darwin zips.
- **Linux builds on `ubuntu-22.04`, deliberately the oldest runner GitHub
  hosts**: the deb and the AppImage link against the runner's glibc and
  webkit2gtk, which sets the oldest system they install on (Debian 12 / Ubuntu
  22.04). Bump it when GitHub retires the image, knowing that raises the floor.
- **Unsigned.** No Apple certificate or Windows signing key is configured, so
  Gatekeeper and SmartScreen warn on first launch (macOS: right-click → Open,
  or `xattr -cr` the app). Wire signing through `tauri.conf.json`'s
  `bundle.macOS` / `bundle.windows` settings and repository secrets when there
  is a certificate.
- `npx tauri build` runs the webui production build itself
  (`beforeBuildCommand: npm --prefix ../webui run build`) and embeds
  `webui/dist`, so the job only needs `npm ci` in both `webui/` and `player/`.
  The Linux runner also needs the native build packages listed in the job
  (webkit2gtk, appindicator, alsa, dbus, `patchelf` for the AppImage).
- Local equivalent: `make player-build` bundles for the host OS into
  `player/target/release/bundle/` with every format the host supports.

## The website

`site/` (Hugo + hugo-book, authoring notes in `site/README.md`) is the public
landing page and user documentation, at `https://andresbott.github.io/aether/`.
`.github/workflows/site.yml` builds and deploys it; `release.yml` calls it as
the `site` job, so the live site tracks the **latest stable release**, not
`main`: a docs fix on `main` goes live with the next release.

- **It always builds the latest stable tag**, whatever ref it runs on: it
  checks out the highest `v*.*.*` tag without a `-` suffix before building.
  So a patch tag on an older release line does not roll the site back, and
  running the Site workflow by hand (from any branch) republishes that tag,
  never `main`. It fails if that tag predates `site/`.
- **Not triggered by the `release` event**: goreleaser publishes the release
  with `GITHUB_TOKEN`, and events caused by that token start no workflows.
  Hence the job inside `release.yml`, `needs: release-linux`.
- **One-time repository setup** (without it the `site` job fails, after the
  release itself is already out): Settings → Pages → Source "GitHub Actions";
  then Settings → Environments → `github-pages` → add a tag rule `v*`. The
  environment only accepts deployments from the default branch by default, so a
  tag run is rejected ("Tag … is not allowed to deploy to github-pages").
- The theme is a Hugo module pinned in `site/go.mod`, which is why the job sets
  up Go. `site/go.mod` is not part of the server module.

## Config resolution

`aether start` with no `-c` probes `./config.yaml` then
`/etc/aether/config.yaml` and uses the first that exists; if neither does, the
built-in defaults apply (`app/cmd/config.go`, `resolveConfigFile`). An explicit
`-c` path is **mandatory** — a typo is an error, never a silent fall-through to
defaults. `AETHER_*` env vars still override everything. The systemd unit passes
`--config /etc/aether/config.yaml` explicitly so a stray `config.yaml` in the
working directory cannot shadow the packaged one.

## Traps

- **The embedded UI is whatever is in `server/app/spa/files/ui` at build time.** A
  local `go build` without a prior `make package-ui` embeds the stale (or
  gitkeep-only) UI. Any release-path change must keep the
  `package-ui → build` ordering.
- **The build is pure Go — `CGO_ENABLED=0` everywhere.** The SQLite driver
  (`glebarez/sqlite`, modernc) and taglib (the wazero/wasm fork) need no C
  toolchain, so every target cross-compiles from any host and the binaries are
  static: no glibc floor, so one artifact runs on Debian 12, older distros and
  musl systems alike. **Linux builds also need the `nodynamic` build tag**:
  `gen2brain/webp` (imagecache) otherwise dlopens libwebp through `purego`,
  which links the binary against glibc *despite* `CGO_ENABLED=0` — it then
  fails with `no such file or directory` on Alpine, including the container
  image. Check with `file dist/linux*/aether` ("statically linked"). If you add a dependency that needs CGO you reintroduce a
  glibc floor (the ubuntu runner's glibc becomes the minimum) and per-target
  cross-compilers — verify `CGO_ENABLED=0 go build` for linux/amd64,
  linux/arm64, windows/amd64 and darwin/{amd64,arm64} before changing this.
- **The systemd sandbox can break features silently.** `ProtectSystem=strict`
  makes everything outside `ReadWritePaths=/var/lib/aether` read-only. Scanning
  works anywhere (reads are unrestricted except `/home`, which
  `ProtectHome=read-only` still allows for reads), but the **metadata editor
  writes tags back into the scanned files** and needs every scan folder's
  `Path` added to `ReadWritePaths` via `systemctl edit aether.service`. `MemoryDenyWriteExecute`
  must stay `false` — wazero JITs the taglib wasm module.
- **`go.mod` replaces taglib with a fork**
  (`go.senan.xyz/taglib` → `github.com/andresbott/go-taglib`). This is a
  module-level replace, so it *does* apply to builds — but it pins a fork;
  version bumps of taglib must go through the fork or remove the replace
  deliberately. The fork is upstream plus `ReadUnsupported` /
  `RemoveUnsupported` (sentriz/go-taglib#28), which the metadata editor's
  hidden-frame removal needs; once that lands upstream the replace can go.
- Version metadata (`app/metainfo`: Version, BuildTime, ShaVer) is injected
  via goreleaser ldflags. The AcoustID app key is a compiled-in constant
  chosen per release line (`metainfo.AcoustIDAppKey`) so usage stats can be
  told apart on acoustid.org — a new major release line needs its own key
  registered there and added to that function. An empty key silently
  disables audio identification by design (optional dependency).
- **No backwards compatibility is promised** (pre-release, no users —
  CLAUDE.md). There are no schema migration guarantees between tags; do not
  add migration code to satisfy a release.
- **The player's bundle identifier `dev.andresbott.aether.player` is
  immutable.** It is the OS keyring service name (a change orphans every
  stored password) and what makes an update the same app to the OS. The
  player's version is tauri's, not the Rust crate's — see above.

See [testing.md](testing.md) for the `make verify` gate and
[architecture.md](architecture.md) for the embed/composition layout.
