# Mobile Sheet Navigation — Strategy & Design

**Date:** 2026-08-16
**Status:** Approved design, pending implementation
**Scope:** Replace the mobile shell's route-swap player navigation with one
always-mounted, gesture-driven Now Playing sheet, so every swipe in the
mini-player ⇄ Now Playing ⇄ queue chain follows the finger over real, mounted
UI — and the system back button walks the same chain down.
**Supersedes:** the player-navigation parts of
`2026-08-13-mobile-responsive-design.md` §4 and the `bfe6bef` ("rework
navigation") per-leg drag implementations.

---

## 1. Problem

After `bfe6bef`, each navigation leg on the phone is individually
finger-following, but every leg ends in a **route swap**, so continuity dies
at the commit point:

- Lifting the mini player drags the 3.5rem bar up to ~78px, the bar slides
  off, **then** `/` swaps in — Now Playing pops in with no spatial relation to
  the drag.
- Dragging the player face down slides the face off over the page background,
  **then** `/browse` swaps in — the drag reveals a blank surface, not the
  destination.
- Queue → face is a 48px threshold that fires a programmatic smooth scroll —
  a jump, not a follow.
- Back-button behavior is incidental: the face↔queue hash is written with
  `replace` (no entry), so back from the queue leaves Now Playing entirely;
  back from Now Playing goes to wherever history happens to point.

The root cause is architectural, not a tuning problem: **a route view cannot
follow a finger into a view that is not mounted yet**, and a dismissal cannot
reveal a view the router has already unmounted. No amount of per-leg polish
fixes that.

## 2. Decision: one hash-addressed sheet

Now Playing + queue become a single **always-mounted bottom sheet** rendered
by the mobile shell chrome (`MobileShell`), stacked over the route content.
The sheet has three vertical detents, and its state is addressed by the **URL
hash of whatever route is underneath**:

| Detent | Position | Hash | Visible |
|---|---|---|---|
| `collapsed` | 0 | *(none)* | Mini strip docked at the bottom of the content page |
| `playing` | 1 | `#playing` | Player face fills the screen |
| `queue` | 2 | `#queue` | Queue panel fills the screen |

`/library`, `/library#playing`, `/library#queue` are three addresses of the
same mounted content view with the sheet at three heights. Hash pushes are
real history entries, so **system back pops queue → playing → content page
with zero popstate machinery** — the property that killed the old
`PlayerSheet` overlay is preserved: routing still gives back-dismissal for
free. The invariant to protect in review: *the URL is the single source of
truth; gestures and buttons are just two ways to change it.*

Because the content view never unmounts:

- every swipe direction manipulates real UI (the dismiss drag reveals the
  live page underneath, scroll position intact);
- the "slide out, wait for `transitionend`, safety-timer, then navigate"
  machinery in `MiniPlayer` and `MobilePlayView` is deleted — a settle just
  commits the hash while CSS animates the transform independently;
- `prefersReducedMotion()` JS is deleted — nothing waits on a transition any
  more, so a plain CSS `@media (prefers-reduced-motion: reduce)` rule suffices.

### 2.1 Deliberate behavior change

Today the face's downward drag always lands on `/browse`. With the sheet,
collapsing returns to **the route underneath** — the page the player was
opened from (Spotify/Apple Music convention: dismiss = go back to where you
were). `/browse` stays one hamburger tap away on that page. The user's
"swipe down → landing page" reading still holds for the common case, because
`/` redirects onto `/browse#playing` (§4) and browse is where empty-queue
users start.

## 3. Gesture map

One continuous position `p ∈ [0, 2]` (0 = collapsed, 1 = playing, 2 = queue).
Every gesture moves `p` 1:1 with the finger (px mapped through the segment's
travel); release settles to a detent by **position + velocity** (flick ≥
0.5 px/ms wins over the midpoint rule); the settle commits the hash.

| Surface | Direction | Range | Notes |
|---|---|---|---|
| Mini strip | up | [0, 1] | Tap still opens (`#playing`); strip cross-fades out as the sheet rises |
| Player face | up / down | [0, 2] | One gesture can reveal the queue or dismiss; `.play-seek` never arms |
| Queue list | down | [1, 2] | Only when the list is at `scrollTop === 0`; claimed moves `preventDefault()` so the list doesn't fight the drag |
| Queue heading | down | [1, 2] | Works at any list scroll position (the escape hatch keeps its job); direction is now visible, so the old "either direction" quirk drops |

Claiming: a touch claims the gesture only after 8px of dominant-vertical
movement (`|dy| > 8 && |dy| > |dx|`), so taps, the seek slider, and the
transport buttons are untouched. A claimed gesture swallows the click the
browser delivers on release (capture-phase, once).

Non-gesture twins stay: mini-strip tap, the face's ⌄ (now "collapse", was
"to /browse") and ⌃ ("show queue") buttons — all route through the hash.

