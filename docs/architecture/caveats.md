# Architecture Caveats

Behaviours that are **known and accepted, not fixed**. Each entry records the gap,
the assumption that keeps it out of reach in practice, and enough of the analysis
that whoever revisits it does not have to rediscover it.

Write an entry here — rather than burying a note in a code comment — when a gap
(a) can silently produce *wrong* data rather than an error, (b) is held closed by a
deployment assumption rather than by code, or (c) has candidate fixes worth
recording but no chosen direction.

This is not a work list. `TODO.md` carries the scheduled work and links here; every
gap it lists under **Won't implement** should have an entry here, because that
section records a decision and this file records the consequence.

---

## Vanished sub-trees inside a present library root

*This heading (and the `TODO.md` item title quoted just below) keep their
original wording because `TODO.md` links this entry by that exact anchor.
"Library root" there means a **scan folder**'s root — the directory on disk a
`ScanFolders` entry names, not a `libraries` row — and the rest of this entry
says "scan folder" throughout.*

**Status:** accepted — out of reach under the mount assumption below. Marked
*won't implement* in `TODO.md` ("Guarding a vanished or unreadable sub-tree inside a
present library root"), which merged the separate "unreadable subtree" item into
this one: an unreadable subtree and an unattached one fail identically from the
scanner's side, and the EACCES flavour is already declined (see *Not this caveat*).
**Affects:** `internal/scanner` — `planTrackContinuity` (`trackcontinuity.go`) and
step 5 cleanup (`store.Cleanup` → `store.DeleteOrphanedAggregates`).
**Failure mode:** silent misattribution of user data. No error, nothing in the logs.

### The operating assumption that defers this

**A mount is the scan folder root itself, never a directory inside a scan folder.**
`mount /music/collection1` is expected; `/music/collection1/some/mounted/subdir` is not.

That assumption is what makes this caveat theoretical rather than urgent. When the
mount *is* the root, a dropped mount takes the root with it, and phase 1's guards
(below) fail the scan for every scan folder before anything is written — the abort is
atomic and no data moves. Every guard Aether has for this class of problem lives at
the root, so the assumption is precisely the boundary of what is protected.

What would invalidate it, and should send you back here:

- a per-album, per-disc or per-collection mount *inside* a scan folder root
- a bind mount, junction or symlink pointing a scan folder subdirectory at another volume
- an automounted (autofs) subdirectory that mounts on access and unmounts on idle
- a NAS share attached at a subfolder rather than at the scan folder root
- Windows, if it ever becomes a target: "mount in an empty NTFS folder" volume mount
  points and `mklink /J` junctions reproduce the same shape (see *Portability* below)

### The gap

The scanner cannot distinguish **"this file is gone"** from **"this file is
unreachable"**. Both answer the only question it asks — *is the file there?* — with
a bare no. An unmounted mountpoint is the worst version of that, because it produces
no error at all: the directory exists, opens fine, and is honestly empty.

Two guards already cover the root-level version of this (`scanning.md`, pipeline
step 1): a root that does not stat as a directory is refused, and a walk that finds
zero audio files while `store.CountTracksInScanFolder` is non-zero is refused. `makeWalkFn`
swallows every error including the root's, so without them an unmounted share would
scan "successfully" with zero results and let cleanup delete the whole scan folder.

Neither guard sees *inside* a root. A scan folder whose root is healthy while one
subdirectory is hollow passes both, and the tracks under that subdirectory are
treated exactly as deletions.

### What happens, in order

Given `/music` on local disk with `/music/Live Recordings` attached from elsewhere,
and the share offline when a scan runs:

1. `/music` stats fine and the walk finds plenty of audio, so both root guards pass.
2. Every file recorded under `Live Recordings` fails to stat — indistinguishable from
   a deletion.
3. **`planTrackContinuity` runs first**, before any deletion, and asks of each
   vanished row: did this file move? Its proof is equal `file_size`, equal `title`,
   `duration` within ±1s, the old path gone from disk, and exactly one vanished row
   and one new file sharing the fingerprint.
4. If a byte-identical copy of one of those tracks exists elsewhere in the scan folder —
   a duplicate in a compilation folder, say — the proof succeeds. The row is
   re-pointed at the copy, carrying its `starred_items`, `playlist_tracks`,
   `play_histories` and `play_queue_entries` with it.
5. Rows that found no such match are swept by cleanup, and their stars, playlist
   entries and history are hard-deleted with them.
