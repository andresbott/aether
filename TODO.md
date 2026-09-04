<!-- todo:guide — managed by todo; this block is rewritten on save. Docs: https://github.com/andresbott/todo
This file is a todo list managed by "todo", a terminal TODO app:
https://github.com/andresbott/todo

todo watches this file and reloads it automatically when it changes on disk, so
you — human or agent — can edit it directly in any editor. Keep to this format
so todo can parse what you write:

  # Heading           Headings ("#" to "######") are categories; they nest by
                      heading level.
  - [ ] Open task     A "- [ ]" line is an open task; "- [x]" marks it done.
  - [/] In progress   "- [/]" flags a task in progress, "- [>]" defers it.
  - [x] Done task     Tasks must live under a category heading.
    - [ ] Subtask     Indent by two spaces to nest a subtask under a task.
    Description text  An indented, non-checkbox line is the task's description.

Notes for editors:
- Text above the first heading (this block included) is preserved on save.
- todo rewrites the file into the canonical form above on every change, so any
  other free-form markdown placed between items is not kept.
-->

# 1.0

## Features

- [ ] Proper playlist editing
- [ ] multi user playlists
- [ ] detach scaned folders from libraries
  use metaadata queries insetd of folders for sources of songs

## Security & Authentication

## Backend

### Backend — API Surface

- [ ] Extend the OpenAPI response-contract test to the upstream-mocked and still-uncovered endpoints
  `app/router/openapi_response_contract_test.go`'s kin-openapi response-contract test (the `TestContract*` functions) validates real handler responses against `docs/openapi/aether-v1.yaml`'s schemas, but only for endpoints reachable with just an in-memory store — bootstrap, auth/tokens, libraries, users, tasks. Closing the gap REQUIRES mocking the radio-browser and MusicBrainz upstreams (`internal/radiobrowser`, `internal/artistimage.MusicBrainzSearch`) so `searchRadioStations`, `getRadioFavicon`, `searchMusicBrainzArtists`, `searchMusicBrainzReleases`, `getReleaseGroupGenres`, `listArtistImageCandidates` and `setArtistImageFromSearch` can be asserted without hitting the real internet. Still uncovered beyond that: fixtures for identify/identify-album audio-fingerprint identification (needs sample audio plus a fake AcoustID backend), the whole `metadata` group (folders/tracks browsing, pictures inventory/apply/removals, artist-folder/artist-image), binary responses (image bytes from `getPictureImage`/`getArtistImage`/`getRadioFavicon` — schema validation only applies to their JSON error paths), and the update/delete/patch mutation variants (`updateTracks`, `clearPictureSelection`, `deleteArtistImage`, `deleteToken`, `deleteUser`, `deleteLibrary`, `patchTaskSchedule`, `deleteTaskSchedule`, `cancelTaskExecution`) whose response shapes are never exercised today.

### Backend — OpenSubsonic Compliance

- [ ] Review the non-OpenSubsonic API surface
  Audit custom (non-Subsonic) endpoints, then move Libraries and Tasks management under an `/admin` path (e.g. `/api/admin/libraries`, `/api/admin/tasks`) so admin concerns are clearly separated from the Subsonic-compatible surface; update the frontend accordingly. Pre-1.0 because it is a breaking URL reorg — free now under the no-backwards-compat rule, expensive once anything depends on the paths. Not started: everything is still on one `/api/v1` subrouter (`app/router/main.go:201`), no `/admin` prefix anywhere. Note the authorization half already landed — `/api/v1` defaults to admin-only in both modes via the three-tier guards (`api_v1.go:56`, `proxy_auth.go`), so this is now purely about URL shape, not access control.
- [ ] XML response format for third-party clients
  Check compatibility with third-party Subsonic clients (DSub, Ultrasonic, Symfonium, etc.). XML is what several clients default to, so this gates the "third-party clients work" promise. Today `f=xml` is explicitly rejected with an error (`subsonic/subsonic.go:66-67`), so those clients fail at the first request. Note the handlers build `map[string]any` throughout (`albumToMap`, `trackToChild`, …), which does not marshal to spec-shaped XML — this needs a serialization layer, not a flag.

### Backend — Data Integrity & Scanning

- [ ] Document the audio-hash format limit in the user-facing docs
  The eight formats `libs/audiohash` does not cover are now a deliberate non-goal (see "Won't implement" → "Audio-hash coverage for the remaining eight audio formats"), and that decision is **user-visible**: on a library of FLAC/MP3/M4A/WAV/AIFF/Ogg/Opus, an external tagger that retags and re-files in one pass (Picard, beets) keeps every track's playlists, stars, play history and queue position; on a WMA, APE, WavPack, raw AAC, Matroska/WebM, TTA or DSF library the same operation silently loses them. Today that is written down only in agent-facing docs (`docs/agents/scanning.md`, `planTrackContinuity`'s doc comment), which no user reads. It needs a home in `README.md`: which formats survive an external retag-and-move with their library data intact, and that the rest fall back to a size-and-title heuristic that a retag defeats. `README.md` has no limitations section today, so this either adds one or extends "Features" with the honest caveat — decide which when writing it. Worth stating the same way the auth caveat already is: plainly, near the top, not buried.
  Same doc owes the operator **one full scan after any release that widens hash coverage**, and says why: the move proof needs the *old* row to already carry a hash, an incremental scan only re-reads files that changed, and a release that adds a format changes the server, not the files — so newly-supported files stay unarmed until something force-reads them. Widening coverage is therefore inert on an existing library until that scan runs. Applies to the WAV/AIFF/Ogg/Opus release specifically, and to any future one.
- [x] A row that has never been hashed stays unhashed until a full scan
  **Accepted, not fixed:** "run one full scan" is the answer, and the only obligation left is telling the user so — folded into the user-facing-docs item above. The audio-hash move proof needs the **old** row to already carry a hash, and an incremental scan only reads the files it thinks changed, so any row never successfully hashed — indexed before the column existed, in a format only supported by a later release, or declined once for a structural reason — keeps `""` and cannot be re-linked however many incremental scans run. A full scan arms every file `libs/audiohash` covers (the other eight never arm; see "Won't implement"). Revisit only if the manual full scan proves to be a real burden, in which case the fix is an opportunistic backfill — hash rows with an empty `audio_hash` even on an incremental scan, bounded and one time per file — designed in `docs/agents/scanning.md` under known scanner debt. Also noted in `planTrackContinuity`'s doc comment.
- [x] Genre ids churn on rename, orphaning covers
  Same root cause as the closed album item above. The artist half is now won't-implement — see "Stable artist identity across a rename (artist id churn)" — so what stays in scope here is genres plus the positional-id cover key, which is also the piece that shrinks the artist case's blast radius without needing rename detection.
  - [x] Genres: raw-`name` identity, DB-id cover with no fallback
    Genres: identity is `name`, not even normalised (`internal/store/genre.go:19`, unique index `internal/model/genre.go:9`), and the cover keys on the DB id with no fallback (`subsonic/genres.go:48`), so a genre rename always orphans it. Milder otherwise — there is no genre star type in the cleanup list, and `/genre/:name` routes by name, so links survive.
  - [x] Cheap partial win: stop keying covers on a positional id
    Cheap partial win, independent of rename detection: stop keying genre and unmatched-artist covers on a positional id. **Made more urgent by the drop-and-recreate policy, not less:** while there are no consumers, "drop the DB and rescan" is the accepted answer to every schema change, and a rescan reassigns autoincrement ids in whatever order it happens to insert — so a hand-uploaded genre or unmatched-artist cover comes back attached to a *different* entity, silently. The escape hatch is what triggers the bug. A non-positional key fixes the rename case and the rebuild case at once; scope them together. (Supersedes the old "DB rebuilds misattributing images" backlog item, which no longer exists separately.) This is also the only part of the won't-implement artist item that stays in scope: with a content-derived key, an artist rename still loses the star, `LastImageFetchAt` and the `/artist/:id` link, but no longer orphans or misattributes the cover.
  - [x] Three fix shapes already exist in the codebase
    Three fix shapes exist and the codebase already contains one of each: preserve the row (albums — `planAlbumContinuity`); migrate on re-key (radio — `subsonic/radio.go:230-243` computes old and new `RadioKey(streamURL)` and moves the cover so a URL edit does not orphan it); key on content (artist MBIDs). The remaining work is applying them, not inventing them.
