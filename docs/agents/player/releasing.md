# Player releasing — bundles, versioning, cautions

The player ships with every Aether release: pushing a `v*.*.*` tag runs
`.github/workflows/release.yml`, whose `player` job builds the Tauri bundles on
GitHub's Linux, Windows and macOS runners and uploads them to the release the
server's goreleaser job created. The server-side view of the same pipeline is
[../releasing.md](../releasing.md#the-desktop-player-bundles). No release has
been published with the player in it yet. Paths are relative to `player/`.

## How a bundle is produced

- Local: `make player-build` (root) or `make -C player build` → `npx tauri
  build`, which runs the webui production build via `beforeBuildCommand`
  (`npm --prefix ../webui run build`, so `webui/node_modules` must be
  installed — `make player-prepare`), then bundles every format the host OS
  supports (`bundle.targets: "all"`) into `target/release/bundle/`.
- CI: the `player` matrix in `release.yml` — `ubuntu-22.04` (deb + AppImage),
  `windows-latest` (NSIS installer), `macos-latest` twice
  (`aarch64-apple-darwin` and `x86_64-apple-darwin`, a dmg each).
  `--bundles` narrows the formats per job and
  `--config '{"version":"<tag without v>"}'` sets the version. The files are
  uploaded with `gh release upload --clobber` (a rerun replaces them) under
  Tauri's own names: `aether-player_<ver>_amd64.deb`,
  `aether-player_<ver>_amd64.AppImage`, `aether-player_<ver>_x64-setup.exe`,
  `aether-player_<ver>_aarch64.dmg`, `aether-player_<ver>_x64.dmg`. They are
  not in goreleaser's `checksums.txt`.
- The pre-merge gate is `.github/workflows/player.yml` (fmt, clippy, tests on
  Ubuntu) — see [testing.md](testing.md). It never bundles.

## Version

The version users see is Tauri's package version: `src-tauri/tauri.conf.json`'s
`version` (`0.1.0`, the dev/snapshot value), overridden in CI from the tag. The
`app_version` IPC command reads that same value (`app.package_info().version`),
so the UI and the installer always agree — don't switch it back to
`CARGO_PKG_VERSION`, which stays at the workspace's `0.1.0`. Keep
`Cargo.toml`'s `[workspace.package] version` and `tauri.conf.json`'s `version`
equal anyway; both crates inherit the former.

A prerelease tag (`v1.2.3-rc1`) becomes `1.2.3-rc1`. That is why the CI
bundle set is what it is:

- **Windows: NSIS only, no MSI.** WiX versions are numeric (`a.b.c.d`); tauri's
  bundler rejects a non-numeric prerelease identifier for the MSI, while NSIS
  carries the full string and derives its numeric `VIProductVersion` itself.
- **Linux: deb + AppImage, no rpm.** An RPM `Version` cannot contain `-`; deb
  and AppImage take the string as is. It also matches the server's Linux
  formats (deb + archive).
- **macOS: dmg per architecture**, no universal binary — two smaller
  downloads, and the server's darwin builds are per-arch too.

## Cautions

- **Never change the bundle identifier `dev.andresbott.aether.player`.** It is
  the OS keyring service name — changing it orphans every user's stored
  password — and a different identifier is a different app to the OS.
- **The macOS bundles are unsigned and not notarized** (no Apple Developer
  certificate in CI): Gatekeeper refuses them on first open until the user
  right-clicks → Open, or runs `xattr -cr` on the app. Signing is a
  `tauri.conf.json` `bundle.macOS.signingIdentity` + repository-secrets
  change when there is a certificate. The Windows installer is unsigned too
  (SmartScreen warning).
- **Build the Linux bundles on the oldest runner GitHub hosts** (`ubuntu-22.04`
  today): the deb and the AppImage link against the runner's glibc and
  webkit2gtk, so that is the oldest system they install on (Debian 12 /
  Ubuntu 22.04). Bump the runner when GitHub retires it, knowing it raises
  that floor.
- `webui/dist/` is gitignored and rebuilt by `beforeBuildCommand`; never
  hand-edit or commit it. A stale `webui/dist` is a stale app: `make build-ui`
  refreshes it.
- `package.json` exists only to pin `@tauri-apps/cli` — CI runs `npm ci` here
  before `npx tauri build`; keep `package-lock.json` current when bumping the
  CLI.
- Linux bundle builds need the apt packages listed in `release.yml` and
  [testing.md](testing.md) (`patchelf` on top, for the AppImage).