6. The share comes back. The original files are indexed as brand-new tracks with no
   history. The compilation copy is now wearing history that belongs to another file.

### Why misattribution is ranked above loss

Step 5 is data loss and it is *visible*: a playlist gets shorter, a star disappears.
Step 4 produces a scan folder that looks completely healthy while a star sits on the
wrong file and a play count is a blend of two files' listening. There is no error, no
log line, and no way for the user to discover it. A false match also merges two
tracks' listening history, which the track-identity design already calls worse than
losing one track's — the same judgement applies here.

Note step 4 is *created* by move re-linking. Before that feature, these rows were
only ever swept.

### Not this caveat

- **Whole scan folder unavailable** — guarded in phase 1, fails the scan atomically.
- **A subtree that fails with EACCES** rather than looking empty —
  `planTrackContinuity` narrows on `fs.ErrNotExist`, so a permissions failure already
  declines. It is specifically the *readable and empty* case that slips through.
- **A scan folder the user genuinely emptied** — the zero-files guard refuses that
  too, and its error message points at removing the entry from `ScanFolders` in the
  config and restarting, because that is the same cascade — the next scan sweeps the
  folder's tracks — the guard just declined to perform outright.
- **Cloud placeholder files** (OneDrive, Offline Files) — those stat successfully and
  report their true size, so nothing is swept; tag reads fail instead and land in
  `ScanStats.Errors`. Different, milder failure.

### Candidate fixes

None chosen. Recorded smallest-first, with the objection to each.

1. **Volume tripwire** (portable, blunt, favoured). Refuse to sweep or re-link when a
   run would affect an implausible share of one scan folder's tracks. This is the
   existing zero-files guard generalised from "all of them" to "too many of them",
   so it introduces no new concept. Catches unattached mounts, dropped shares,
   half-finished imports and permission accidents with one mechanism, and needs no
   OS-specific code. *Objection:* threshold policy, and it needs an escape hatch for
   a user who really did remove most of a scan folder.
2. **Hollow-directory rule.** Refuse to *re-link* a row whose directory still exists
   but now holds nothing. Narrower than the rejected rule below: a reorganisation
   takes the directory with it, so a legitimate move should not trip it. *Objection:*
   needs validating against real move shapes first — notably a partial move that
   leaves the source directory empty in one run and moves the remainder in the next.
3. **Disk-identity check** (precise, Linux-only). Record each directory's device id
   during a scan; an unattached mountpoint reports its *parent's* device. A directory
   that previously reported its own device, now reports its parent's, and holds zero
   files is almost certainly unmounted rather than emptied. *Objection:* would be the
   first platform-specific file in the tree — `FileInfo.Sys()` carries no volume
   identity on Windows, which needs `GetFileInformationByHandle`. A large cost for a
   case that is rare even on Linux.
4. **Soft delete.** Mark vanished rows absent instead of removing them, and revive
   them when the file reappears. Fixes the *loss* half well and scale-independently,
   where the tripwire only fires above a threshold. **Does not fix the
   misattribution half at all** — re-linking happens before any deletion, so nothing
   is deleted in that path and the mechanism never engages. *Objections, if adopted
   anyway:* the re-link candidate pool must stay restricted to rows that vanished in
   the current run, or a row deleted months ago can be resurrected onto an unrelated
   new file that happens to share the fingerprint; `tracks.file_path`'s unique index
   must become live-rows-only (SQLite partial index — `model.Migrate` already
   hand-writes one raw index, so there is precedent); every raw-SQL and
   `Table("tracks")` read must learn to exclude absent rows, because GORM's soft
   delete does not apply to those (`FilterChanged`, `BulkMarkSeen`,
   `TrackAlbumIDs`, `AlbumTrackCounts`, `GetAlbumList`'s `EXISTS` filter, the
   discovery aggregates, and the fifteen `DELETE`s in `scan_helpers.go`); orphan
   aggregates need a policy (an album whose tracks are all absent must not be
   collected, or album row continuity is undone); and nothing reaps absent rows
   without an age-based prune.
5. **Two-scan quarantine.** Require two consecutive scans to see a file gone before
   allowing a re-link, so a briefly offline share never becomes a candidate.
   *Objection:* it makes the legitimate path harder, not easier — the new file is
   indexed as a fresh track on the first scan, so the second scan must *merge* two
   live rows rather than re-point one vanished row.