- [x] Small code-health follow-ups noted during the album-continuity review
  - [x] `planAlbumContinuity` batch transaction vs per-album proof
    **Fixed.** `planAlbumContinuity` was running a whole batch in one transaction while its proof is per album, so one album's DB error rolled back every other album's retag in that batch. Restructured as snapshot → plan → apply: `readAlbumSnapshot` reads the batch state in one read transaction, `planAlbumRetags` is a pure function over it (no DB, so every leg of the proof — split, disagreeing batch, unchanged identity, merge survivor, ordering — is unit-testable), and `applyAlbumRetag` commits **one album per transaction**, re-proving the plan against the live rows inside the writing transaction. That re-check is what makes the read/write split safe: the counts *are* the proof, and acting on a stale one would rename an album that has since split. A per-album failure is logged and skipped, so it degrades only that album to a new id — matching `reconcile`'s own per-track loop. Also drops the write-transaction-held-across-a-whole-library, so a scan no longer blocks the API's writers for the duration. Documented in `docs/agents/scanning.md`.
- [/] Big review of the whole import task
  Done 2026-09-04: six-agent review (Pike, go-architect-reviewer, go-code-reviewer,
  Tony, Natalia, vue-code-reviewer) of the metadata-editing flow, the song-import/scan
  flow, and the two combined (an edit writes tags/pictures to disk then triggers a
  synchronous RescanPaths that reconciles; scheduled scan/scan-full index whole
  libraries, both freshly-imported and pre-existing). Findings catalogued under
  "26-09-04-big-import-review" below. Task stays open until those are triaged/fixed.

### 26-09-04-big-import-review

#### Backend — scan/import correctness & architecture

- [ ] [HIGH] "Audio file" is defined four independent ways — the editor edits tags on files the index can never hold, and reports success
  `walk.audioExtensions` (16), `tags/taglib.taglibExtensions` (12), `tags/ffprobe.ffprobeExtensions`
  (22) and `FallbackReader.CanRead` (union = 22) each answer "does Aether handle this file?"
  differently, with no single source of truth and no test asserting a subset relationship. Six
  extensions — `.mpc`, `.oga`, `.tak`, `.spx`, `.w64`, `.rf64` — are readable/editable but ABSENT
  from `walk.audioExtensions`, so neither `RescanPaths.admitPath` (`IsAudioFile`) nor any full
  `Scan` will ever index them. `metadataedit.ListTracks` lists via `reader.CanRead` (22), so a user
  sees such a file, edits it, gets a green "N of M saved" with `rescan.ok:true` (a skipped path is
  deliberately not counted as a shortfall — `incompleteRescanMessage`), and the track never appears
  in the library. The success ledger actively hides an impossible index update, and the set grows
  silently as formats are added to one map. Distinct from the known audio-hash-coverage item (that
  is about move re-linking across audiohash's 8 formats; this is basic indexability + a false
  success signal). Direction: one source of truth for the file-type sets, with an enforced invariant
  that editor-listable ⊆ indexable, or the editor must visibly mark non-indexable rows.
- [ ] [HIGH] A single per-file tag-write failure during an album-identity edit splits the album and strands its manual cover, stars and created_at
  `updateTracks` (`app/router/handlers/metadata/metadata.go:534-559`) collects only successfully
  written paths into `written` and hands only that subset to `rescanSaved`. `planAlbumContinuity`
  (`internal/scanner/albumcontinuity.go:174-179`) proves an in-place retag only when the batch
  contains EVERY track the album holds. Scenario: user renames a 12-track album; track 7's write
  fails (SMB lock / read-only / EACCES on the NAS layout). `written` has 11 → continuity declines
  as a "split" → the 11 retagged tracks get a NEW album row via `FindOrCreateAlbum` while track 7
  keeps the old identity. The old `albums.id` (its asset-store manual cover keyed by old identity,
  its stars, `created_at`, `newest`/discovery ranking, cached `/album/:id`) is stranded on a 1-track
  remnant. No self-heal: re-saving edits only the failed file (a subset again) → another split. The
  user sees one failed row but nothing warns the cover/stars moved. Direction: reconcile the album's
  FULL track set after an identity-affecting edit (pass the album's dir paths, not just `written`),
  or refuse continuity unless the whole selection wrote; at minimum warn "album cover/stars may have
  moved" when `written < len(paths)` on an identity field.
- [ ] [MEDIUM] Folder-art / album.CoverPath changes are invisible to incremental scans — a failed editor rescan is NOT fixed by "the next scan"
  `album.CoverPath` is only (re)detected inside `reconcileTrack` (`detectCoverInDir`). `filterChanged`
  (`internal/scanner/scanner.go:260-278`) keys purely on AUDIO-file size/modtime, and a folder-cover
  write (`WriteFolderPicture`, `internal/metadataedit/pictures.go:107-137`) or artist.jpg touches no
  audio mtime — so an incremental scan reconciles zero tracks in that dir and never re-runs cover
  detection. The editor's folder/cover/artist-image writes rely ENTIRELY on their synchronous
  post-write `RescanPaths` to repoint the cover; docs claim a rescan failure just means "the index
  lags until the next scan," which is FALSE for folder art — only a full scan (or an unrelated tag
  change to a track in that folder) picks it up. If the editor's rescan returns `ok:false` (e.g. lost
  the SQLite write lock to a concurrent scheduled scan), the cover stays wrong indefinitely under the
  normal incremental cadence. Direction: document accurately, and/or have the editor treat a
  folder/cover/artist-image rescan `ok:false` as "a full scan is required," not a soft "lags" notice.
- [ ] [MEDIUM] Editor's synchronous RescanPaths bypasses the task runner entirely — no governance, request-coupled durability, write-lock contention
  Scheduled `scan`/`scan-full` run through the task runner (queue, history, `MaxParallelism`); the
  editor's rescan does not — `applyPicture`/`updateTracks`/`removals`/artist-folder save call
  `h.Rescan.RescanPaths(...)` directly on the request goroutine, on a shared `libScanner`, tied to
  `r.Context()` (`app/router/handlers/metadata/metadata.go:113-125`; three `scanner.New` over one
  `*store.Store` at `app/cmd/server.go:162,192-193`). Correctness under overlap rests only on
  hand-maintained invariants; there is no bound on concurrent rescans, no coordination with an
  in-flight full scan, and the reindex's durability is coupled to the browser request. At scale this
  is a write-lock contention hotspot on single-writer SQLite (`busy_timeout=5000`): a 50-track folder
  picture-apply reconciles serially in the request while a full scan holds the lock; each per-track
  txn can wait 5s then fail, and `reconcile` swallows it (see below), so `rescan.ok:false` is the only
  trace. Navigating away mid-save cancels the context and aborts the reindex partway (files written,
  index partial), self-healing only on the next scan. Direction: consider making edit-triggered
  reindex a first-class task-runner job (enqueued, bounded, observable, request-decoupled), accepting
  the editor would then poll or lose its synchronous "index is current" guarantee.
