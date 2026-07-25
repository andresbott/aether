# Releasing — tagging, goreleaser, the embedded-UI trap

Releases are goreleaser builds triggered by pushing a `v*.*.*` tag
(`.github/workflows/release.yml`). Aether ships as a **single binary with the
SPA embedded**, so the UI build step is part of every release path.

## Cutting a release

    make tag version="v1.2.3"

`make tag` refuses unless you are on `main` (`check-branch`) with a clean
working tree (`check-git-clean`), then (re)creates and pushes the annotated
tag. Note: unlike CI, `make tag` does **not** run `make verify` — run it
yourself first.

## What CI does on a tag

- **Linux + Windows job** (ubuntu-latest): `npm ci` + `make package-ui` (build
  the SPA and copy it into `app/spa/files/ui/` — the `//go:embed files/ui/*`
  in `app/spa/spa.go` picks it up), then goreleaser with `.goreleaser.yaml`
  (binary name `aether`, `CGO_ENABLED=0`, deb packages, GitHub release).
- **macOS job**: same UI build, goreleaser with `.goreleaser-darwin.yaml`.

## Build artifacts

| Build id | Targets | Ships as |
|---|---|---|
| `linux` | linux/amd64 **v1**, linux/arm64 | zip **and** deb |
| `linux-opt` | linux/amd64 **v3** | zip **and** deb |
| `windows` | windows/amd64 v1, v3 | zip |
| `darwin` | darwin/amd64, darwin/arm64 | zip (separate config) |

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

Contents and lifecycle (assets in `zarf/packaging/`):

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
  never destroys someone's library database.

Dependency policy: no libc dependency (static binary). `ffmpeg` is
**Recommends** — it provides `ffprobe`, the fallback tag reader
(`internal/tags/ffprobe.go`) for formats taglib cannot handle; without it the
scanner degrades to taglib-only. `libchromaprint-tools` is **Suggests** — it
provides `fpcalc` for AcoustID identification, which disables itself when
absent (`app/cmd/server.go`).

## Config resolution

`aether start` with no `-c` probes `./config.yaml` then
`/etc/aether/config.yaml` and uses the first that exists; if neither does, the
built-in defaults apply (`app/cmd/config.go`, `resolveConfigFile`). An explicit
`-c` path is **mandatory** — a typo is an error, never a silent fall-through to
defaults. `AETHER_*` env vars still override everything. The systemd unit passes
`--config /etc/aether/config.yaml` explicitly so a stray `config.yaml` in the
working directory cannot shadow the packaged one.

## Traps

- **The embedded UI is whatever is in `app/spa/files/ui` at build time.** A
  local `go build` without a prior `make package-ui` embeds the stale (or
  gitkeep-only) UI. Any release-path change must keep the
  `package-ui → build` ordering.
- **The build is pure Go — `CGO_ENABLED=0` everywhere.** The SQLite driver
  (`glebarez/sqlite`, modernc) and taglib (the wazero/wasm fork) need no C
  toolchain, so every target cross-compiles from any host and the binaries are
  static: no glibc floor, so one artifact runs on Debian 12, older distros and
  musl systems alike. If you add a dependency that needs CGO you reintroduce a
  glibc floor (the ubuntu runner's glibc becomes the minimum) and per-target
  cross-compilers — verify `CGO_ENABLED=0 go build` for linux/amd64,
  linux/arm64, windows/amd64 and darwin/{amd64,arm64} before changing this.
- **The systemd sandbox can break features silently.** `ProtectSystem=strict`
  makes everything outside `ReadWritePaths=/var/lib/aether` read-only. Scanning
  works anywhere (reads are unrestricted except `/home`, which
  `ProtectHome=read-only` still allows for reads), but the **metadata editor
  writes tags back into library files** and needs the library root added to
  `ReadWritePaths` via `systemctl edit aether.service`. `MemoryDenyWriteExecute`
  must stay `false` — wazero JITs the taglib wasm module.
- **`go.mod` replaces taglib with a fork**
  (`go.senan.xyz/taglib` → `github.com/andresbott/go-taglib`). This is a
  module-level replace, so it *does* apply to builds — but it pins a fork;
  version bumps of taglib must go through the fork or remove the replace
  deliberately.
- Version metadata (`app/metainfo`: Version, BuildTime, ShaVer) is injected
  via goreleaser ldflags. The AcoustID app key is a compiled-in constant
  chosen per release line (`metainfo.AcoustIDAppKey`) so usage stats can be
  told apart on acoustid.org — a new major release line needs its own key
  registered there and added to that function. An empty key silently
  disables audio identification by design (optional dependency).
- **No backwards compatibility is promised** (pre-release, no users —
  CLAUDE.md). There are no schema migration guarantees between tags; do not
  add migration code to satisfy a release.

See [testing.md](testing.md) for the `make verify` gate and
[architecture.md](architecture.md) for the embed/composition layout.