**Rejected:** requiring a vanished row's parent directory to still exist. That breaks
the primary use case, since reorganising a scan folder moves whole directories — exactly
when re-linking matters most.

### Portability note

Windows is not a build target today (Debian packaging only, no cross-compile targets,
no platform-specific sources), but the shape matters for choosing a fix. On Windows
the common layout puts the *whole* scan folder on the network (`Z:\Music`, `\\nas\music`),
which is the root — so the existing root guards cover it. Windows also reports a
distinct error for a dead share rather than "not found", which the `fs.ErrNotExist`
narrowing already declines on. The exposed case there is the same nested one: a volume
mounted into an empty NTFS folder, or a junction. Fix 1 is portable; fix 3 is not.

---

## A move that straddles two scan runs

**Status:** accepted — *won't implement* in `TODO.md` ("Recovering a move that
straddles two scan runs"). The fix is a feature with a UI, not a repair.
**Affects:** `internal/scanner` — `planTrackContinuity` (`trackcontinuity.go`) vs.
step 5 cleanup (`store.Cleanup`).
**Failure mode:** silent data loss, bounded to the moved tracks. No error.

### The gap

Move re-linking is a **within-one-run** proof. `planTrackContinuity` matches rows
that vanished in *this* run against files that are new in *this* run; it runs before
cleanup precisely so the old row is still there to re-point. Once cleanup has run,
the row is hard-deleted and there is nothing left to match — the later scan sees only
a brand-new file.

So a move survives if and only if the disappearance and the reappearance are visible
to the same scan. In practice:

- **Safe:** reorganise the library, then scan. One run sees both halves.
- **Loses data:** move the files somewhere outside every library root, scan, move them
  back (or onward), scan again.
- **Loses data:** move tracks between two scan folders if those are reconciled as separate
  runs rather than in one `Scan` call.

What goes with the deleted row: `starred_items`, `playlist_tracks`, `play_histories`
and `play_queue_entries` for that track. The files are byte-identical throughout — the
audio hash would have proved the move perfectly, it simply has no surviving row to
prove it against.

### The assumption that defers it

**A reorganisation is completed before the next scan.** The operator controls the
sequence: batch the moves, then scan once, and pause the scheduled scan task while
rearranging a library by hand. That is also the natural way to do it, which is why
this stays theoretical.

### Why the fix is out of proportion

Tombstones (soft-delete plus revive on reappearance) are the only mechanism that
spans runs, and the cost is not in the scanner. Every read path has to decide whether
an absent row is visible, `DeleteOrphanedAggregates` must not reap an album or artist
that holds only tombstones, and without an age-based prune the DB grows forever — so a
purge flow becomes mandatory, with UI. The full objection list is fix 4 under *Vanished
sub-trees* above, which is the same mechanism; note it also carries the trap that
matters most here — the re-link candidate pool must stay restricted to rows that
vanished in the current run, or a row deleted months ago gets resurrected onto an
unrelated file that happens to share a fingerprint.

**Revisit when:** soft delete is wanted for its own sake — a "recently removed" view,
an undo, a trash can. At that point the read-path and purge costs are already being
paid and cross-run re-linking is nearly free on top. It is not worth paying for alone.

---

## Artist id churn on rename

**Status:** accepted — *won't implement* in `TODO.md` ("Stable artist identity across
a rename"). Genres have the same root cause and **remain scheduled** there, as does
the positional-id cover key that shrinks this entry's blast radius.
**Affects:** `internal/store/artist.go:21`, `internal/model/artist.go:8` (identity),
`scan_helpers.go:75,83` (orphan cleanup), `internal/assetkey/assetkey.go:74-79`
(cover key), `subsonic/media.go:141,153` (imagecache key).
**Failure mode:** visible data loss on a deliberate user action.

### The gap

An artist's identity is `name_norm` alone. Correcting a spelling therefore does not
rename anything — it creates a second artist row, and `DeleteOrphanedAggregates`
collects the first one once no track credits it. What goes with it:

- the **star** (`scan_helpers.go:83`)
- the **imagecache derivative** keyed on the DB id (`subsonic/media.go:141,153`)
- any **`/artist/:id`** link or bookmark

The **manual cover** survives only for artists carrying an MBID: `assetkey.Artist`
prefers `MBArtistID` and falls back to hashing `name_norm`, so a matched artist keeps
its asset across the rename and an unmatched one does not. That inverts badly — the
covers that break belong to exactly the *unmatched* artists, which are the ones most
likely to hold a hand-uploaded image, since no MBID means no auto-fetch.

