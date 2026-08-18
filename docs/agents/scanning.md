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
live tracks. It is **monotonic in both writers**: `reconcileTrack` guards its
assignment, and `store.BulkUpdateLastSeen` carries `last_seen_at < scanTime`
in its WHERE clause. Concurrent runs with different `scanStart` values are
normal — a targeted rescan (below) uses its own, and `scan` / `scan-full` are
separately registered tasks so `MaxParallelism: 1` does not stop them
overlapping. Writing an older timestamp over a newer one would make a live
track look stale to the other run's `Cleanup` and delete it, taking its
playlist memberships, play history and stars with it. Within a single scan
every row is either already at `scanStart` or older, so the guards never skip
a bump that was needed.

## Targeted rescan (`rescan.go`)

`Scanner.RescanPaths(ctx, libraryID, absPaths)` re-indexes an explicit list of
files: it admits each path (inside the library root, audio extension, not
excluded, stat-able, `tagReader.CanRead`), reads its tags serially, and hands
the results to the same `reconcile` step 4 uses. The metadata editor calls it
synchronously after writing tags or pictures
(`app/router/handlers/metadata`), so a save is visible in the music UI
without a scan task. Inadmissible paths are silently skipped and counted in
`ScanStats.TracksSkipped`; only real tag-read failures land in
`ScanStats.Errors`.

`reconcile` owns `album.CoverPath` outright: it re-detects the best cover file in
the track's directory on every pass, but never lets a disc folder with no art
blank out a cover found in a sibling folder of the same album (albums are keyed
on name + album artist + mbReleaseID, not directory, so a multi-disc release
laid out as `Album/CD 1/` + `Album/CD 2/` collapses to one row). An album's
cover is updated when (a) art appears in THIS directory, (b) the stored path is
unusable (`IsUsableCoverPath` rejects a missing or disqualified file), or (c)
the stored path belongs to this directory and art here has since vanished.
Nothing else writes the field — the metadata editor writes art files on disk and
relies on its post-write rescan to repoint the album. A cover uploaded through
the `updateAlbum` extension lives in the asset store, not in `CoverPath`, and
wins at serve time (`subsonic/media.go`, `albumCoverMeta`).

**A run indexed everything it should when `TracksProcessed ==
len(absPaths) - TracksSkipped` and `Errors` is empty.** Never compare
`TracksProcessed` to `len(absPaths)`: the editor's file listing is deliberately
*wider* than the scanner's admission — `metadataedit.ListTracks` ignores
`lib.ExcludePatterns` entirely and `tags.Reader.CanRead` accepts extensions
(`.oga`, `.mpc`, `.tak`, ...) absent from `walk.go`'s `audioExtensions`. A
perfectly correct save therefore routinely hands `RescanPaths` paths it will
not index, and the picture endpoints do so on the *normal* path
(`selectionPaths` → `folderTrackPaths` lists the whole album dir recursively
when the client sends no explicit paths, which is what the editor does for the
folder slot). A full `Scan`'s paths all come from its own walk, so
`TracksSkipped` stays zero there.

Admission must stay a superset-free mirror of `Walk`: `Walk` prunes whole
*directories* with `SkipDir`, so `admitPath` tests every ancestor segment
against the excludes (`excludedByAnySegment`), not just the full relative path
and the filename. Both sides share `matchesExclude` in `walk.go`. Admitting a
path the walk prunes would index a row the next scan immediately deletes.

Two invariants:

- **It must never call `store.Cleanup` / `DeleteTracksNotSeenSince`.** Those
  delete every track whose `last_seen_at` predates the run — with only N
  paths reconciled that is the whole library.
- **It does call `DeleteOrphanedAggregates`.** An edit can empty an
  album/artist/genre (renaming the last track by an artist), and that prune
  is keyed on "has no tracks" rather than a timestamp, so it is safe
  standalone.

A rescan failure is reported in the response's `rescan: {ok, error}` field
and never fails the write: the tags are already on disk, so the only
consequence is that the index lags until the next scan. `ok: true` means every
written path the library *covers* was re-indexed — the handler (`rescanSaved`)
also treats a non-empty `ScanStats.Errors` or a shortfall against
`len(paths) - TracksSkipped` as a failure, because `reconcile` swallows
per-track transaction errors and still returns `nil`. Deliberately skipped
paths are not a failure (see above). The frontend warns on `ok: false` for both
tag and picture writes.

## Identity & normalization rules

- **Track identity is `FilePath`** (unique index). Moved files are a
  delete + insert.
- **Album identity is the composite unique index** `(name_norm,
  album_artist_norm, mb_release_id)` — created in `model.Migrate`, matched by
  `store.FindOrCreateAlbum`, and derived from tags by **one** function,
  `scanner.AlbumIdentityOf` (`internal/scanner/albumidentity.go`), which
  `reconcileTrack` and `planAlbumContinuity` both call. All three parts flow
  through `unidecode.Normalize` (lowercase ASCII transliteration) where
  applicable. Change album identity semantics in **all** of those places or
  scans will duplicate albums.
