# CUE Sheet Support — single audio file + `.cue` as a regular album

**Status:** Not implemented. Architecture assessment recorded for a future implementation.
**Date:** 2026-07-22
**Related code:** `internal/model/track.go`, `internal/scanner/`, `internal/tags/`,
`app/router/handlers/subsonic/media.go`, `webui/src/composables/usePlayer.ts`

## Goal

Support albums stored as **one large audio file plus a sidecar `.cue` sheet**
(DJ mixes, live sets, EAC single-image FLAC/APE rips). Each cue TRACK should
appear and behave like any other song — browsable, searchable, queueable
individually, playlistable, starrable — while playback of consecutive cue
tracks in the web UI is **seamless** (bit-perfect gapless, since they are
literally contiguous bytes of the same file).

### Is this format still relevant?

Niche but persistent:

- **DJ mixes / live sets** — the main ongoing use case; a continuous mix is
  inherently one file and the cue sheet marks boundaries without destroying
  transitions. Usually MP3+cue.
- **EAC-era album rips** — large existing collections of FLAC+cue and APE+cue
  from the single-image ripping convention (2000s–2010s trackers). Not growing,
  but the archives persist and their owners are exactly the self-hosted music
  server audience.
- Long-open feature requests in Navidrome and Jellyfin confirm sustained
  demand; foobar2000, DeaDBeeF, and mpd support it natively.

**Design implication:** although the trigger was "big MP3 + cue", real-world
files are predominantly **FLAC+cue**. The region model must be format-agnostic
from the start (it costs nothing extra; FLAC actually slices cleaner than MP3).

## Core architectural change

The whole system currently assumes **one audio file = one track**. CUE support
introduces **virtual tracks**: a track becomes *a file + a time region*. The
invariant change ripples through four layers; everything else (browsing,
search, playlists, stars, cover art, Subsonic IDs) operates on `Track` rows and
integrates automatically once the rows exist.

### What fights us today

1. **`internal/model/track.go`** — `FilePath string gorm:"uniqueIndex;not null"`.
   Multiple cue tracks must share one file; track identity must become
   `(FilePath, StartMs)`.
2. **`internal/scanner/walk.go`** — the walker only collects files with audio
   extensions (`audioExtensions` map); `.cue` files are invisible. There is
   also no notion of "this audio file is claimed by a cue sheet — don't index
   it as a single track."
3. **`internal/scanner/reconcile.go`** — upsert is keyed by `file_path = ?`;
   change detection (`FilterChanged`, `BulkUpdateLastSeen` in
   `internal/store/scan_helpers.go`) keys on the audio file's path + mtime
   only. A change to the `.cue` alone would go unnoticed.
4. **`app/router/handlers/subsonic/media.go`** — `stream` is
   `http.ServeFile(w, r, filePath)`. No transcoding/segmenting infrastructure
   exists (ffprobe is already shelled out to in `internal/tags/ffprobe.go`, so
   an ffmpeg dependency is not a new class of dependency, but nothing streams
   through it yet).
5. **`webui/src/composables/usePlayer.ts`** — the dual-`<audio>`
   preload/standby-swap design assumes each track is its own URL; no concept of
   "next track is the same file, keep playing."

## Work breakdown

### 1. Data model (small)

- Add `StartMs`/`EndMs` (cue INDEX points are 1/75-second frames, so integer
  seconds are not precise enough; keep the existing `Duration int` seconds for
  display) and `CuePath string` to `Track`.
- Change the unique index from `file_path` to `(file_path, start_ms)`.
- No migration code (project rule: no backwards compatibility).

### 2. CUE parser + scanner integration (medium)

- New `internal/cue` package. The format is simple enough to hand-roll:
  `FILE`, `TRACK`, `INDEX 01`, `TITLE`, `PERFORMER`, `REM DATE`, `REM GENRE`.
- Walk collects `.cue` files alongside audio and pairs each cue with the audio
  file it references (also handle the reverse direction: an audio file next to
  a same-basename cue).
- A paired file emits **N virtual walk/tag results** instead of one. Metadata
  merge: cue wins for title/performer/track number; the file's real tags fill
  gaps (genre, year, ReplayGain, MusicBrainz IDs where sensible).
- Per-track duration = next `INDEX 01` − this `INDEX 01`; the last track runs
  to file end (total duration from taglib/ffprobe).
- Change detection must use `max(mtime(audio), mtime(cue))`.
- Reconcile must handle a cue **appearing or disappearing** next to an
  already-indexed file: single track ↔ N virtual tracks, with orphan cleanup
  of the replaced rows.