### Why the album fix does not port

`planAlbumContinuity` (the closed equivalent) proves continuity from *"every track
this album holds is present in this batch"*. That test does not transfer:

- An artist spans many albums and hundreds of tracks through two associations
  (`album_artists`, `track_artists`), so on an incremental scan the batch essentially
  never holds all of them — the guard would decline precisely when it is needed.
- Credits are multi-valued, so *"the tracks agree on one new identity"* does not even
  have the same shape as it does for a single-artist album.

A real fix needs a different signal and its own spec: the MBID (already the durable
asset key) plus explicit rename detection. That is a lot of machinery for a rare
manual correction, which is why it is declined rather than deferred.

### What is being fixed instead

The three fix shapes for id churn all exist in the tree already — preserve the row
(albums, `planAlbumContinuity`), migrate on re-key (radio, `subsonic/radio.go:230-243`
moves the cover when a stream URL edit changes `RadioKey`), and key on content (artist
MBIDs). The scheduled work takes the third for covers: with a content-derived key
instead of a positional DB id, an artist rename still loses the star and the
link, but no longer orphans or misattributes the image.
That also fixes the drop-and-rescan case, where autoincrement ids are reassigned in
insertion order and a hand-uploaded cover silently comes back on a different entity.

**Revisit when:** renaming artists in the metadata editor becomes a routine operation
rather than an occasional typo fix.

---

## Content reached through a symlink that leaves every scan folder