- **Album *row continuity* is decided by track-set overlap, not by the tuple.**
  `scanner.planAlbumContinuity` (`albumcontinuity.go`), which runs once per
  `reconcile` batch before the per-track loop, retags an album row **in place**
  when every track the album currently holds is in this batch, they all resolve
  to the same new identity, and no other row holds it. That keeps `albums.id`
  and `created_at` across the retags the metadata editor performs — and with
  them the manual cover in the asset store (keyed on the DB id in
  `handlers/subsonic/albums.go`), stars, the `newest` ordering, the discovery
  feed's recency term, and client-cached `/album/:id`. Everything unprovable
  falls through to `FindOrCreateAlbum` and churns the id as before: partial
  edits, splits, merges into an existing identity, identity swaps, albums
  spanning two libraries (`reconcile` runs per library), and albums with a track
  deleted from disk but not yet swept by `Cleanup`. Several albums collapsing
  into one identity in one batch keep the row of the album with the most tracks
  (lowest id as tiebreak). The entire pre-pass is deliberately independent of
  tag-reader ordering — the survivor pick and the iteration order over claims
  are both deterministic.
  Design: [`../superpowers/specs/2026-08-18-album-identity-continuity.md`](../superpowers/specs/2026-08-18-album-identity-continuity.md).
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

`reconcile.go` re-detects `album.CoverPath` per directory on every pass, but
never lets a disc folder with no art blank a cover found in a sibling folder of
the same album: albums are keyed on (name, album_artist_norm, mb_release_id),
not directory, so a multi-disc release spanning several folders collapses to one
row. `detectCoverInDir` picks the best front cover in the reconciled track's
directory; that replaces the stored path when (a) art is found here, (b) the
stored path is unusable (`IsUsableCoverPath` rejects a missing file or one no
longer qualifying as front art), or (c) the stored path belongs to this
directory and art here has since vanished. Tracks record `HasEmbeddedCover`.

Still open in TODO.md: `store.GetCoverTrackPath` picks the *first* track with
`has_embedded_cover=true` with no ordering, so which embedded cover wins is
unstable across rescans, and `getCoverArt` sends `Cache-Control: no-cache`
but no ETag. If you touch cover resolution, read that entry first.

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
says "No file chosen") and to disable Remove for a folder image. The
upload-vs-fetched split comes from `assetstore.GetEntry`, which surfaces the
manual/auto filename encoding (`cover.png` vs `cover.auto.png`).

This endpoint's precedence **duplicates** `artistCoverMeta`; change both
together or the note will describe an image the user isn't looking at.

### Manual online image search

`ArtistView`'s "Search online" button (`ArtistImageSearchDialog`) drives the same
provider chain as the `fetch-artist-images` job, but from a MusicBrainz artist the
user picks by name rather than the artist's stored `MBArtistID`:

- `GET /api/v1/artists/image-preview?mbid=…` runs the chain and streams the image
  back without storing it. Third-party bytes, so the response type is
  `http.DetectContentType`-sniffed (not the provider's claimed extension),
  non-image payloads are refused with 502, and it carries `nosniff` +
  `Cache-Control: no-store`.
- `PUT /api/v1/artists/{id}/image-from-search` — called by the **editor's Save**,
  not the dialog: a pick is staged in `ArtistView` like a file upload (previewed
  in the cover, marks the editor dirty, discarded by Cancel/Remove). The three
  staged edits (file, clear, searched pick) are mutually exclusive; the last one
  wins, and `saveEdit` routes a pick here instead of `updateArtist`. It stores
  the pick as a **manual**
  upload, so it outranks anything the job later writes to the auto slot. It files
  under `artistCoverKey` (MBID slot when matched, else DB ID) — the same slot a
  normal upload uses, because cover resolution reads the MBID slot first and a
  pick filed under the DB ID would lose to an auto-fetched image. The *chosen*
  MBID is not written to `artist.MBArtistID`: picking a portrait is not asserting
  a metadata match.

## Known scanner debt (TODO.md, direction chosen)

- Full scan should drop-and-reinsert a track's derived rows so renamed
  artists/genres don't linger (currently updates in place). **Scope it to
  associations and track-level rows only** — dropping and re-inserting *album*
  rows would re-introduce the id churn `planAlbumContinuity` exists to prevent,
  taking stars, manual covers and `created_at` with it.
- Artists and genres still churn their ids on a rename: `artists.name_norm` and
  `genres.name` are their identities, and artist covers key on **MBID or DB id**,
  so the DB-id slot fails exactly as album covers did. The album planner
  generalises, but the continuity signal is weaker for an artist; needs its own
  spec.
- `FindOrCreateArtists`/`FindOrCreateAlbum` should use
  `errors.Is(err, gorm.ErrRecordNotFound)` to distinguish not-found from
  real DB errors.
- `store.GetArtist` combines `Preload("Artists")` with a manual join on the
  same m2m and can return empty `Artists` (GORM gotcha; worked around in
  `ArtistView.vue`).

See [architecture.md](architecture.md) for how scans are scheduled and
[testing.md](testing.md) for scanner test fixtures (`internal/*/testdata`).