### 3. Streaming a time slice — the crux (medium-large)

`/rest/` must stay OpenSubsonic-compliant: third-party clients call
`stream?id=X` and expect a complete, playable audio file. Offsets cannot be
pushed onto arbitrary clients, so the server must produce "a file containing
only track X" at request time.

**Chosen direction: on-the-fly ffmpeg remux, with a segment cache.**

- `ffmpeg -ss <start> -to <end> -i file -c copy -f <fmt> -` — **remux, not
  transcode**: existing frames are copied between the timestamps, audio is not
  re-compressed, CPU cost is basically I/O, latency to first byte is tens of
  ms. This is what Jellyfin-style servers do.
- **Cache-then-serve** rather than pure piped streaming: first request slices
  to a temp file (reuse the atomic `CreateTemp` + `os.Rename` pattern from the
  cover cache, `media.go` `generatedCoverPath`), subsequent requests are plain
  `http.ServeFile`. This restores `Content-Length` and **HTTP range requests**
  — many Subsonic clients seek by byte range, which a pipe cannot satisfy.
  Cost is disk (a fully-sliced 700 MB rip ≈ another 700 MB until evicted), so
  the cache dir needs an LRU size cap. Also support the standard `timeOffset`
  stream parameter.
- **Gapless caveat (third-party clients only):** MP3 frame-boundary cuts can
  leave a tiny artifact at segment edges (bit reservoir spans the cut). FLAC
  has no such issue. The web UI is exempt (see below).
- Rejected alternative: native MP3/FLAC frame slicing in Go (parse Xing/LAME
  headers, compute byte ranges). Avoids the ffmpeg process spawn but is a lot
  of finicky per-format code; ffprobe is already a runtime dependency, so
  requiring ffmpeg is consistent with the deployment story.
- `store.GetTrackFilePath` grows into "file path + region".

### 4. Seamless playback in the web UI (medium — the payoff)

For the built-in player, **don't slice at all**: play the whole physical file
through one `<audio>` element and treat track boundaries as bookkeeping.

- The UI needs the region offsets. Per project API rules this must be an
  **OpenSubsonic extension**: extra child fields (e.g. `sourceOffset` /
  `sourceDuration` on the song child) or a small `getCueInfo`-style endpoint,
  advertised via `getOpenSubsonicExtensions`
  (`app/router/handlers/subsonic/extensions.go`) so other clients ignore it.
- **Single cue track queued alone:** on load, seek to `startMs`; browsers
  fetch via range request from that byte onward and only buffer a limited
  window ahead — the whole 700 MB file is *not* downloaded. Since the `ended`
  event only fires at end of *file*, the player watches `timeupdate` and
  advances the queue when `currentTime` reaches `endMs`. The displayed
  timeline shows `currentTime − startMs` against the track duration, so the
  seek bar looks like a normal song. Seeking within the track =
  `el.currentTime = startMs + t`.
- **Seamless is a detected special case, not a mode:** at the boundary, if the
  *next queue item* is the same file with `startMs === current endMs`, skip
  the standby-swap and let the element keep playing — just swap the displayed
  track metadata. That is bit-perfect gapless, better than any server-side
  splitting. Otherwise behave exactly like today (point at the next track's
  URL / swap to the preloaded standby).
- Queue composition stays unrestricted: single cue tracks, mixed with regular
  songs, any order. Contiguity is an optimization the player detects, not
  something the queue must know about.

### 5. Guard rails (small, easy to forget)

- **Metadata editor**: writing tags back to a shared file would clobber all
  sibling cue tracks. Virtual tracks should be read-only in the editor
  initially (editing the `.cue` itself is a possible later feature).
- Scrobbling / `getSong` / search need nothing — they are row-based.
- `stream`'s `estimateContentLength`-style params and any future download
  endpoints need the same region awareness.

## Suggested build order

1. **Model + cue parser + scanner** — virtual tracks appear in the UI;
   playback still plays the whole file (already usable for testing).
2. **Web-UI extension + `usePlayer` boundary handling** — the primary goal
   (seamless, album-integrated) is fully achieved here.
3. **ffmpeg segment streaming for `/rest/stream`** — makes third-party
   Subsonic clients correct; independent of the UI path, can ship last. Budget
   the most testing time here: process management, client seek behavior, and
   cache eviction are where the sharp edges live.

## Related

- `docs/gapless-playback-web-audio.md` — the deferred Web Audio API approach
  for sample-accurate gapless between *separate* files. CUE contiguous
  playback does not need it (same file, one element), but crossfading or
  gapless across distinct files would.