**Status:** known defect, SCHEDULED in `TODO.md` ("Record a logical (as-spelled) path
per track"). Recorded here because it is silent and because its fix was analysed.
**Affects:** `internal/scanner/walk.go` (`walkSymlinkEntry`, `followSymlinkEntry`,
`symWalk` — they record the RESOLVED path), `internal/pathguard` (`Guard.Allows`),
`subsonic/media.go` (`mediaPathAllowed`), `internal/store/scope.go` (`pathClause`).
**Failure mode:** tracks are listed but do not play, and their embedded / on-disk art
is replaced by a generated cover. Nothing at scan time; "song not found" at play time.

### The gap

With `FollowSymlinks: true` the walker follows a symlinked directory or file and
records what it finds under the link's TARGET path. The target may lie outside every
configured root. The track's `scan_folder` marker still names the folder that walked
it — that is why the marker is a name and not derived from `file_path` — so the row is
indexed, counted and listed. But `pathguard` resolves a path and requires it to sit
inside a resolved root before `stream` or `getCoverArt` may read it, and this one does
not. The same recorded-as-resolved rule has **four** more consequences:

- a library **`path` filter** on, or below, a symlink matches nothing (the folder
  picker warns when such a directory is selected);
- a scan folder whose **root itself** is a symlink is refused outright by
  `Folder.Available()` — deliberately loud (scan and re-index refuse it, startup warns,
  `GET /api/v0/scan-folders` reports `available:false` with the reason);
- the scan's second guard loses its path-range backup for such rows: it still sees
  them through the `scan_folder` marker, but right after the folder was RENAMED in
  the config — the one window the range exists for — an empty walk of a folder whose
  content is all reached through symlinks is not refused;
- the **metadata editor** never lists or traverses a symlinked directory
  (`metadataedit.ListFolders` without `IncludeSymlinks`, `metadataedit.SearchFolders`;
  only the library folder picker's `browse` asks for symlinks), so tracks reached
  through a link cannot be edited in the browser at all.

All of the above is `FollowSymlinks: true`. With `false` a symlinked DIRECTORY is
not indexed at all, but a symlinked FILE still is — under the link's OWN path
(`walk.go` runs a plain `filepath.WalkDir`, and `appendAudio` / `audioFileInfo`
take the link's path while stat-ing through it for size and mtime). The guard
refuses that row the same way once the target leaves every root, so the failure
mode is identical; only the recorded path differs.

### The workaround

List the link's target directory as a scan folder of its own. That works unless the
target directory CONTAINS another scan folder's root — roots may not be equal or
nested (`scanfolder.NewSet`) — in which case there is no workaround short of moving
the content. The files are then walked under a path inside a root, the guard allows
them, and — the recorded path being the same — no row is duplicated (with
`FollowSymlinks: true`; with `false` the file was recorded under the LINK's path, so
the second folder indexes it a second time under its real one). For a symlinked
root: point `Path` at the real directory.

### The fix that was chosen

Record the path as spelled next to the resolved one (the "logical path" column the
design deferred). For follow-symlinks folders the media guard becomes lexical
containment of the spelled path, `path` filters match what the admin sees in the
picker, the editor stops mapping spelled to resolved paths, and the second scan guard
gets a range that covers symlink-only folders. Rejected as an interim: a per-folder
scan warning — counting correctly needs the guard's per-file symlink resolution
(about a million syscalls per scan at 100k files) unless the walker flags
symlink-reached results. `RescanPaths` must keep its availability guard until the
logical path replaces it.

---

## Libraries are filters: the edges that come with it

**Status:** accepted — consequences of libraries being dynamic predicates over
`tracks` (`store.ScopeOf`) with no materialized membership. Edge 2 is an open
decision in `TODO.md`.
**Affects:** `internal/store/scope.go`, `internal/store/artist.go`
(`excludeHiddenArtists`), `subsonic` (`libraryScope`), `internal/libraryfilter`,
`handlers/libraries`, webui `LibraryFilterBuilder` / `LibraryDialog`.
**Failure mode:** a library silently shows more, or less, than its name promises —
except edge 8, which fails outright instead.

### The edges

1. **Lists narrow, detail views do not.** `musicFolderId` exists only on list
   endpoints. `getAlbum` and `getArtist` take none, so an album found through a
   "Lossless" library still shows its MP3 tracks, and an artist page shows every album.
   Changing that needs an OpenSubsonic extension, not a server-side guess.
2. **A filtered library's artist list ignores OTHER hide-artists libraries.**
   `excludeHiddenArtists` runs only for a zero scope — the all-libraries index, and
   a library without filters addressed by id — never inside a filtered library. Open
   decision in `TODO.md`.
3. **Saved values are not re-checked against the catalog.** Retagging a genre, or
   moving a scan folder's `Path` under a `path` filter, makes the library match less
   with no warning: only `scan_folder` values produce `warnings[]` and a startup
   warning. The edit dialog marks `genre` / `format` / `release_type` values that the
   catalog no longer offers "(not in the catalog)"; a `path` value has no such marker —
   the live match count is the signal.
4. **`release_type` is matched case-insensitively but offered exactly.** A stored
   `Album` can read "(not in the catalog)" beside an offered `album` and still match.
5. **A library with a dangling `scan_folder` value cannot be saved from the admin UI**
   — not even renamed — until the value is removed or replaced: the dialog always sends
   `filters` and the server validates what it is sent. (An API client that omits
   `filters` on `PUT` keeps them unvalidated, which is what keeps such a library
   renameable at all.) Sending filters only when changed was rejected: it needs
   dirty-tracking whose failure mode is a silently unsaved edit.
6. **Values are picked, not typed** (except `path`), because `genre` and
   `release_type` are matched against what the scanner recorded, verbatim. A library
   for a genre that is not in the catalog yet cannot be built in the UI; the API
   accepts any value.
7. **The `path` control commits on Enter and trims.** PrimeVue's chips input drops
   surrounding spaces, so a directory whose name begins or ends with a space can only
   be chosen with *Browse…*, which passes the path through untouched.
8. **One unreadable `libraries.filters` value fails every library.** The column is
   JSON; a hand-edited value that does not decode makes `ListLibraries` fail, so
   `getMusicFolders` and the admin page fail for ALL libraries, and the API cannot
   delete the row because reading comes first. Only a hand edit can cause it. Repair
   with the server stopped:

   ```sh
   sqlite3 <DataDir>/aether.db "SELECT id, name FROM libraries WHERE NOT json_valid(filters) OR json_type(filters) <> 'array';"
   sqlite3 <DataDir>/aether.db "UPDATE libraries SET filters = '[]' WHERE id = <id>;"
   ```

   The query finds invalid JSON and non-arrays (a JSON `null` too, which decodes fine
   and is harmless to reset); an array whose elements have the wrong shape needs the
   same `UPDATE` by id. A library reset to `[]` is the whole catalog —
   and if it hides its artists it is ignored by the artist index until it has a filter.

**Revisit when:** libraries need to be exact at album or artist granularity (edge 1),
or a per-user library model arrives — both want a materialized membership table,
which the design kept as its escape hatch.
