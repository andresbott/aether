# Player playback — engine trait, player lock contract, rodio actor

The playback subsystem spans `crates/aether-core/src/player/` (trait, queue,
orchestration — pure Rust, tested with fakes) and `src-tauri/src/engine/`
(per-OS implementations). Read this before touching either side; the
concurrency contract is easy to break and was formalized in a review round of
the bootstrap (2026-07-22). Paths are relative to `player/`.

## The concurrency contract (do not break)

`Player` holds its internal `Mutex<Inner>` **during every engine call**.
Consequences, documented on the `PlaybackEngine` trait
(`crates/aether-core/src/player/engine.rs`):

- **Engine implementations MUST NOT call back into `Player`** — instant
  deadlock.
- **`position()` and `is_idle()` MUST be fast and non-blocking** — they are
  called every 500 ms by the poll loop while the lock is held. The rodio
  engine satisfies this with a `Shared { position, idle }` mutex updated by
  the actor thread; new engines need an equivalent snapshot mechanism.
- **`load()` may block** (v0 downloads the whole file). While it blocks, the
  `Player` lock is held, so `state()`/`poll()`/every other player method
  stalls — this is the accepted v0 trade-off; progressive streaming is the
  catalogued follow-up. Because of this, every IPC command that can reach
  `load` runs on `spawn_blocking` (see [ipc.md](ipc.md)), and the tray's
  "Next Track" does the same — copy that pattern for any new caller.

## Roles

- **`Queue`** (`queue.rs`) — pure index-over-`Vec<String>` state machine.
  `next()`/`prev()` return `None` at the ends *without moving the index*.
  Track ids only; the UI owns track metadata.
- **`Player`** (`player.rs`) — orchestration: resolves track id → stream URL
  via `StreamUrlResolver`, drives the engine, owns paused/stopped/volume.
  `poll()` implements auto-advance: if not stopped/paused and the engine is
  idle, advance the queue and load, else mark stopped. On auto-advance
  failure it logs and stops — it does not retry.
- **`PlaybackEngine`** — the platform boundary. Selected in
  `src-tauri/src/engine/mod.rs` via `#[cfg(target_os)]`: `RodioEngine`
  (desktop), `AndroidEngine`/`IosEngine` (stubs returning
  `CoreError::Engine("… not implemented")` from `load`/`seek`, no-ops
  elsewhere). Stubs need `Default` impls — clippy on mobile targets demanded
  them, and clippy only runs on the host target locally (see
  [testing.md](testing.md)).
- **`StreamUrlResolver`** — implemented by `ConnectionResolver` in
  `src-tauri/src/state.rs`; returns `CoreError::NotConnected` when there is
  no connection. Keeps `aether-core` ignorant of where URLs come from — the
  future cache can sit behind another resolver/engine seam.

## RodioEngine (desktop) internals

`src-tauri/src/engine/desktop.rs` — an actor thread (`aether-audio`) owning
the rodio `OutputStream` + `Sink`; commands arrive over an mpsc channel.

- **Fail fast at startup:** the output stream is opened before `start()`
  returns (readiness channel); no audio device → `CoreError::Engine` at app
  startup, not at first play.
- The actor loop uses `recv_timeout(250ms)` so `Shared.position`/`idle` stay
  fresh even when no commands arrive.
- `do_load` fetches the full track with blocking reqwest into memory, then
  decodes via `rodio::Decoder` on a `Cursor` (v0 — see contract above).
- `load` replies over a per-call channel, so errors (bad URL, decode failure)
  surface synchronously to `Player`.

## Testing pattern

`player.rs` tests use `FakeEngine` (records loads, atomic idle/paused flags)
+ `FakeResolver` — extend these fakes rather than pulling rodio into core
tests. Engine behavior needing a real audio device is not CI-testable; keep
such logic minimal and behind the trait. See [testing.md](testing.md).