## 4. Routing & history invariants

- **Upward** transitions `push` the deeper hash: `…` → `…#playing` →
  `…#queue`. Back then walks down the chain: exactly the requested
  back-button behavior.
- **Downward** transitions call `router.back()` **iff** vue-router's own
  `window.history.state.back` equals the expected shallower address
  (repeated swipes never grow history); otherwise `router.replace()` (deep
  link / reload where no shallower entry exists).
- All transitions are single-step (±1 detent) by construction of the ranges.
- **`/` on mobile** becomes a pure alias: `replace` → `/browse#playing` when
  the queue is non-empty, `/browse` when empty. Desktop `/` (QueueView) is
  untouched. Old links and the MiniPlayer keep working.
- **Route path changes while expanded** (e.g. face's "go to album") drop the
  hash → the sheet watcher collapses it, revealing the new page. Back from
  the album returns to `…#playing` and the sheet reopens — history stays
  honest.
- **Queue empties / shell flips to desktop** → the sheet unmounts; its
  `onUnmounted` replaces a live sheet hash away so no stale `#playing`
  lingers on desktop URLs.
- **Reload on `…#playing` / `…#queue`** mounts the sheet at that detent with
  no animation.

Empty queue: no sheet, no strip, no entries — back is plain navigation, i.e.
"landing page", matching the requirement.

## 5. Component architecture

```
MobileShell
├── .mini-spacer                 (flex child reserving the strip's height)
└── NowPlayingSheet              (absolute inset:0 over .player-shell; owns ALL
    │                             gestures, transforms, hash sync)
    ├── MiniPlayer               (dumb bar again: renders + transport; emits `open`)
    ├── PlayerFace  (new)        (art / meta / seek / transport / hints; emits
    │                             `collapse`, `show-queue`)
    └── QueuePanel  (new)        (heading + shuffle/repeat + ⋮ overflow +
                                  QueueBody + SavePlaylistDialog; owns edit mode,
                                  exits it when the detent leaves `queue`)
```

- `lib/sheetGesture.ts` — pure math: position⇄travel mapping, drag clamping,
  velocity tracking, settle decision. Unit-testable without DOM.
- `composables/useNowPlayingSheet.ts` — module-scoped singleton (the
  `usePlayer` pattern): `detent`, `position`, `dragging`, `open`, `snapTo`,
  plus the hash↔detent mapping and `commitDetent` (push / back-or-replace).
  `PlayerLayout` reads `open` to set `inert` on the covered content (the
  lightweight replacement for the old focus-trap; likewise the collapsed
  sheet body and the hidden strip are `inert`).
- Transforms: outer `translateY(calc((100% − strip) · (1 − min(p,1))))` on the
  sheet; inner `translateY(−max(p−1,0) · 50%)` on a 200%-tall two-panel track.
  Transitions disabled while dragging and pre-first-paint; player palette
  token remap moves from `.mobile-play-view` to the sheet root.
- **Deleted:** `MobilePlayView.vue` (redistributed), the scroll-snap +
  `programmaticTarget` machinery, both leave/lift slide-out state machines,
  `lib/motion.ts`.

`HomeView` keeps only the desktop branch plus the mobile alias redirect.
`ContentScaffold` (hamburger → `/browse`) is untouched.

## 6. Error handling & degradation

- jsdom / no dimensions: travel math guards `max(1, …)`; the sheet defaults
  to `collapsed` and pure hash navigation still works with gestures inert.
- A transition that never runs no longer strands anything: no code waits on
  `transitionend`.
- Touch cancel (`touchcancel`) settles like a release.
- `history.state` missing/foreign → downward transitions fall back to
  `replace` (never a wrong `back()` out of the app).

## 7. Testing

- `sheetGesture` unit specs: mapping round-trips, clamping, flick vs midpoint
  settles, velocity window.
- Component specs follow the existing touch-trigger idiom
  (`trigger('touchstart', { touches: [{ clientY }] })`, mocked
  `offsetHeight`); `settleDetent` is partially mocked in the sheet's gesture
  spec so settles are deterministic (velocity depends on real timestamps).
- Route sync specs stub `window.history.replaceState({ back: … })` to pin the
  back-or-replace decision.
- Style guards keep their off-disk technique: palette remap and
  transition-gating move to `NowPlayingSheet`, safe-area assertions to
  `PlayerFace`/`QueuePanel`; `mobile-chrome.safeArea.spec.ts` updated.
- Manual gate (chrome-devtools emulation + real phone): all four swipe legs,
  flicks, back-button chain, reload on each hash, rotation, queue-empties,
  reduced motion.