- [ ] [MEDIUM] reconcile swallows per-track transaction failures and the scheduled scan surfaces them nowhere
  `reconcile` runs one txn per track and on failure logs at Warn + `continue` — it never returns the
  error and never records it in `ScanStats.Errors` (which only collects tag-READ failures, `scanner.go:216-220`).
  The rescan path compensates (`rescanSaved` treats `TracksProcessed < indexable` as `ok:false`), but
  `NewScanTaskFn` (`app/tasks/scan.go:45-50`) only logs `len(stats.Errors)`. So a reconcile txn failure
  — lost write lock (likely under the contention above), constraint violation, association-replace
  error — is counted in neither `Processed` nor `Errors`: a scheduled scan can silently under-index an
  arbitrary number of tracks and report success. The two entry points sharing `reconcile` give the
  operator opposite visibility into the same failure. Direction: have `reconcile` return a per-track
  failure count (or append to `ScanStats.Errors` with a distinct category) so the task log / a future
  metric can distinguish processed / tag-read-failed / reconcile-failed. Swallow-and-continue itself
  is correct; the invisibility on the scan path is the gap.
- [ ] [MEDIUM] admitPath vs Walk is a fragile hand-maintained mirror; symlink semantics are not mirrored at all
  The invariant "admission must be a superset-free mirror of Walk" is enforced only by two shared
  helpers (`matchesExclude`, `IsAudioFile`) plus prose — no test runs the same paths through both and
  asserts agreement. It is already incomplete for symlinks: `Walk` honors `Library.FollowSymlinks`
  (descends symlinked dirs, resolves to canonical paths in `symWalk`/`walkSymlinkEntry`, stats the
  target for `FileSize`) while `admitPath` (`internal/scanner/rescan.go`) ignores `FollowSymlinks` and
  just `os.Stat`s the given path. A path through a symlinked dir is admitted by the rescan when
  `FollowSymlinks` is false (the walk would never reach it), and the two can disagree on the canonical
  form of a followed-symlink file — colliding with `planTrackContinuity`'s path-identity assumptions.
  An edit under a symlinked layout can index a row a scheduled scan then deletes (id churn, dropping
  stars/playlists/history), or index one file under two path spellings. Narrow blast radius today, but
  the invariant is load-bearing and unguarded. Direction: a single exported per-entry admission
  predicate both `Walk` and `admitPath` call, plus a table-driven test asserting `admitPath(p) ⟺
  Walk-would-emit(p)` over a fixture tree with excludes, ancestor pruning and symlinks.
- [ ] [MEDIUM] DeleteOrphanedAggregates runs a whole-database sweep on every single edit — cost scales with library size, not edit size
  Every index-touching save (`updateTracks`/`applyPicture`/`removals`/artist-folder) ends with
  `RescanPaths` → `DeleteOrphanedAggregates` (`internal/store/scan_helpers.go:66-90`), which runs 15
  unindexed `DELETE ... WHERE id NOT IN (SELECT ...)` anti-joins across albums, artists, genres, join
  tables, `playlist_tracks`, `play_histories`, `playlist_plays`, `play_queue_entries` and four
  `starred_items` variants. Correct for the rare "edit emptied an album" case, but O(size of every one
  of those tables) regardless of how many files the edit touched — on the single SQLite writer, in the
  request goroutine, compounding the contention above. Direction: have the targeted rescan compute
  which aggregates its specific edit could have emptied (it already knows the touched tracks and their
  prior album/artist/genre ids via the snapshot machinery) and prune only those, leaving the
  exhaustive sweep to the scheduled scan's `Cleanup`.
  - [x] Wrap Cleanup + DeleteOrphanedAggregates in a single transaction (fixed 7d37c37)
    `Cleanup` (`internal/store/scan_helpers.go:66-97`) calls `DeleteTracksNotSeenSince` then runs 16
    sequential `db.Exec`s with no enclosing transaction. A failure/crash partway (`return
    fmt.Errorf(...)` at line 86) leaves a partially-cleaned DB — tracks deleted but join /
    `starred_items` rows still present, or albums gone with `album_artists` dangling — until the next
    full scan re-sweeps. Not a concurrency data-loss (SQLite serializes writers), but the error path
    commits a subset. Wrap both the track delete and the aggregate sweep in one `store.Transaction`.
- [ ] [MEDIUM] WriteMetadata applies RemoveUnsupported and WriteTags as two non-atomic taglib operations
  `internal/metadataedit/writer.go:161-181`. The comment promises "an invalid patch never
  half-applies," but that only covers validation (`BuildTagMap` before mutating). The write itself is
  two independent taglib ops: `RemoveUnsupported(path, ...)` then `WriteTags(path, ...)`. If the first
  succeeds and the second fails (or the process dies between), the file has hidden frames stripped but
  the structured edit not applied — and the per-row result reports the `WriteTags` error, telling the
  user "this row failed" while the removal already persisted. Direction: document the ordering
  guarantee, ideally fold both into one write pass if the fork's API allows, or run the destructive
  `RemoveUnsupported` last.
- [ ] [LOW] FindOrCreate* unique-violation under overlapping runs skips a track avoidably
  `FindOrCreateArtists`/`FindOrCreateAlbum` (`internal/store/{artist,album}.go`) do read-then-create
  with no `ON CONFLICT`. A targeted `RescanPaths` at `time.Now()` overlapping a scheduled `scan` can
  have both `First`-miss the same brand-new artist/album and both `Create`; the loser hits a
  unique-violation that propagates out of `reconcileTrack` and the per-track txn is logged-and-skipped
  (`reconcile.go:71`). No data loss (the scheduled scan already bumped `last_seen_at` up front; the
  editor rescan never calls `Cleanup`), only transient staleness the next scan fixes — but a real
  avoidable skip. Extend the known "`FindOrCreate*` should use `errors.Is(gorm.ErrRecordNotFound)`"
  item to ALSO retry the read on a unique-violation instead of failing the whole track.
- [ ] [OPPORTUNITY] The metadata Handler is a 13-dependency god struct spanning eight internal packages
  `app/router/handlers/metadata/metadata.go:54-94` wires Store, tags, coverart, artistimage, dlcache,
  imagecache, scanner and metadataedit into one struct serving ~18 routes (structured tag edit, raw
  tags, embedded/folder pictures, artist-folder images, identify, identify-album). Each concern is
  clean and the disk-vs-index boundary is well kept, but they share little beyond "the editor UI calls
  them," and this is where the next editor feature will land. The identify/identify-album surface
  (depends on `Identifier`/`AlbumIdentifier`, nothing the picture/tag endpoints use) is a natural
  separate handler mounted under `/metadata`. Not urgent; decide before the next capability is bolted
  on. Handler-side counterpart to the known flat-`api_v1.go` / `/admin` reorg item.

#### Backend — code health (line-level)

- [x] [MED] Unchecked error from the genre re-query can silently corrupt associations (fixed e55886d)
  `internal/store/genre.go:25` — after conflict-handling create, the code re-queries for the id via
  `s.db.Where("name = ?", name).First(&genre)` without checking the error. On a race / connection loss
  the re-query fails, `genre.ID` stays 0, and that track's genre association is silently wrong. Check
  the error and return it.
