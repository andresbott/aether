# Scanning — pipeline, identity rules, cleanup invariants

`internal/scanner` turns files on disk into `internal/model` rows. It runs as
the `scan` / `scan-full` tasks (registered in `app/cmd/server.go`, defined in
`app/tasks/scan.go`). Task progress/log lines go through `tempo`'s
context logger (`tempo.Info(ctx, ...)`) so they land in the per-execution log.

## Pipeline (per library, `scanner.go`)

1. **Walk** (`walk.go`) — collect audio files under `Library.Path`, honoring
   JSON-encoded `Library.ExcludePatterns` (`excludes.go`) and
   `FollowSymlinks`.
2. **Change filter** — incremental scans skip files whose size/modtime match
   the DB (`store.FilterChanged`); unchanged files only get their
   `last_seen_at` bumped in 500-row chunks (`store.BulkUpdateLastSeen`).
   A **full** scan (`ScanOptions.IsFull`) re-reads every file's tags.
3. **Tag read** — a worker pool (`Config.TagReadWorkers`, 0 = NumCPU) reads
   tags via `tags.Reader`.
4. **Reconcile** (`reconcile.go`) — one `store.Transaction` **per track**; a
   failed track is logged and skipped, never aborts the scan.
5. **Cleanup** — after all libraries: tracks with `last_seen_at < scanStart`
   are removed (`store.Cleanup`), then `DeleteOrphanedAggregates`
   (`store/scan_helpers.go`) deletes albums/artists/genres with no tracks
   plus their join rows, playlist entries, stars, and play history.

`LastSeenAt` is the liveness marker — every code path that touches a track
during a scan must set it to the scan's start time, or cleanup will delete
live tracks.

## Identity & normalization rules

- **Track identity is `FilePath`** (unique index). Moved files are a
  delete + insert.
- **Album identity is the composite unique index** `(name_norm,
  album_artist_norm, mb_release_id)` — created in `model.Migrate`, matched by
  `store.FindOrCreateAlbum`. All three parts flow through
  `unidecode.Normalize` (lowercase ASCII transliteration) where applicable.
  Change album identity semantics in both places or scans will duplicate
  albums.
- Fallbacks when tags are empty: artist → "Unknown Artist"; album artist →
  "Various Artists" if `Compilation`, else the track artists; album →
  "Unknown Album".
- Multi-value tag frames arrive from `tags.Reader` as separate list entries;
  take them as-is (no splitting on `;` etc. in the scanner).
- MusicBrainz IDs from tags (`MBArtistID`, `MBReleaseID`, `MBRecordingID`,
  …) are aligned positionally with artist names (`alignMBIDs`) and stored —
  they drive artist-image fetching and album identity.

## Tag reading (`internal/tags`)

`Reader` interface (`CanRead` + `Read`) with two implementations:
`TaglibReader` (go.senan.xyz/taglib — **replaced in go.mod by the fork
`github.com/andresbott/go-taglib`**, wazero/WASM so no cgo for tags) and
`FFProbeReader` (shells out to ffprobe). Production wiring is
`tags.NewFallbackReader(taglib, ffprobe)` — taglib first, ffprobe for what it
can't read. `ErrUnsupported` marks unreadable file types.

## Cover art at scan time

`reconcile.go` sets `album.CoverPath` from a folder image
(`detectCoverInDir`) **only when it is empty**, and tracks record
`HasEmbeddedCover`. This "only when empty" rule is the root cause of the
known stale-cover bug after retagging — TODO.md carries the full analysis and
the candidate fixes (clear-and-redetect per reconcile pass, or drop
`CoverPath` and resolve per-request). If you touch cover resolution, read
that entry first; don't patch around it locally.

## Artist images at scan time (`artistimage.go`)

`reconcile.go` also records `artist.ImagePath` — an image found in the
artist's **own** folder, for the common `<collection>/<artist>/<album>`
layout. `DetectArtistImage(libRoot, trackPath, artistName)` walks from the
track's parent-of-parent up to (excluding) the library root and accepts a
directory only when it is **both** above the album directory **and** named
after the artist (`unidecode.Normalize` on both sides). That double condition
is deliberate: file location alone does not identify an artist, so a library
laid out differently yields `""` rather than a wrong portrait. Accepted
filenames are exact-match only (`artist` > `artistthumb` > `folder`, plus
`coverExts`) — an album's own `cover.jpg`/`front.png` never qualifies, and a
`folder.jpg` inside the album directory stays an album cover.

Unlike `album.CoverPath`, the path is re-validated every pass
(`IsUsableArtistImagePath`) and cleared when the file is gone; it is only
kept across a pass when detection finds nothing but the recorded file still
exists (another library may have supplied it). Detection results are cached
per (track dir, artist) for the pass so a large library lists each folder
once.

`ImagePath` is the **last** fallback in `artistCoverMeta`
(`handlers/subsonic/media.go`): asset store by MBID → asset store by DB ID →
`ImagePath` → name-seeded generated avatar.

`GET /api/v1/artists/{id}/image-source` (`handlers/artists`) reports which of
those slots won — `"upload"` / `"fetched"` / `"folder"` (+ `path`) / `"none"`,
plus a `filename` for everything but `"none"`. `ArtistView`'s cover editor uses
it for the status line under the file picker (PrimeVue's FileUpload only ever
says "No file chosen"). The upload-vs-fetched split comes from
`assetstore.GetEntry`, which surfaces the manual/auto filename encoding
(`cover.png` vs `cover.auto.png`).

**Remove is for the user's own upload only** — `"upload"` (or a staged pick).
A `"folder"` image is the user's file on disk and a `"fetched"` one is aether's
own doing, so neither offers Remove; the `?` hint says to upload an override
instead. The backend enforces the same rule: `updateArtist`'s `coverClear` calls
`assetstore.DeleteManual`, which drops only the manual variant and leaves an
auto-fetched image to become the served cover again (plain `Delete` removes the
whole entity directory and would throw away fetched art). Genre/playlist/radio
covers have no auto-fetcher, so those handlers still use `Delete`.

This endpoint's precedence **duplicates** `artistCoverMeta`; change both
together or the note will describe an image the user isn't looking at.

## Known scanner debt (TODO.md, direction chosen)

- Full scan should drop-and-reinsert a track's derived rows so renamed
  artists/genres don't linger (currently updates in place).
- `FindOrCreateArtists`/`FindOrCreateAlbum` should use
  `errors.Is(err, gorm.ErrRecordNotFound)` to distinguish not-found from
  real DB errors.
- `store.GetArtist` combines `Preload("Artists")` with a manual join on the
  same m2m and can return empty `Artists` (GORM gotcha; worked around in
  `ArtistView.vue`).

See [architecture.md](architecture.md) for how scans are scheduled and
[testing.md](testing.md) for scanner test fixtures (`internal/*/testdata`).
