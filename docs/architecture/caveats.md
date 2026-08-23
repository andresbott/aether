# Architecture Caveats

Behaviours that are **known and accepted, not fixed**. Each entry records the gap,
the assumption that keeps it out of reach in practice, and enough of the analysis
that whoever revisits it does not have to rediscover it.

Write an entry here — rather than burying a note in a code comment — when a gap
(a) can silently produce *wrong* data rather than an error, (b) is held closed by a
deployment assumption rather than by code, or (c) has candidate fixes worth
recording but no chosen direction.

This is not a work list. `TODO.md` carries the scheduled work and links here.

---

## Vanished sub-trees inside a present library root

**Status:** accepted — out of reach under the mount assumption below.
**Affects:** `internal/scanner` — `planTrackContinuity` (`trackcontinuity.go`) and
step 5 cleanup (`store.Cleanup` → `store.DeleteOrphanedAggregates`).
**Failure mode:** silent misattribution of user data. No error, nothing in the logs.

### The operating assumption that defers this

**A mount is the library root itself, never a directory inside a library.**
`mount /music/library1` is expected; `/music/library1/some/mounted/subdir` is not.

That assumption is what makes this caveat theoretical rather than urgent. When the
mount *is* the root, a dropped mount takes the root with it, and phase 1's guards
(below) fail the scan for every library before anything is written — the abort is
atomic and no data moves. Every guard Aether has for this class of problem lives at
the root, so the assumption is precisely the boundary of what is protected.

What would invalidate it, and should send you back here:

- a per-album, per-disc or per-collection mount *inside* a library root
- a bind mount, junction or symlink pointing a library subdirectory at another volume
- an automounted (autofs) subdirectory that mounts on access and unmounts on idle
- a NAS share attached at a subfolder rather than at the library root
- Windows, if it ever becomes a target: "mount in an empty NTFS folder" volume mount
  points and `mklink /J` junctions reproduce the same shape (see *Portability* below)

### The gap

The scanner cannot distinguish **"this file is gone"** from **"this file is
unreachable"**. Both answer the only question it asks — *is the file there?* — with
a bare no. An unmounted mountpoint is the worst version of that, because it produces
no error at all: the directory exists, opens fine, and is honestly empty.

Two guards already cover the root-level version of this (`scanning.md`, pipeline
step 1): a root that does not stat as a directory is refused, and a walk that finds
zero audio files while `CountTracksForLibrary` is non-zero is refused. `makeWalkFn`
swallows every error including the root's, so without them an unmounted share would
scan "successfully" with zero results and let cleanup delete the whole library.

Neither guard sees *inside* a root. A library whose root is healthy while one
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
4. If a byte-identical copy of one of those tracks exists elsewhere in the library —
   a duplicate in a compilation folder, say — the proof succeeds. The row is
   re-pointed at the copy, carrying its `starred_items`, `playlist_tracks`,
   `play_histories` and `play_queue_entries` with it.
5. Rows that found no such match are swept by cleanup, and their stars, playlist
   entries and history are hard-deleted with them.
6. The share comes back. The original files are indexed as brand-new tracks with no
   history. The compilation copy is now wearing history that belongs to another file.

### Why misattribution is ranked above loss

Step 5 is data loss and it is *visible*: a playlist gets shorter, a star disappears.
Step 4 produces a library that looks completely healthy while a star sits on the
wrong file and a play count is a blend of two files' listening. There is no error, no
log line, and no way for the user to discover it. A false match also merges two
tracks' listening history, which the track-identity design already calls worse than
losing one track's — the same judgement applies here.

Note step 4 is *created* by move re-linking. Before that feature, these rows were
only ever swept.

### Not this caveat

- **Whole library unavailable** — guarded in phase 1, fails the scan atomically.
- **A subtree that fails with EACCES** rather than looking empty —
  `planTrackContinuity` narrows on `fs.ErrNotExist`, so a permissions failure already
  declines. It is specifically the *readable and empty* case that slips through.
- **A library the user genuinely emptied** — the zero-files guard refuses that too,
  and its error message says to delete the library instead, because that is the
  cascade it just declined to perform.
- **Cloud placeholder files** (OneDrive, Offline Files) — those stat successfully and
  report their true size, so nothing is swept; tag reads fail instead and land in
  `ScanStats.Errors`. Different, milder failure.

### Candidate fixes

None chosen. Recorded smallest-first, with the objection to each.

1. **Volume tripwire** (portable, blunt, favoured). Refuse to sweep or re-link when a
   run would affect an implausible share of one library's tracks. This is the
   existing zero-files guard generalised from "all of them" to "too many of them",
   so it introduces no new concept. Catches unattached mounts, dropped shares,
   half-finished imports and permission accidents with one mechanism, and needs no
   OS-specific code. *Objection:* threshold policy, and it needs an escape hatch for
   a user who really did remove most of a library.
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
   delete does not apply to those (`FilterChanged`, `BulkUpdateLastSeen`,
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
the primary use case, since reorganising a library moves whole directories — exactly
when re-linking matters most.

### Portability note

Windows is not a build target today (Debian packaging only, no cross-compile targets,
no platform-specific sources), but the shape matters for choosing a fix. On Windows
the common layout puts the *whole* library on the network (`Z:\Music`, `\\nas\music`),
which is the root — so the existing root guards cover it. Windows also reports a
distinct error for a dead share rather than "not found", which the `fs.ErrNotExist`
narrowing already declines on. The exposed case there is the same nested one: a volume
mounted into an empty NTFS folder, or a junction. Fix 1 is portable; fix 3 is not.