- [x] [MED] reconcileTrack returns bare errors with no context — production scan failures are undiagnosable (fixed ac9a85a)
  `internal/scanner/reconcile.go` (≈ lines 93-96, 104-106, 116-119, 126-128, 148-161, 208-210). Many
  `return err` sites pass through failures from `FindOrCreateArtists`/`FindOrCreateGenres`/
  `FindOrCreateAlbum`/`db.Save`/`Association.Replace`/`UpsertTrack` unwrapped, so the log shows
  "reconcile track failed" without which operation failed. Wrap each with `fmt.Errorf("...: %w", err)`.
- [x] [MED] Silent filesystem error in cover detection (fixed 8729d02)
  `internal/scanner/reconcile.go:288-290` — `detectCoverInDir` calls `filepath.Glob` and discards the
  error, returning "" with no log. Permission/I/O errors that break cover detection for an album are
  invisible. Add a debug log before returning.
- [x] [INFO] store.Transaction does not receive a context for cancellation (fixed 53350ef)
  Added `TransactionContext(ctx, fn)` (via `db.WithContext(ctx)`); `Transaction` kept as a
  `context.Background()` wrapper. Threaded `ctx` through the scan path (reconcile per-track loop,
  album continuity, `Cleanup`) and the libraries handlers (`DeleteLibrary`, update). Startup config
  load stays on the plain wrapper. The direct `s.db.Transaction` calls in playlist/playqueue are a
  separate pattern and were left as-is.

#### API contract (OpenAPI) drift

