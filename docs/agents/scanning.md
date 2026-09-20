# Scanning — pipeline, identity rules, cleanup invariants

`internal/scanner` turns files on disk into `internal/model` rows. It backs
three registered tasks: the incremental `scan` and the full `scan-full` — two
distinct, parameterless tasks (`app/tasks/scan.go`, registered in
`app/cmd/server.go`), each pinned to a `ScanOptions.IsFull` mode rather than one
task carrying a `full` flag — and the metadata editor's targeted `reindex`
(`ReindexParams{ScanFolder, Paths}`, `app/tasks/reindex.go` — see "Targeted
re-index" below). `scan` and `scan-full` are user-triggerable and schedulable
(they are in `apptasks.AvailableTasks`); `reindex` is deliberately absent from
that catalog — enqueued only by the editor's write handlers, never triggered or
scheduled by hand. All hand their progress/log lines to the `*slog.Logger` tempo
passes into the task function, which lands in the per-execution log
(`internal/taskrunner`'s `filelog.Store`; see architecture.md).

## Pipeline (per scan folder, `scanner.go`)

1. **Preflight + walk** (`scanner.go`, `walk.go`) — `Scan` is **two-phase**:
   `preflight` validates and walks *every* scan folder before *any* folder is
   reconciled, and phase 2 (`scanFolder`) then reconciles each one from the walk
   preflight already produced — the tree is never walked twice. Folders come
   from `scanner.Config.Folders` (a `*scanfolder.Set`, built once in `app/cmd`
   from `config.yaml`'s `ScanFolders` — not `Library` rows; see
   architecture.md) and are processed in **name order** (`Set.All()`), which
   is what decides ownership when a file is reachable from two folders (see
   "Identity & normalization rules" below). Phase 1 collects audio files under
   `Folder.Path`, honoring `Folder.ExcludePatterns` (compiled regexes, via
   `Folder.Excludes`) and `FollowSymlinks`, and applies two guards: it refuses
   a folder whose root fails `Folder.Available()` — does not stat as a
   directory, or is itself a symlink (`filepath.WalkDir` does not descend a
   symlink root, so the scan would otherwise "succeed" silently with zero
   files; symlinks *inside* a root are unaffected, and are followed when
   `FollowSymlinks` is true) — and refuses to *continue* when the walk found
   no audio files while `store.CountTracksInScanFolder(name, root)` is
   non-zero — the marker **or** the path range, so the guard still trips even
   right after a config rename, before the next scan has re-stamped the
   marker. `makeWalkFn`
   swallows every error including the root's, so an unmounted share would
   otherwise scan "successfully" with zero results and let step 5 delete the
   whole scan folder. Both guards fail the scan task for **every** scan
   folder, and because they all run in phase 1 the run stops before the first
   write: nothing is committed and `Cleanup` never runs, so the abort is atomic.
   A user who really did empty a scan folder removes its entry from
   `ScanFolders` in the config and restarts instead — the next scan then
   sweeps its tracks and with them the playlist entries, stars and play
   history attached to them, and the guard's error message says so, because
   that is the very deletion it just refused to perform.
   **The phase split is load-bearing, not tidiness.**
   `planTrackContinuity`'s candidate pool is deliberately *not* scan-folder-scoped
   (a move between two collections has to keep its row), so a single-phase loop let an
   earlier-sorting scan folder harvest
   the rows of an unavailable folder that had not reached its own guard yet:
   every path of an unplugged drive stats ENOENT, and one byte-identical new file
   was enough to re-link such a row, moving its stars, playlist entries and history
   onto a file from another collection before the later guard
   failed the scan too late to undo it.
   Still unguarded, and sharper than "swept silently": an unreadable or absent
   *subtree* inside a root that is present — a per-directory mount that is gone
   leaves an empty mountpoint directory behind. Its files stat ENOENT
   indistinguishably from deleted ones, so those rows are swept **and can be
   re-linked** onto a byte-identical new file elsewhere. Not fixable by requiring
   a vanished row's parent directory to still exist: that would break the primary
   use case, since reorganising a scan folder moves whole directories.
   `planTrackContinuity`'s narrowing to `fs.ErrNotExist` only helps when the
   subtree fails with EACCES rather than merely looking empty.
   Accepted, won't fix, on an explicit assumption — mounts are scan folder *roots*, never
   directories inside a scan folder — which is what keeps this out of reach, since a
   dropped root mount trips the guards above. Full analysis and the candidate fixes:
   [`../architecture/caveats.md`](../architecture/caveats.md#vanished-sub-trees-inside-a-present-library-root).
2. **Change filter** — incremental scans skip files whose size/modtime match
   the DB (`store.FilterChanged`); every walked file in the folder — changed
   or not — gets its `last_seen_at` bumped and its `scan_folder` re-stamped in
   500-row chunks (`store.BulkMarkSeen`; see "Identity & normalization
   rules"). A **full** scan (`ScanOptions.IsFull`) re-reads every file's tags.
3. **Tag read** — a worker pool (`Config.TagReadWorkers`, 0 = NumCPU) reads
   tags via `tags.Reader`.
4. **Reconcile** (`reconcile.go`) — one `store.Transaction` **per track**; a
   failed track is logged and skipped, never aborts the scan.
5. **Cleanup** — after all scan folders: tracks with `last_seen_at < scanStart`
   are removed (`store.Cleanup`), then `DeleteOrphanedAggregates`
   (`store/scan_helpers.go`) deletes albums/artists/genres with no tracks
   plus their join rows, playlist entries, stars, and play history.

With **zero scan folders configured**, `Scan` logs and returns before phase 1
ever runs, so an empty (or not-yet-written) `ScanFolders` list cannot wipe an
existing index by starving `Cleanup`'s liveness check.

**Changing the config.** Removing a folder's entry from `ScanFolders` does
not delete anything by itself — its tracks are swept at the *next* scan
(with the playlist entries, stars and play history attached to them), and a
startup `WARN` (`warnScanFolders`) keeps giving the operator notice on every
restart until that next scan runs, not just once. Changing a folder's `Path`
keeps its rows (ids, stars, playlists, history) **only if the old location is
gone by the time the next scan runs** — i.e. the directory was moved or
renamed: the old paths vanish from the walk, the new ones appear as unknown
paths, and `planTrackContinuity` re-links the rows across the two because it
can prove the move (see "Identity & normalization rules" below). If the old
copy still exists at scan time instead — rsync to a new disk, repoint `Path`,
verify, delete the old copy later — nothing is re-linked: every file at the
new path gets a new row, and the old rows are swept with everything attached,
exactly like a removed folder. Move instead of copying, or delete/rename the
old copy before the first scan with the new `Path`. Renaming a folder (its
`Name`) heals on the next scan of any kind through `store.BulkMarkSeen`,
which re-stamps every walked file's `scan_folder` unconditionally.

**Progress.** `scan`/`scan-full` report progress to the UI via
`ScanOptions.Progress` (a `scanner.ProgressReporter`; tempo's reporter satisfies
it, wired in `RegisterWithProgress`). The total is `2 × files-to-process`,
counted once per file in the tag-read pass and once per track in reconcile, so
the percentage crosses both phases; the stage string names the current file
relative to the scan folder root. `reindex` passes a no-op reporter (progress is
scans-only for now).

`LastSeenAt` is the liveness marker — every code path that touches a track
during a scan must set it to the scan's start time, or cleanup will delete
live tracks. It is **monotonic in both writers**: `reconcileTrack` guards its
assignment, and `store.BulkMarkSeen` carries `last_seen_at < scanTime`
in its WHERE clause. Concurrent runs with different `scanStart` values used
to be normal — before the `reindex` task existed, the metadata editor's
targeted rescan ran off the request path and could overlap a scheduled scan
freely. `scan`/`scan-full` and `reindex` (below) are now the writers, each a
singly-parallel task in the same `library-writes` exclusion group, so tempo
never runs one of them while the other (or another instance of the same one)
is in flight — there is only ever one `scanStart` live at a time between
them. The guard stays in place regardless: it is what would keep a live track
safe if that ever stopped being true, and it costs nothing to leave it. Within
a single scan
every row is either already at `scanStart` or older, so the guards never skip
a bump that was needed.

**One deliberate exception.** `store.RelinkTrack` is a third writer that touches
a track during a scan and does **not** advance `last_seen_at` — it rewrites
`file_path`, `filename` and `scan_folder` only. That is safe
because the row now carries the new path, so `reconcileTrack` finds it moments
later in the same batch and sets the marker there; and if *that* transaction
fails, `Cleanup` deletes the row exactly as it would have without the re-link.
Writing the marker in `RelinkTrack` would instead invent a new way to keep a
row alive that no reconcile ever confirmed. Anything else that starts touching
tracks mid-scan still has to advance it.

## Targeted re-index (`internal/scanner/rescan.go`, `app/tasks/reindex.go`)

`Scanner.RescanPaths(ctx, scanFolder, absPaths)` re-indexes an explicit list
of files: it looks up `scanFolder` by name in `scanner.Config.Folders` — an
unknown name fails the job outright — applies the scan preflight's availability
guard (`Folder.Available`), so a folder whose root is unmounted, not a directory
or itself a symlink is refused instead of being indexed piecemeal, then admits
each path via
`WalkWouldEmit`, the same predicate `Walk` uses (inside that folder's root,
an audio extension, not excluded, resolved through any followed symlinks),
reads its tags serially — always fresh, with no `filterChanged` mtime gate —
and hands the results to the same `reconcile` step 4 uses. Inadmissible paths
are silently skipped and counted in `ScanStats.TracksSkipped`; only real
tag-read failures land in `ScanStats.Errors`.

The metadata editor never calls `RescanPaths` directly. Every write handler
(`app/router/handlers/metadata`: `updateTracks`, `applyPicture`, `removals`,
and the artist-folder `setArtistImage`/`deleteArtistImage`) writes to disk
first, then enqueues a `reindex` task (`app/tasks/reindex.go`,
`ReindexParams{ScanFolder, Paths}`; `NewReindexTaskFn` reuses one `Scanner`,
like `NewScanTaskFn` does, and just calls `RescanPaths`) through the
`metadata.Reindexer` interface — implemented by the router's `reindexEnqueuer`
over `*taskrunner.Runner` — and reports it as `reindex: {execution_id}` in the
response (`enqueueReindex`; omitted when nothing was written, or re-indexing
is disabled because no task runner is configured). The metadata API
addresses the selection by `scan_folder` (the configured folder's name)
directly, and `RescanPaths` stamps the ADDRESSED folder's name, so for a
file reachable from two folders an edit made through the non-owning folder
stamps that folder until the next scan re-applies the ownership rule. The
artist-folder handlers re-index only one representative track under the
folder (`metadataedit.FirstAudioPath`) — enough for the per-artist reconcile
pass to re-probe the image, not the whole discography.

**`scan`, `scan-full` and `reindex` share tempo's `library-writes` exclusion
group** (`tasks.LibraryWriteExclusionGroup`, joined via
`taskrunner.ExclusionGroup`, all registered in `app/cmd/server.go`): tempo runs
at most one task from the group at a time, so an edit-triggered re-index can
never run while a scan of either kind is in progress, and vice versa — they no
longer contend for SQLite's single write lock. `scan` and `scan-full` each keep
their own `Singleton()` (duplicate triggers coalesce onto that task's in-flight
run). They are two separate tasks rather than one task with a `full` flag
precisely because tempo coalesces by **task name**, ignoring params: a single
`scan` task would let a full run fold onto an in-flight incremental one and be
silently dropped. `reindex` is not a singleton — every enqueue carries a
different path list, so coalescing two edits' re-indexes would silently drop one
of them.

The SPA polls the enqueued job instead of trusting the write to mean "the
index is current" — a guarantee the old synchronous call gave for free and
this design deliberately gives up. `useReindexPolling.ts`'s `pollReindex`
polls `listTaskExecutions` until every collected id reaches one of tempo's
terminal statuses (`complete`, `failed`, `panicked`, `canceled`,
`cancel_error`, `unknown`; an id **seen running and then** rolled out of the
runner's bounded execution history counts as complete, but an id never seen
stays pending). It reports `{ failed, pending }` — `pending` covering ids left
unconfirmed by the timeout, an aborted poll (it takes an `AbortSignal`,
cancelled on scope teardown), or a persistent poll error — so a write never
claims a clean success it could not confirm. Every write mutation in
`useMetadataEditor.ts` (`useUpdateTracks`, `useApplyPicture`,
`useDeletePicture`) awaits that before invalidating the music-UI caches
(`invalidateAfterMetadataWrite`: the `metadata/tracks`, `metadata/raw` and
`subsonic` query keys); `useEditSession.save`, which can fire several picture
and tag writes in one session save, collects every one's execution id and
polls them all together exactly once at the end instead of per write.

**Only the music UI waits on that poll — the editor itself does not.**
`metadataedit.ListTracks` (the `tracks` handler) reads tags straight from disk
on every call, never from the DB, so the editor's own track list already
reflects a save the instant the file write lands, whether or not its reindex
job has finished. It is the `subsonic`-backed views — everything served
through `/rest`, reading the DB — that would show stale data if refetched
before the job catches up; that asymmetry is the entire reason
`invalidateAfterMetadataWrite` waits on the poll instead of firing right after
the write.

`reconcile` owns `album.CoverPath` outright: it re-detects the best cover file in
the track's directory on every pass, but never lets a disc folder with no art
blank out a cover found in a sibling folder of the same album (albums are keyed
on name + album artist + mbReleaseID, not directory, so a multi-disc release
laid out as `Album/CD 1/` + `Album/CD 2/` collapses to one row). An album's
cover is updated when (a) art appears in THIS directory, (b) the stored path is
unusable (`IsUsableCoverPath` rejects a missing or disqualified file), or (c)
the stored path belongs to this directory and art here has since vanished.
Nothing else writes the field — the metadata editor writes art files on disk and
relies on its post-write reindex job to repoint the album. A cover uploaded through
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
  paths reconciled that is the whole catalog.
- **It calls `store.PruneOrphanedAggregates`, not `DeleteOrphanedAggregates`.**
  Only on the aggregates the touched tracks belonged to before reconcile
  (`store.TouchedAggregatesForPaths`, snapshotted first), not the exhaustive
  whole-catalog sweep the scheduled scan's `Cleanup` runs. An edit can empty
  an album/artist/genre (renaming the last track by an artist), and pruning
  by "has no tracks" rather than a timestamp is safe standalone; anything a
  narrower snapshot misses (a moved-and-retagged row, say) is still swept by
  the scheduled scan's exhaustive pass.

**A successful `reindex` job does not mean every touched track reconciled.**
`NewReindexTaskFn` fails the job only when `RescanPaths` itself returns an
error (an unconfigured scan folder name, unparsable exclude patterns, a
canceled context, or a DB error snapshotting/pruning aggregates); a non-empty `ScanStats.Errors`
(tag-read failures) or a `TracksProcessed` shortfall against
`len(paths)-TracksSkipped` is only logged, exactly like the scheduled `scan`
task, and never fails it. The synchronous handler this replaced (`rescanSaved`,
now removed) used to treat that shortfall as a failure and report it in the
response's `rescan.ok`/`rescan.error` fields — visibility the edit path had
that the scheduled scan never did. Moving to the job engine traded that away:
`reconcile`'s per-track transaction failures are now swallowed identically on
both entry points, and the SPA's poll only ever sees the job's terminal
status, never a processed-count shortfall. Surfacing those failures is the
separate "reconcile swallows failures" item in TODO.md, untouched by this
change.

What a re-index failure costs still depends on the write, and **"the next
scan catches up" is only true for audio-file writes.** A tag or
embedded-picture write changes the file's mtime, so `filterChanged` admits it
and the next *incremental* scan re-indexes it. A **folder-cover or
artist-image write touches no audio mtime**: `filterChanged` sees nothing
changed in that directory, so an incremental scan reconciles zero tracks
there and never re-runs `detectCoverInDir` — the stale cover persists
indefinitely under the normal incremental cadence, and only a **full scan**
(`opts.IsFull` bypasses `filterChanged`) — or an unrelated tag edit to a track
in the same folder — repoints it from there. That gap only matters when the
reindex job itself never runs to completion: a *successful* job always
repoints it immediately regardless of mtime, since `RescanPaths` re-reads
unconditionally. The frontend's warning on a failed poll ("the re-index did
not complete; a library scan will fix it") is now one generic message for
every write kind — unlike the old `rescan.error` note, it no longer spells out
that a folder-cover or artist-image write specifically needs a *full* scan,
not just any scan, to self-heal.

## Identity & normalization rules

- **Track identity is `FilePath`** (unique index), but a path is *transferable*.
  `scanner.planTrackContinuity` (`trackcontinuity.go`), the first thing
  `reconcile` does, re-points a row at the path its file moved to instead of
  letting the move become a delete plus an insert — which would hard-delete the
  track's `playlist_tracks`, `play_histories`, `play_queue_entries` and
  `starred_items` rows via `DeleteOrphanedAggregates`. Every pair must satisfy
  three requirements — `duration` within ±1s, the old path **gone from disk**
  (this is what tells a move from a copy), and exactly one vanished row and one
  new file sharing the key (`file_mod_time` breaks a tie and is never required,
  because plenty of copy tools do not preserve it) — and then one of **two
  proofs**, tried in order:
  1. **`file_size` + `title`.** Free, since the walk and the tag read already
     produced both. Catches a plain move.
  2. **`tracks.audio_hash`** — `libs/audiohash`, a metadata-invariant hash of the
     audio payload (`scanner.audioHashOf`, stored by `reconcileTrack`, looked up
     by `store.TracksByAudioHashes`). Catches a move that **also retagged**: a tag
     edit rewrites the file, so proof 1 loses both anchors, while the hash reads
     only the audio and comes out identical. This is the common real-world case,
     because the tools that rename files from tags (Picard, beets) retag and
     re-file in one operation. The hash is computed in the tag-read worker pool
     for the files a pass actually reads, so an incremental scan hashes exactly
     what changed and a steady state hashes nothing; `audiohash` reads at most
     256 KiB of payload per file.
  A file claimed by proof 1 is withdrawn before proof 2 runs. It all runs
  **before** `planAlbumContinuity` so a moved album is not counted as a split.
  Deliberate misses: a retagged move of a format `audiohash` does not cover (it
  handles FLAC, MP3, MP4, WAV, AIFF, Ogg Vorbis and Opus — eight of `walk.go`'s
  sixteen extensions; WMA, APE, WavPack, TTA, DSF, Matroska/WebM and raw AAC
  remain, as does an Ogg file carrying some other mapping, an Ogg file that is
  chained, truncated or carries a trailer — its stream then has no page ending at
  end of file, so the digest has no length component and every such file would
  otherwise share one value — and a file with no locatable audio at all, such as a
  WAV declaring a `data` size of 0) or of a row that has not been hashed yet —
  **one full scan arms it**, since incremental scans only read
  changed files, and that is the accepted answer for now (see the backfill entry
  under [known scanner debt](#known-scanner-debt-todomd-direction-chosen)); two
  files swapping paths; a move straddling two scan runs (accepted, won't fix —
  [`caveats.md`](../architecture/caveats.md#a-move-that-straddles-two-scan-runs));
  and any ambiguous key — a false match would merge two tracks' listening
  history, which is worse than losing one's. Note that byte-identical duplicates
  of the same track share an audio hash as well as a size, so both proofs rely on
  the same one-of-each rule to stay out of trouble.
  Design: [`../superpowers/specs/2026-08-18-track-identity-across-moves-design.md`](../superpowers/specs/2026-08-18-track-identity-across-moves-design.md).
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
  them the manual cover in the asset store (via the re-key hook below), stars,
  the `newest` ordering, the discovery feed's recency term, and client-cached
  `/album/:id`. Everything unprovable falls through to `FindOrCreateAlbum` and
  churns the id as before: partial edits, splits, merges into an existing
  identity, identity swaps, albums spanning two scan folders (`reconcile` runs
  per scan folder), and albums with a track deleted from disk but not yet swept by
  `Cleanup`. Several albums collapsing into one identity in one batch keep the
  row of the album with the most tracks (lowest id as tiebreak). The entire
  pre-pass is deliberately independent of tag-reader ordering — the survivor
  pick and the iteration order over claims are both deterministic.
  Design: [`../superpowers/specs/2026-08-18-album-identity-continuity.md`](../superpowers/specs/2026-08-18-album-identity-continuity.md).
- **The album pre-pass is snapshot → plan → one transaction per album.**
  `readAlbumSnapshot` reads the batch's current album ids, track counts and held
  identities in one read transaction; `planAlbumRetags` is a **pure** function
  over that snapshot (no DB, so every leg of the proof is unit-testable); and
  `applyAlbumRetag` commits **one album per transaction**. The proof is per
  album, so the unit of work is too — one album's DB error degrades that album
  to a new id and leaves the rest of the batch retagged, matching `reconcile`'s
  own per-track loop. Errors are logged and skipped, never returned, so a
  failure never fails the scan. What makes splitting the reads from the writes
  safe is that `applyAlbumRetag` **re-proves the plan inside the writing
  transaction**: track count unchanged, the row still holds the planned old
  identity, the target identity still free. The counts *are* the proof, and
  acting on a stale one would rename an album that has since split — merging two
  albums that must stay apart, the one failure this path exists to avoid. Any
  leg that no longer holds is a decline (not an error) and falls through to
  `FindOrCreateAlbum`. If you add a signal to the proof, add it in **both**
  places or the re-check stops covering it.
- **Stored images follow the row when continuity moves it.** Three re-key hooks
  (`assetstore.Rekey`, `internal/assetstore/assetstore.go:231-265`) carry an
  entity's asset-store images to its new identity-derived key:
  1. **The album planner** (`scanner.rekeyAlbumImages`, `albumcontinuity.go:303-319`)
     re-keys an album's images **after** that album's transaction commits, so a
     rollback cannot leave a directory moved while the row is not. Failure is tolerated —
     the row moved and the image did not, which is recoverable — and logged.
  2. **An artist gaining an MBID** (`store.FindOrCreateArtists`, `internal/store/artist.go:33-54`)
     appends the artist to a `gained` slice; `reconcileTrack` drains it into
     `pendingArtistRekeys` (`reconcile.go:71-73,91,101`), and `rekeyArtistImages`
     (`reconcile.go:300-310`) runs **after** the transaction. An MBID **change**
     (old MBID non-empty and different from new) is deliberately excluded
     (`artist.go:52-54`): the MBID slot is content-addressed by the real-world
     artist, so moving images when tags reassert a different MusicBrainz artist
     would attach the old artist's portrait to the new one — a misattribution,
     worse than the stranding it avoids.
  3. **Radio stream-URL edit** (`PUT /rest/updateInternetRadioStation`,
     `handlers/subsonic/radio.go:230-243`) re-keys unless the user also uploaded
     a cover in the same request (which is a replace, not a move). Both an
     occupied destination (`assetstore.ErrKeyOccupied`) and hard failures are
     logged but never fail the request, since the station update already
     succeeded and the images stay intact. Replaced the old read-and-re-put,
     which silently dropped named entries and the auto variant; a directory
     rename is lossless.
  **An artist or genre rename still leaves its images behind.** The model says a
  renamed artist is a different artist, and no continuity proof exists: an artist
  spans many albums through two associations (`album_artists`, `track_artists`),
  so "every track crediting this artist is in this batch" is essentially never
  true on an incremental scan, and credits are multi-valued so "they agree on
  one new identity" does not have the same shape as the album proof. The
  continuity signal is weaker; needs its own spec.
- Fallbacks when tags are empty: artist → "Unknown Artist"; album artist →
  "Various Artists" if `Compilation`, else the track artists; album →
  "Unknown Album".
- Multi-value tag frames arrive from `tags.Reader` as separate list entries;
  take them as-is (no splitting on `;` etc. in the scanner).
- MusicBrainz IDs from tags (`MBArtistID`, `MBReleaseID`, `MBRecordingID`,
  …) are aligned positionally with artist names (`alignMBIDs`) and stored —
  they drive artist-image fetching and album identity.
- **`tracks.scan_folder` is a name marker, not a foreign key**, and it is not
  derivable from `file_path`: with `FollowSymlinks` the walker records content
  reached through a symlink under its *resolved* path (`walkSymlinkEntry`,
  `followSymlinkEntry`), which can lie outside the root. Three writers keep it
  current — `reconcileTrack` (files a pass reads), `store.BulkMarkSeen` (every
  walked file, every scan, so a renamed folder heals on the next incremental
  scan) and `store.RelinkTrack` (a move across folders). `tracks.suffix` is the
  lowercase extension, written by `reconcileTrack` only.
  **Ownership when a file is reachable from two scan folders** — nested roots
  are rejected at config load (`scanfolder.NewSet`), so the only remaining
  case is two folders reaching one directory through symlinks — **is decided
  by the last folder that walks it, in name order**, identically for a full
  and an incremental scan: `store.BulkMarkSeen` stamps `scan_folder` in a statement
  deliberately not behind the `last_seen_at` liveness guard, so every folder of
  a scan gets to (re)stamp the row rather than only the first one.

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

## Artist images at scan time (`internal/artistimage`)

`reconcile.go` also records `artist.ImagePath` — an image found in the
artist's **own** folder, for the `<collection>/<artist>/<album>` layout at any
depth (intermediate and disc folders such as
`<collection>/<label>/<artist>/<album>/CD1` are handled). Detection lives in the
reusable `internal/artistimage` package so callers outside the scanner (the
metadata editor, to create an artist image file) can share it.
`artistimage.Detect(libRoot, startDir, artistName)` walks from `startDir`'s
parent up to (excluding) the scan folder root and accepts a directory only when it is
**both** above the album directory **and** named after the artist
(`unidecode.Normalize` on both sides). That double condition is deliberate: file
location alone does not identify an artist, so a differently laid out collection
yields `""` rather than a wrong portrait. `artistimage.FindDir` returns that
folder even when it holds no image yet — what a caller writing a new artist image
needs. Accepted filenames are exact-match only (`artist` > `artistthumb` >
`folder`, plus the package's image extensions) — an album's own
`cover.jpg`/`front.png` never qualifies, and a `folder.jpg` inside the album
directory stays an album cover.

Unlike `album.CoverPath`, the path is re-validated every pass
(`artistimage.IsUsablePath`) and cleared when the file is gone; it is only kept
across a pass when detection finds nothing but the recorded file still exists
(another scan folder may have supplied it). Images are reconciled **once per artist**
in a single pass after every track is in (`reconcileArtistImages`), not per
track, so a large scan folder lists each artist folder at most once per run.

`ImagePath` is the **last** fallback in `artistCoverMeta`
(`handlers/subsonic/media.go`): asset store by MBID → asset store by DB ID →
`ImagePath` → name-seeded generated avatar.

`GET /api/v0/artists/{id}/image-source` (`handlers/artists`) reports which of
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
user picks by name rather than the artist's stored `MBArtistID`, and shows every
candidate portrait as a selectable grid rather than auto-picking one:

- `GET /api/v0/artists/image-candidates?mbid=…` runs `Chain.List` and returns
  every provider's portraits as `{url, thumbUrl, provider}` JSON — no bytes
  downloaded server-side, since the browser loads each `thumbUrl` straight from
  the provider's own CDN for the grid.
- `PUT /api/v0/artists/{id}/image-from-search` (body `{mbid, url}`) — called by
  the **editor's Save**, not the dialog: a pick is staged in `ArtistView` like a
  file upload (previewed in the cover, marks the editor dirty, discarded by
  Cancel/Remove). The three staged edits (file, clear, searched pick) are
  mutually exclusive; the last one wins, and `saveEdit` routes a pick here
  instead of `updateArtist`. The handler re-runs `Chain.List` for `mbid` itself
  and only accepts `url` if it matches a `FullURL` that call just returned — an
  SSRF guard against downloading an arbitrary client-supplied URL — then
  downloads through the matched provider (`Chain.Download(provider, url)`) and
  stores it as a **manual** upload, so it outranks anything the job later writes
  to the auto slot. It files under the artist's asset key (MBID slot when
  matched, else DB ID) — the same slot a normal upload uses, because cover
  resolution reads the MBID slot first and a pick filed under the DB ID would
  lose to an auto-fetched image. The *chosen* MBID is only used for the `List`/
  `Download` calls — it is never written to `artist.MBArtistID`: picking a
  portrait is not asserting a metadata match.

## Known scanner debt (TODO.md, direction chosen)

- Full scan should drop-and-reinsert a track's derived rows so renamed
  artists/genres don't linger (currently updates in place). **Scope it to
  associations and track-level rows only** — dropping and re-inserting *album*
  rows would re-introduce the id churn `planAlbumContinuity` exists to prevent,
  taking stars, manual covers and `created_at` with it.
- **Genres** still churn their ids on a rename: `genres.name` (not even normalised)
  is the identity and the cover keys on the DB id with no fallback. In scope, along
  with the wider fix of not keying any cover on a **positional** id — a
  drop-and-rescan reassigns autoincrement ids in insertion order, so a hand-uploaded
  genre or unmatched-artist cover silently returns attached to a different entity.
  **Artists** have the same root cause and are *accepted, won't fix* — the album
  planner does not generalise (an artist spans many albums through two multi-valued
  associations, so the batch-completeness proof is never true on an incremental scan)
  and a real fix needs MBID-based rename detection with its own spec. What a rename
  loses, and why unmatched artists' covers are the ones that break:
  [`caveats.md`](../architecture/caveats.md#artist-id-churn-on-rename).
- An unhashed `tracks.audio_hash` is armed only by a **full scan**, and that is
  the accepted answer until the app reaches a stable release. An incremental scan
  reads only the files it thinks changed, so a row whose file has not been touched
  since the column existed keeps `""` and cannot prove a retagged move. The cheap
  fix is an **opportunistic backfill**: alongside the changed-file list, collect
  the files that are still at the path the DB records but whose row has no hash,
  hash those on the same worker pool and write that one column — no tag read, no
  re-derived aggregates, nothing else touched, and filtered to the extensions
  `audiohash` covers so an unsupported format is not re-opened on every scan
  (without that filter a one-time repair becomes a permanent per-scan tax). It is
  self-terminating: each file it arms it never sees again, so a steady state does
  no work. **Deliberately deferred**, because its whole value is arming an install
  whose operator does not know it needed arming, and with nothing shipped there is
  no install to protect — "run one full scan" is free advice today. Note it can
  never recover a *past* move either way: a file that moved before it was hashed
  has no old path left to read.
- `FindOrCreateArtists`/`FindOrCreateAlbum` should use
  `errors.Is(err, gorm.ErrRecordNotFound)` to distinguish not-found from
  real DB errors.
- `store.GetArtist` combines `Preload("Artists")` with a manual join on the
  same m2m and can return empty `Artists` (GORM gotcha; worked around in
  `ArtistView.vue`).

See [architecture.md](architecture.md) for how scans are scheduled and
[testing.md](testing.md) for scanner test fixtures (`internal/*/testdata`).
