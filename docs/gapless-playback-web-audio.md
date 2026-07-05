# Gapless Playback — Web Audio API alternative (deferred)

**Status:** Not implemented. Recorded as the alternative to the approach currently shipping.
**Date:** 2026-06-30
**Related code:** `webui/src/composables/usePlayer.ts`, `webui/src/utils/playbackOrder.ts`

## Context

There was an audible gap when the player switched from one song to the next. The
cause: `usePlayer` used a single `HTMLAudioElement` and only started loading the
next track *after* the current one ended (`ended` → set `audio.src` → fetch →
buffer → decode → play). All of that latency landed in the silence between
tracks. The backend isn't involved — `stream` just `http.ServeFile`s the
original file with range support (`app/router/handlers/subsonic/media.go`), so
the gap is entirely browser-side fetch + buffer time.

Two solutions were compared:

- **Approach A — dual `<audio>` preloading (SHIPPED).** Keep two audio elements,
  active + standby. While the current track plays, point the standby at the next
  track's URL with `preload="auto"` so the browser buffers it ahead. On `ended`,
  swap roles instead of re-pointing one element's `src`. Removes the
  fetch/buffer latency; the gap becomes imperceptible for normal tracks.
- **Approach B — Web Audio API true gapless (THIS DOC).** Decode tracks into PCM
  buffers and schedule them back-to-back on an `AudioContext` clock for
  sample-accurate, truly seamless playback.

Approach A was chosen because it is a contained change to one composable, keeps
streaming/seeking/format handling for free, and is low risk. This document
records Approach B in case true gapless (or crossfading) later becomes a hard
requirement.

## What Approach B buys you that A does not

Approach A's swap is driven by the JS `ended` event and a fresh `play()` call, so
it is **not sample-accurate**. For tracks with a little natural silence at the
edges, the gap is inaudible. But for material mastered as one continuous
performance — live albums, DJ mixes, classical movements, *Dark Side of the
Moon* — a tiny seam or click can remain at the boundary.

Approach B is the only way to get:

1. **Sample-accurate gapless** — zero gap, even on continuous albums.
2. **Real crossfading** — high-quality, trivially, via `GainNode` ramps.

## How it works

Fetch each track, decode it to an in-memory PCM `AudioBuffer`, and schedule it on
the `AudioContext` timeline. Because you can tell the next source node to start
at the exact sample time the current buffer ends, the boundary is seamless.

```ts
const ctx = new AudioContext()

async function loadBuffer(url: string): Promise<AudioBuffer> {
    const res = await fetch(url)
    const encoded = await res.arrayBuffer()
    return ctx.decodeAudioData(encoded) // needs the WHOLE file
}

function schedule(buffer: AudioBuffer, when: number): AudioBufferSourceNode {
    const node = ctx.createBufferSource()
    node.buffer = buffer
    const gain = ctx.createGain()
    node.connect(gain).connect(ctx.destination)
    node.start(when) // sample-accurate start on the context clock
    return node
}

// Schedule the next track to begin exactly when the current one ends:
const nextStart = currentStartTime + currentBuffer.duration
schedule(nextBuffer, nextStart)
```

## What you have to rebuild

The `<audio>` element gives a lot for free. With Web Audio you reimplement all of
it:

| Concern | With `<audio>` (today) | With Web Audio |
|---|---|---|
| Download | Progressive + HTTP range; playback starts before full download | `decodeAudioData` needs the **entire** encoded file first |
| Seeking | `audio.currentTime = t` | Stop the source node, create a new one with a start offset |
| Pause / resume | Native | Source nodes can't pause — stop, record elapsed time, recreate with offset on resume |
| Progress / `currentTime` | `timeupdate` event | Derive from `AudioContext.currentTime` minus the track's start offset (rAF/interval) |
| Duration | `durationchange` event | `AudioBuffer.duration` |
| Volume | `audio.volume` | `GainNode.gain` (also enables crossfade) |
| Format decoding | Browser handles it | `decodeAudioData` uses browser codecs; FLAC/Opus support varies slightly by browser |

## Memory & bandwidth

Decoded PCM is large: roughly **~20 MB per stereo minute** (float32 @ 44.1 kHz).
A 5-minute FLAC decodes to ~100 MB+ in RAM; pre-buffering the next track doubles
the peak. You must manage buffer lifecycle aggressively (decode the next track
ahead, free the previous one) and you lose progressive streaming — the whole
encoded file is downloaded up front before playback can begin.

## Edge cases specific to this codebase

- **Queue model.** `playbackOrder.nextQueueIndex` already decides "what plays
  next" (honoring repeat `all`/`none`); reuse it to decide which buffer to
  schedule. Repeat `one` re-schedules the same buffer.
- **Queue edits mid-playback.** Reorder/insert/remove (`moveInQueue`,
  `insertIntoQueue`, `removeFromQueue`) must cancel and reschedule the pending
  next-track node, not just re-point a `src`.
- **Repeat / shuffle toggles** change the scheduled-next track — same
  reschedule requirement.
- **Persisted state.** `usePlayer` restores `queue`/`currentIndex`/`volume` from
  localStorage; the Web Audio engine must rehydrate to a paused, seekable state
  without auto-decoding everything.
- **Autoplay policy.** `AudioContext` starts `suspended` until a user gesture;
  call `ctx.resume()` on the first play interaction.
- **Cover/stream auth.** Stream URLs come from `subsonicClient.getStreamUrl`;
  `fetch` them the same way the element's `src` does today.

## Effort & risk

- **Approach A (shipped):** ~½–1 day, localized to `usePlayer.ts` + tests, low
  risk.
- **Approach B:** multi-day to weeks. It's effectively writing a small media
  engine (download, decode, schedule, seek, pause, progress, memory management).
  High risk and ongoing maintenance.

## When to choose B

Pick Approach B only if one of these becomes a hard requirement:

1. **Sample-accurate gapless** for continuous albums — Approach A's tiny seam is
   unacceptable.
2. **Crossfading** between tracks — Web Audio `GainNode` is the clean way to do
   it; the `<audio>` approach can't do it well.

Otherwise Approach A is sufficient and far cheaper.

## Possible hybrid

Keep Approach A as the default engine and only switch to a Web Audio engine for a
"gapless / crossfade" mode the user opts into. This avoids paying the memory and
complexity cost for the common case while still offering true gapless when asked.
The shared `nextQueueIndex` helper and the `usePlayer` public API (`play`,
`pause`, `seek`, `currentTime`, `duration`, `preloadedTrack`, …) make a
pluggable engine behind the same surface feasible.