- [x] [MED] applyPicture: spec claims `paths` has NO maxItems cap, but code enforces 50 and returns 422 (fixed f9e1ed3)
  `docs/openapi/aether-v1.yaml` `ApplyPictureForm` (2245-2273) states verbatim that `paths` "carries
  no server-side maxItems cap" and omits `maxItems`, but `app/router/handlers/metadata/pictures.go:408-411`
  caps at `maxSelectionPaths` (50) and answers 422 `ValidationProblem` on `/paths`. Genuine drift (cap
  shipped #42, false prose written later in #43). A client trusting the spec sends a 60-track multi-disc
  selection and gets an undocumented-shape 422. Fix: add `maxItems: 50` to `ApplyPictureForm.paths` and
  delete the "no maxItems cap" sentence (sibling schemas `PictureSelection`/`UpdateTracksRequest` both
  carry it; the 422 response is already listed on the op).
- [x] [MED] 401/403 documented on ~11 metadata ops but silently omitted on 8 equally-gated ones (fixed 4174666)
  The whole `/api/v1` subrouter is admin-gated (`app/router/api_v1.go:157,161`) so every route can
  answer 401/403, but the spec documents them on ~11 ops and omits them on `listPictureInventory`,
  `getPictureImage`, `applyPicture`, `clearPictureSelection`, `batchReadRawTags`,
  `listPictureCandidates`, `listMetadataFolders`, `listMetadataTracks` (GET). A consumer generating
  error handling from the spec won't handle 401/403 on the picture/browse endpoints. Fix: add the
  `Unauthorized`/`Forbidden` response `$ref`s (both components exist) to those 8 — or strip 401/403
  from all metadata ops and note the front-door guard once. The half-and-half state is the defect.
- [ ] [LOW] Empty `paths` returns 400 on updateTracks/identifyTracks/identifyAlbum, but the spec frames empty selection as 422
  The shared `UnprocessableEntity` response lists "an empty selection" as a 422 case and every
  selection schema has `minItems`, which holds for the 3 JSON picture-selection endpoints
  (`decodeSelection` → 422 `errNoSelection`). But `updateTracks` (`metadata.go:420-421,480`),
  `identifyTracks` (`identify.go:86-88`) and `identifyAlbum` (`identify_album.go:58-61`) return a plain
  400 with no `errors[]`. Make these 422 to match, or drop the "empty selection" example from the
  shared 422 description.
- [x] [LOW] updateTracks rejects an all-empty `fields: {}` with 400, undocumented (fixed dc09d2f)
  `metadata.go:427-429` rejects a zero-value `fields` with 400 "fields must set at least one value to
  write", but `UpdateTracksFields` (spec 2392-2441) has no `required`/`minProperties` and nothing
  documents the no-op rejection. A client sending `fields: {}` after a no-op diff expects a 200 no-op
  ledger. Fix: `minProperties: 1` on `UpdateTracksFields` and/or a prose note.
- [x] [INFO] Undocumented 1 MiB JSON selection-body cap (fixed cf4d310)
  `decodeSelection` wraps the body in `http.MaxBytesReader(..., maxSelectionBodyBytes)` (1 MiB,
  `limits.go:27`; `metadata.go:241`) for inventory/removals/raw-tags; over-cap collapses into the
  generic 400. Defensible (no missing response) but the limit is undocumented on `PictureSelection`.
  Optional sentence. NOTE: extending the OpenAPI response-contract test to the mutation endpoints
  (the existing "extend response-contract test" backlog item) would auto-catch the applyPicture,
  empty-selection and empty-fields findings above — worth pairing.

#### Frontend — metadata editor

- [ ] [HIGH] Session picture-save fires one success toast + one full-tree cache invalidation PER op, defeating the aggregate-report design
  `useEditSession.ts:337-340` builds picture mutations with `quietRescanWarning: true` specifically so
  "a 6-cell save doesn't stack 6 toasts," but that flag only suppresses the rescan WARNING.
  `useApplyPicture/useDeletePicture.onSuccess` (`useMetadataEditor.ts:271-308`) still unconditionally
  `toast.add({severity:'success'})` and `invalidateAfterMetadataWrite(qc)` on every op, and
  `savePictures()` drives them in a `mutateAsync` loop. So an N-cell save stacks N green toasts and
  calls `invalidateQueries(['subsonic'])` (the entire music-UI tree) + `['metadata','tracks']` N times
  sequentially mid-save; each `['metadata','tracks']` invalidation refetches the editor's own active
  query and re-triggers the session's `watch(originals)` prune WHILE it is still writing. Direction:
  gate the success toast on `quietRescanWarning` (or a broader `quiet`), and drop/debounce the per-op
  invalidation when quiet so `save()` invalidates once at the end.
- [ ] [HIGH] Selection is never re-synced to refetched track data after a save — the form baseline goes stale
  `MetadataEditorView.vue` (:29,252,263) holds `selection` as `Track` object references captured before
  the write; after `save()` invalidates and `tracksQuery` refetches, nothing updates `selection` — the
  three `selection.value = [...selection.value]` sites re-copy the SAME stale objects. `EditPanel`
  renders off `props.selection` (`diffInitialValues`), so its "original" baseline stays pre-save until
  the user reselects. When the server normalizes on write (genre trimming, the two-PUT artist+MBID
  split in `updateTracksPartitioned`, a partial per-path failure), the panel shows what the user typed,
  not what's on disk; re-editing diffs against stale originals and mis-detects "dirty." Direction: after
  a successful save + refetch, remap `selection` by path onto the fresh `tracksQuery.data` (dropping
  vanished paths), not re-copy old references.
- [x] [CRITICAL] In-flight identify runs are orphaned, not aborted, when a new run starts (fixed cc55d18)
  `useIdentifyRuns.ts:61-62,121-123` — `identify()`/`identifyAlbum()` overwrite
  `identifyAbort`/`albumAbort` with a new `AbortController` without aborting the previous one. The stale
  response is discarded (`if (identifyAbort !== abort) return`) but the previous request — an expensive
  fpcalc + rate-limited AcoustID/MusicBrainz pass — keeps running server-side, wasting the user's quota.
  Re-identify (`MetadataEditorView.vue:269`) is the likely trigger (its button isn't gated on
  `isIdentifying`). Fix: `identifyAbort?.abort()` at the top of each run.
- [x] [CRITICAL] Debounce timer leaked on unmount in MetadataEditorView (fixed 51ae43e)
  `MetadataEditorView.vue:42-49` — the `folderSearchTimer` setTimeout is never cleared; no `onUnmounted`
  hook exists. Navigating away while the 400ms debounce is pending fires the timer after unmount and
  mutates `folderFilter.value`. Fix: `onUnmounted(() => { if (folderSearchTimer) clearTimeout(folderSearchTimer) })`.
- [ ] [CRITICAL] Async races in FolderTree loading — stale results overwrite fresh ones
  `FolderTree.vue` `runSearch()` (:44-69) and `resetTree()` (:166-171) are async, called from watchers
  (:72,186) with no cancellation or versioning. Changing library while a search is in-flight lets the
  old library's results overwrite the new library's folders. Fix: add sequence numbering and discard
  stale results — `PicturesSection.vue:69-78` already has the reference pattern.
- [x] [MED] RawEditPanel edit buffers are never cleared on selection change — stale values shown for a different track (fixed eceb1af)
  `RawEditPanel.vue:86-99` — `editBuffers` (keyed by tag key) is written per keystroke and `displayValue`
  prefers it over recomputed `row.values`, with no watch clearing it when `selectionPaths`/`results`
  change. Since the panel stays mounted while the user clicks a different track in the live TrackList,
  the textareas keep showing the previously-typed text for a matching key while the staged/dirty state
  reflects the new selection. (Save is masked because save flips `rawMode` off, remounting; the
  selection-change case is not.) Fix: `watch([() => props.selection, results], () => editBuffers.value.clear())`.
- [ ] [MED] scrollIntoView used inside the app shell — documented-forbidden pattern, causes mobile regression
  `TrackList.vue:200-203` (arrow-key row nav) and `FolderTree.vue:117` call
  `element.scrollIntoView(...)` directly inside the shell. `docs/agents/frontend.md` forbids this ("scroll
  the intended scroller with `scrollTo`") because it reveals the target in every scrollable ancestor and
  the mobile visual viewport, sliding the whole app under the URL bar (won't reproduce under DevTools
  emulation; the editor still renders on the mobile shell to phone tier). Fix: compute the row/node
  offset and `scrollTo` the known scroller, matching `QueueBody`'s current-row pattern.
- [ ] [MED] EditPanel.vue is a 1168-line god component with a second source of truth for form state
  Its ~250 lines of buffer/diff/stage logic (`values`, `placeholders`, `artistPairs`, `genresList`,
  `reset`, `stageScalar`, `undo*`, …) are a second source of truth alongside the session overlays, kept
  in sync only by `watch(() => props.selection, reset, {deep:true})`. That is why the view resorts to
  `selection.value = [...selection.value]` no-op re-copies to re-fire the watcher after identify/cancel
  stages overlays externally — any future code that stages into the session without this ritual leaves
  the panel stale. Direction: extract `useEditForm(selection, session)` deriving buffers FROM the session
  (killing the re-copy trick), and split the pair/genre editors into child components. Related perf: the
  `fieldDirty(key)` template helper (`EditPanel.vue:138-140`, called per field per render) does
  O(fields×paths) work each render — a computed dirty-Map would cache it.
- [x] [MED-LOW] Over-broad `deep: true` watcher on the selection array (fixed fca0123)
  `EditPanel.vue:375` — `watch(() => props.selection, reset, {immediate:true, deep:true})` deep-traverses
  an array of full `Track` objects on every trigger, but selection is only ever REPLACED by a new array
  reference (never mutated in place), so the deep flag buys nothing and costs a recursive traversal each
  change. Use a shallow watch (or watch `selectionPaths.value.join('\n')`).
- [ ] [MED] Editor forms have real accessibility gaps
  Field rows use bare `<label>Title</label>` next to PrimeVue `InputText`/`InputNumber` with no `for`/`id`
  association (`EditPanel.vue:505-537` and every field row); RawEditPanel textareas are labeled only by an
  adjacent span. Clicking labels doesn't focus and screen readers announce unlabeled inputs. Load-bearing
  help/warning tooltips sit on non-focusable `<i>` elements (`EditPanel.vue:703,804`; `RawEditPanel.vue:284`)
  — the album-grouping warning ("filling Release ID for only some songs splits the album in two") is
  mouse-hover-only, unreachable by keyboard/AT. Fix: associate labels via `for`/`id`; move tooltip content
  onto focusable elements (`tabindex="0"` + `aria-label`).
- [ ] [MED-LOW] TrackList uses a non-virtualized PrimeVue DataTable
  `TrackList.vue:226-255` — the `DataTable` has no `scrollable`/`virtualScroll`; `listTracks(folder)` for a
  large flat folder (hundreds–thousands of files) renders one `<tr>` per track, and the selection logic
  (`rangeBetween`, `dedupe`, `findIndex` per toggle) is O(n) per interaction. The rest of the app routes
  card grids through `VirtualCardGrid` for this reason. Add `scrollable` + `virtualScroll`.
- [ ] [MED-LOW] FolderTree scrollToNode abuses the selectionKeys v-model as a DOM-query hack
  `FolderTree.vue:99-122` — to locate a node it temporarily sets `selectionKeys.value = {[nodeKey]:true}`,
  waits a tick, queries `[aria-selected="true"]`, scrolls, then clears it (the code's own comment calls it
  "imprecise"). This mutates the Tree's bound selection (visible flash, races a user click landing in the
  same tick) purely to find an element. Query by stable `key`/`data-*` instead, and `scrollTo` (see the
  scrollIntoView item).
- [ ] [LOW] A picture/artist-image save failure silently skips the pending tag edits
  `useEditSession.ts:817-828` — `save()` runs `savePictures()` then `saveArtistImages()` first; if either
  returns `ok:false` it reports the rescan warning and RETURNS before the tag batches. The staged field
  overlays are neither written nor reported — the user sees the picture error and the "Unsaved changes"
  pill but nothing states the tag edits weren't attempted. At minimum surface that tag writes were skipped.
- [x] [LOW] Blob preview URLs rely solely on the route-leave guard for cleanup (fixed e87dc68)
  `useEditSession.ts:564-628` — object URLs are revoked on overwrite/discard/save and via `discardAll()`
  from `onBeforeRouteLeave`, but there is no `onScopeDispose`/`onUnmounted(discardAll)`. Any teardown that
  bypasses the router guard (programmatic unmount, error boundary, HMR) leaks the staged previews. Add an
  `onScopeDispose(discardAll)`.
- [ ] [LOW] PictureCell edit controls are hover/focus-only — unreachable by touch
  `PictureCell.vue:183-186` — Change/Remove/Undo live on the back face of a CSS flip revealed by
  `:hover`/`:focus-within`. Keyboard works (buttons stay tabbable), but a touch tablet (stays on the
  desktop shell) has no hover and no tap handler to flip the tile, so an occupied cell's controls can't be
  reached by tapping. Add a tap-to-flip affordance on coarse pointers.
- [ ] [NOTE] Investigated, NOT a bug: TanStack Query queryKey getters
  vue-code-reviewer flagged `useFolders`/`useTracks`/`useRawTags` passing getter functions in the
  `queryKey` array (`useMetadataEditor.ts:40-53,184-189`) as broken reactivity. Natalia verified it is
  fine: vue-query's `cloneDeepUnref` turns `unrefGetters` on inside the queryKey branch, so
  `() => selectedPath.value` is invoked and tracked reactively. The `as any` cast is a typing wart, not a
  correctness bug. Recorded so it isn't re-flagged; only the cast is worth tidying.

### Backend — Resource Leaks

- [ ] Nothing ever evicts from the image cache
  `Cache.Delete(kind, key)` exists (`internal/imagecache/imagecache.go:130`) and still has zero callers of any kind, production or test, so deleting an entity leaves its derivative directory behind forever. No prune task exists in `app/tasks` either. Superseded fingerprints of a still-live entry are already swept on rebuild (`Cache.sweep`), so this is only about entities that go away. Wire `Delete` in alongside the existing `assets.Delete` calls: `subsonic/artists.go:55,59`, `subsonic/genres.go:56`, `subsonic/playlists.go:348,423`, `subsonic/radio.go:223,226,228,275`, plus album deletion in the scanner's orphan cleanup (`store.DeleteOrphanedAggregates`, which has no assetstore counterpart today). One trap remains, reduced but not removed:
  - [ ] Artist cache eviction must handle both key slots
    The artist cache key uses the MBID-preferring derivation (`artistCoverKey` in `subsonic/artists.go`) and deliberately does not track which of the two slots the image came from (manual upload vs. auto-fetched). An artist later gaining an MBID therefore orphans its old derivatives keyed on the name-hash slot — a leak, not a misattribution, since no other artist can inherit them. The exception is documented in `subsonic-api.md`. Artist-specific eviction logic must handle both keys.
  - [ ] Editor thumbnails can't be swept by entity id (1.0-critical)
    Editor thumbnails (`kind: "editor"`) can't be swept this way at all — `pictureThumbKey` keys them by a hash of the file path or the image bytes (`metadata/pictures.go:419`), which is not derivable from an entity id. They need either a different key scheme or an age-based sweep, so a periodic prune task in `app/tasks` may be the better shape for the whole problem than per-deletion hooks. The editor-thumbnail half is the 1.0-critical part — those grow on every normal use of the metadata editor, not just on deletion. The per-entity wiring could slip to a later release if needed.

## Frontend

### Frontend - mobile

### Frontend — Metadata editor

- [ ] Surface field-level validation errors (RFC 9457 `errors[]`) in the editor forms — the backend now returns `422` `ValidationProblem` with `errors[]` (`{pointer, detail}`) and the SPA type carries it, but no view renders it; the UI still shows only the top-level `detail`/`title`.
- [ ] When identifying albums sometimes the track position is wrong — can we improve that?

# Future releases

## Frontend - Metadata editor

- [ ] Explore exposing Lyrics as an editable field
  Already read and stored (`tags.Metadata.Lyrics` → `track.Lyrics`, `scanner/reconcile.go:200`) but the editor can't set it, so a mis-tagged lyric is unfixable in-app. Only the write side is missing: `metadataedit.Track`/`Patch`/`BuildTagMap` (`taglib.Lyrics`), `managedTagKeys`, and the frontend `Track`/`PatchFields`/`MANAGED_TAG_KEYS` + `EditPanel`.
- [ ] Explore exposing Release type as an editable field
  Same shape as lyrics: read and stored (`tags.Metadata.ReleaseType` → `album.ReleaseType`, `scanner/reconcile.go:128`) but not editable, so album/EP/single/compilation classification can't be corrected in the UI. `taglib.ReleaseType` (`RELEASETYPE`).
- [ ] Explore sort keys as editable fields
  MusicBrainz-style sort tags — ArtistSort/AlbumSort/TitleSort/AlbumArtistSort/ComposerSort (`TSOP`/`TSOA`/`TSOT`/`TSO2`/`TSOC`) — control browse ordering, which is aether's whole job. Bigger slice than the two above: not read today, so it needs scanner read + a store column *and* the editor field, not just the write side.
- [ ] Improve track position when identifying albums
  Track position is sometimes wrong when identifying albums — can we improve it?

## Backend — Multi-user

- [ ] Multi-user — per-user scoping of queue, stars, playlists, and history
  Landed via session identity (owner-keyed schemas). PAT layer landed (replaces the interim cookie resolver and unblocks third-party clients). Remaining work: re-key the owner columns on `User.ID` instead of the login string, then re-enable rename. The caveat is now contained, not live: `owner` is still the LOGIN string (`patIdentityResolver` returns `info.LoginID`, `app/router/main.go:133`), so a rename would orphan queue/stars/playlists/history — but renaming is refused with 400 (`errRenameUnsupported`, `handlers/users/users.go`) and `UserDialog.vue` shows the login read-only, so nothing can trigger the orphaning. Lifting the refusal is the last step of this item, not a prerequisite.
- [ ] Favorites schema — superseded by `Owner` on `starred_items`
  Junction table keyed `(owner, item_type, item_id)` with unique index `idx_starred_item`. A per-type split (`album_stars`, `artist_stars`, `track_stars`) with `(user_id, item_id)` PKs and cascade deletes is optional later if there's a concrete benefit; the current schema works. **Decide now, not "later":** with no consumers, a schema change costs nothing but dropping the DB — that is the only window in which this is free, and after 1.0 it needs a real migration for a change that is optional by its own admission. So either commit to the split while it is cheap or close this as won't-do; "later" is the one answer that gets expensive.

## Backend — OpenSubsonic Completeness

- [ ] Transcoding for formats browsers can't play
  Identify formats browsers can't play natively and add FFmpeg transcoding. Not a 1.0 gate: browsers natively handle FLAC/MP3/OGG/Opus/AAC, so this is for exotic formats and bandwidth-limited remote listening, and it's a whole subsystem (ffmpeg dependency, cache, per-client profiles).
- [ ] CUE sheet support
  Single audio file + `.cue` sidecar (DJ mixes, EAC FLAC/APE rips). Exposed as regular per-track albums with seamless web-UI playback; virtual tracks (file + time region), scanner pairing, ffmpeg remux slicing for third-party clients, OpenSubsonic extension for region offsets. Full assessment: `docs/cue-playing.md`.
- [ ] setRating persistence — add rating column to tracks/albums when needed
  Confirmed unimplemented: `setRating` is a two-line `writeResponse(w, nil)` (`subsonic/annotation.go:103-105`) and no model carries a rating field. The handler answers OK for a write it drops — if this stays unimplemented through 1.0, that silent lie is the part worth revisiting.
- [ ] getArtistInfo / getAlbumInfo
  External metadata (MusicBrainz bios, similar artists).
- [ ] getTopSongs / getSimilarSongs
  Requires external data or play-history analysis.
- [ ] Podcasts, Jukebox — not in scope for this pass
  Internet Radio has since been implemented: full CRUD under `/rest/` + Radio UI.
- [ ] Bookmarks — not needed for resume
  `savePlayQueue`'s `position` already covers resume. Only worth adding for per-track offsets that survive a queue replacement (audiobooks, long sets). If added, the play queue stays the single source of truth for the current track's position — do not write both on the same tick.

## Backend — Cleanup

- [ ] Extract a shared helper for the three cover-art extension handlers
  `updateArtist` (`handlers/subsonic/artists.go`), `updateGenre` (`genres.go`) and `updateAlbum` (`albums.go`) differ in five places — the endpoint name in one error string, the `decodeID` kind, the store lookup, the `assetstore.Kind`, and the key derivation (`artistCoverKey`'s MBID-or-DB-id logic vs `strconv.FormatUint`) — while everything around them is character-identical: multipart guard, byte cap, `id` presence, kind check, 404, `readCoverFile`, the put/clear switch, `writeResponse`. Roughly 40 duplicated lines. Shape: a helper taking `(endpointName, idKind string, kind assetstore.Kind, resolve func(uint) (writeKey string, clearKeys []string, err error))`, where the artist's extra DB-id slot clear fits `clearKeys` naturally. `dupl` flagged the albums/genres pair when `albums.go` was added and stopped firing once `requireAdmin` diverged them, so nothing is currently suppressed — this is a real DRY item, not a lint workaround. While there: `maxRadioRequestBytes` / `radioMultipartMemory` / `radioCoverMaxBytes` are now imported by four non-radio handlers and deserve cover-neutral names.

## Backend — Library

- [ ] Add library statistics
  e.g. albums, artists, songs, genres, disk space used.
- [ ] Rework libraries: scan folders + metadata-composed
  Stop filtering filesystem concerns into the app. Keep a list of folders to scan (config- or DB-stored); compose libraries from track metadata rather than mapping each library 1:1 to a filesystem path.

## Frontend — Music Browsing & Features

- [ ] Artists tab — grouped page instead of card grid + drill-down
  Replace the grid-of-artist-cards + drill-down with a single scrollable page grouped by artist. One header per artist (alphabetical), followed by that artist's albums sorted by year; no per-artist navigation step. Partial, still accurate: Library has an Artists tab with grid and virtualized list views + alphabet rail — `components/library/ArtistGrid.vue`/`ArtistListView.vue`, wired at `LibraryView.vue:206-211` — but each card is a `router-link` to the `artist` route (`ArtistCard.vue:33`), so it's still artists-that-navigate, not the grouped artist-header + albums layout.
- [ ] Hover multi-select checkbox in song lists
  Spotify-style: on row hover, show a checkbox next to the duration for multi-select. A checkbox-in-the-index-cell pattern already exists in queue edit mode — `components/layout/QueueRow.vue:68-82` — but the browsing song lists (`components/library/AlbumTrackRow.vue`, `GenreTrackRow.vue`) select by plain/ctrl/shift click with no hover affordance; they only tint the row on `:hover`.
- [ ] Album cover drag and drop in the album view
- [ ] Album cover Remove can't tell if there's anything to remove
  `AlbumView`'s hero Remove clears aether's managed cover via `updateAlbum`'s `coverClear`, but most albums are served from folder art or embedded tags instead — so Remove → Save deletes a non-existent asset entry and the old cover reappears. Currently mitigated only by helper text spelling out the semantics; `HeroHeader` already has a `coverRemovable` prop for suppressing the affordance, and `ArtistView` drives it from an image-source query (`/api/v1/artists/{id}/image-source`, surfaced as `canRemoveImage`). The album equivalent needs `/rest` to report whether the served cover is aether-managed — an OpenSubsonic extension field or small endpoint, not an `/api/v1` route, since album covers are music functionality. Same gap exists for genres.
- [ ] Better genre handling — needs scoping before it can be planned
- [ ] Playlist edit is not a nice experience for now — needs scoping
  Name the specific interactions that are wrong (reorder? multi-remove? add-from-search?) before this can be estimated.
- [>] Add filter to artist / album etc

## Frontend — Player & Controls

- [ ] Implement radio mode queue => keep playing based on same type/taste
  - [ ] If I just listened to an album, put the next album of the same artist
    - [ ] If the artist has no more albums, jump to the next artist with similar tags
- [ ] Jukebox functionality — use the web UI only to control the audio
- [ ] Relay — like jukebox, but loading songs from another instance

## Frontend — Layout

## Metadata & External Integrations

- [ ] Last.fm scrobbling
  Forward each play to the user's Last.fm account so listening history lives off-box, plus the `track.love`/`artist.getInfo`/`album.getInfo` features that ride the same API key. Nothing outbound exists today (confirmed: no `lastfm`/`listenbrainz` reference anywhere in Go) — the local side only: the browser applies Last.fm's own 50%/4min/30s rule (`usePlayer.ts:53`), calls `/rest/scrobble`, and the server appends to `play_history` (`subsonic/annotation.go:62`). No client, credentials, config, or retry queue. Two prerequisites: the `submission` parameter is currently ignored, so now-playing pings are recorded as completed plays; and there is no user/settings table to hold a session key. Consider ListenBrainz first — same off-box history and stats, one unsigned POST with a token, no signed handshake or app registration.
- [ ] DLNA / UPnP endpoint
  Expose the library as a DLNA MediaServer so devices on the LAN (TVs, receivers, stock media players) can browse and stream without the Subsonic client.

# Backlog — needs investigation

- [ ] Support track comments
  COMMENT is a standard, ubiquitous tag (ID3 `COMM`, Vorbis `COMMENT`, MP4 `©cmt`), so exposing it is feasible — but unlike the Lyrics/Release type items it is not a clean promotion, because our tag library flattens comments. `go-taglib` (a binding over TagLib) reads every tag through TagLib's "property map", which represents a comment as plain text under a single `COMMENT` key. That drops the comment's language sub-field and its description/label, and cannot distinguish the several distinctly-labelled comments a file may hold — apps stash private data in labelled comment frames (e.g. iTunes' `iTunNORM`). So a naive single "Comment" text box could show/edit the wrong comment, silently overwrite or erase that hidden app data, or collapse multiple comments into one on save. This is a limitation of TagLib's property-map abstraction (upstream, inherited by the fork), not the file format and not aether: the library offers no per-frame API, only the flat map plus a delete-only "unsupported frames" list, so a faithful comment editor would need a different or extended library. A safe minimal design is to only ever read/write the unlabelled "main" comment and leave labelled frames untouched — but that is a deliberate policy call. Investigated 2026-08-23.
- [ ] Radio stations not saved in the play queue
  Radio stations aren't persisted in the play queue across sessions / devices. Investigated 2026-08-11, not fixed; needs a direction decision before any code.
  - [ ] Root cause: radio enters the queue as a synthetic non-`tr-` `Song`
    Root cause: a station enters the queue as a synthetic `Song` (`webui/src/utils/radioSong.ts:8` — `id: radio-<name>` plus a `streamUrl`, deliberately not the real `rs-<n>`). `useQueueSync.pushQueue` (`webui/src/composables/useQueueSync.ts:50`) sends `queue.map(s => s.id)` verbatim, and the backend's `decodeTrackIDs` (`subsonic/playlists.go:249`) silently drops every non-`tr-` id. `model.PlayQueueEntry.TrackID uint` is track-only anyway, and restore rebuilds entries from `model.Track` via `starredSongList`, which emits no `streamUrl` — so nothing radio-shaped can round-trip even if it were stored.
  - [ ] Two collateral bugs, worse than the missing station
    Two collateral bugs are worse than the missing station, and are worth fixing whichever direction wins: (a) the client's `currentIndex` counts the dropped entry, so a station before the playing track makes `currentIndex >= len(trackIDs)` and `savePlayQueueByIndex` answers error 10 (`subsonic/playqueue.go:73`) — discarding the whole save, tracks included; a station after it nominates the wrong current track. (b) A radio-only queue decodes to zero ids, which routes to `clearSavedQueue` (`playqueue.go:58`) and deletes the other device's saved session. Minor: `usePlayer.ts:74` scrobbles `radio-<name>`, which answers `invalid id` (log noise only).
  - [ ] OpenSubsonic has no mechanism for queuing radio
    OpenSubsonic has no mechanism for this: `savePlayQueue`'s `id`/`current` are defined strictly as song ids; `PlayQueue.entry` is an array of `Child` ("the list of songs in the queue") with a MUST that `current` be "a valid id in the list of songs"; `Child.type` ∈ `{music, podcast, audiobook, video}` and `mediaType` ∈ `{song, album, artist}` — no stream/radio value, no `streamUrl` field. Stations are a separate entity with their own CRUD: the spec's model is that a station is something you play, not something you queue. The extension registry (11 entries) has nothing for it — podcast episodes got a dedicated extension, radio-in-queue never did — so there is no upstream extension to adopt, only one to author.
  - [ ] Options: (A) follow spec, or (B) author a `radioQueue` extension
    Options: (A) follow the spec — radio doesn't persist cross-device, but the client strips non-track entries and recomputes `currentIndex` over the survivors so (a)/(b) become impossible. Pure bugfix, no spec deviation, no schema change. (B) author a `radioQueue` extension — polymorphic `PlayQueueEntry` (kind + ref), `rs-` ids accepted on the `ByIndex` variant only, spec-shaped `getPlayQueue` still filtered to songs. Schema drop, and deviate-first-upstream-later. **The "schema drop" half of B's cost is free right now** — with no consumers, dropping and recreating the DB is the accepted answer to any schema change, so B is not "spec-pure vs. expensive" but simply spec-pure vs. not; weigh it on the deviation alone, and decide before 1.0 makes the schema half real. Either way fix the two collateral bugs above first: they are wrong under both options.
  - [ ] Trap for (B): `stream` ignores the id kind (`rs-3` serves track 3)
    Trap for (B): `stream` discards the id kind (`_, id, err := decodeID(...)`, `subsonic/media.go:29`), so `stream?id=rs-3` today serves track 3's file. A latent bug a radio-in-queue design walks straight into — worth fixing on its own regardless.

# Won't implement

- [>] Recovering a move that straddles two scan runs (tombstones / soft delete)
  Move re-linking is a within-one-run proof: `planTrackContinuity` matches rows that vanished in *this* run against files new in *this* run, and by the time a later run sees the new path, `Cleanup` has hard-deleted the row. Spanning runs needs tombstones, and that cost is not in the scanner — every read path has to decide whether an absent row is visible, orphan cleanup must not reap aggregates holding only tombstones, and a purge flow becomes mandatory or the DB grows forever. A feature with a UI, not a fix, for a sequence the operator controls. **Edge case:** "reorganise, then scan" is safe; "move the files out, scan, move them back, scan again" silently loses that track's stars, playlists, history and queue position even though the files are byte-identical — same for a move between two libraries reconciled as separate runs. Mitigation: batch the moves and scan once, pausing the scheduled scan task while rearranging by hand. Full analysis, the objection list and the revisit trigger: [`docs/architecture/caveats.md#a-move-that-straddles-two-scan-runs`](docs/architecture/caveats.md#a-move-that-straddles-two-scan-runs).
- [>] Guarding a vanished or unreadable sub-tree inside a present library root
  Merges the former "Unreadable subtree swept silently — and can be re-linked" (Data Integrity & Scanning) and "Guard a vanished sub-tree inside a present library root" (Backend — Library): an unreadable subtree and an unattached one fail identically from the scanner's side. `makeWalkFn` swallows per-entry errors, so a hollow mountpoint's files stat ENOENT exactly as deleted ones do — the tracks are swept, and worse, `planTrackContinuity` can re-point such a row onto a byte-identical file elsewhere, handing its stars, playlist entries and history to the wrong file with no error anywhere. Accepted on an explicit assumption: mounts are library **roots** (`mount /music/library1`), never directories inside a library — and a dropped root trips `Scan`'s phase-1 guards, failing the run atomically. **Edge case:** a sub-mount, bind mount, junction or autofs subdirectory *under* a root that is not attached when a scan runs. Mitigation: mount at the library root, never inside it, and do not scan a half-mounted library. Full analysis, five candidate fixes with their objections (favoured: the portable volume tripwire) and the revisit trigger: [`docs/architecture/caveats.md#vanished-sub-trees-inside-a-present-library-root`](docs/architecture/caveats.md#vanished-sub-trees-inside-a-present-library-root).
- [>] Stable artist identity across a rename (artist id churn)
  An artist's identity is `name_norm` alone, so correcting a spelling creates a new row and orphan cleanup deletes the old one. The album fix does not port — `planAlbumContinuity` proves continuity from "every track this album holds is in this batch", which for an artist spanning many albums through two multi-valued associations is essentially never true on an incremental scan, so the guard would decline exactly when it is needed. A real fix needs its own signal and its own spec (MBID plus explicit rename detection): too much machinery for a rare manual correction. **Edge case:** a rename loses the star, the DB-id-keyed image cache, `LastImageFetchAt` (so the artist-image task re-hits the rate-limited providers) and `/artist/:id` links; the manual cover survives only *with* an MBID, so the covers that break belong to the unmatched artists most likely to hold a hand-uploaded image. Genres (same root cause) and the positional-id cover key stay scheduled under "Backend — Data Integrity & Scanning" — the latter shrinks this to the star, `LastImageFetchAt` and the link. Full analysis and the revisit trigger: [`docs/architecture/caveats.md#artist-id-churn-on-rename`](docs/architecture/caveats.md#artist-id-churn-on-rename).
- [>] Audio-hash coverage for the remaining eight audio formats
  `libs/audiohash` fingerprints the audio payload of FLAC, MP3, MP4/M4A/M4B, WAV, AIFF, Ogg Vorbis and Opus — eight of `walk.go`'s sixteen extensions — which is what lets `planTrackContinuity` keep a track's row, and with it its playlists, play history, stars and queue position, across a move that *also* retags the file (the common Picard/beets rename-from-tags case, where `file_size` and `title` both change so the `size+title` proof goes blind). The other eight — **WMA, APE, WavPack, raw AAC, Matroska/WebM (`.mka`, `.webm`), TTA, DSF** — will not be covered for now. Each needs a real per-format container parser (ASF objects, EBML, and four bespoke codec containers) for far worse coverage-per-line than the chunk-list and Ogg-page walkers already written, and they are rare in real libraries; the covered eight are what people actually store music in. The consequence is bounded and fail-safe: an uncovered format falls back to the `size+title` proof, which is exactly what every format had before the hash existed — nothing regressed, those moves just stay unprovable. Same for an Ogg file carrying a mapping other than Vorbis or Opus (Ogg FLAC is declined deliberately: it gives each metadata block its own packet, so a fixed header-packet skip would leave an embedded picture inside the digest and the hash would not survive an art edit), and for a chained, truncated or trailer-bearing Ogg, declined rather than hashed on a missing length component — a missing length component would collapse every such file into one collision class, and a false match merges two tracks' histories, which is worse than losing one's. Revisit only if a library shows up that is materially WMA or Matroska. **The user-visible half of this decision still needs writing down** — tracked under "Backend — Data Integrity & Scanning".
- [>] Sharing and Chat
  Sharing exists to hand out public unauthenticated links (`/share.php?id=…&secret=…` + an HTML landing page) that bypass auth by design; Chat is a global message wall with no rooms or delivery, vestigial in the ecosystem and pointless on a single-user server. Don't add them, and don't file them as gaps again.
- [>] `getUsers` (and Subsonic user CRUD: `createUser`/`updateUser`/`deleteUser`/`changePassword`)
  Admin-only user administration over `/rest`. It only duplicates the users CRUD that already lives on `/api/v1` — the intended admin surface per `CLAUDE.md`'s `/rest`-vs-`/api/v1` split — breaks no playback client (they only ever call `getUser` for their own record), and would add a second privileged write surface plus extra plumbing (the subsonic `Handler` holds only the `AdminChecker` closure, not a user lister). `getUser` (own record) IS implemented; this is the deliberate line where `/rest` stops. Don't file it as a gap again.
