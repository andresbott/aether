# Mobile Sheet Navigation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the mobile shell's route-swap player navigation with one always-mounted, hash-addressed Now Playing sheet so mini-player ⇄ Now Playing ⇄ queue swipes follow the finger over mounted UI, and system back walks queue → playing → content page.

**Architecture:** A three-detent bottom sheet (`collapsed`/`playing`/`queue` ⇢ hash `''`/`#playing`/`#queue` on the current route) rendered by `MobileShell` over the never-unmounted content view. Pure gesture math in `lib/sheetGesture.ts`, a singleton state store in `useNowPlayingSheet`, DOM/gesture/route glue in `NowPlayingSheet.vue` composing `MiniPlayer` + new `PlayerFace` + new `QueuePanel`. Upward detent changes `push` the hash; downward ones `back()` when vue-router's `history.state.back` matches, else `replace()`. `MobilePlayView.vue` is deleted.

**Tech Stack:** Vue 3 `<script setup>` + TypeScript, vue-router 4, PrimeVue, Vitest + @vue/test-utils (jsdom), off-disk style-guard specs (node env).

**Spec:** `docs/superpowers/specs/2026-08-16-mobile-sheet-navigation-design.md` (read it first; this plan argues from it).

## Global Constraints

- Frontend-only: no `/rest` or `/api/v1` changes.
- No backwards compatibility / migration code (project rule): delete freely, no fallback branches.
- Desktop shell behavior must not change; `useViewport` breakpoints (`BP_PHONE_MAX = 768`, `BP_DESKTOP_MIN = 1024`, `BP_SHELL_MIN_HEIGHT = 600`) untouched.
- No `vh` units in global SCSS or `PlayerLayout` (guarded by `webui/src/assets/scss/__tests__/appShell.spec.ts`); safe-area `env()` insets must be preserved wherever code moves.
- Never run `npm run format` (rewrites ~130 unrelated files). Match existing file style by hand.
- All webui commands run from `webui/`. Full gate: `npm test` (= `vue-tsc --noEmit && vitest run`). Single specs: `npx vitest run <path>`.
- Commits: one-line message, no Co-Authored-By, `git add` as a separate step, never on `main` (work happens on `better-mobile`).
- The URL is the single source of truth for sheet state; gestures and buttons only change it (or animate toward it). Do not add parallel open/close state.

---

### Task 1: Gesture math library

**Files:**
- Create: `webui/src/lib/sheetGesture.ts`
- Test: `webui/src/lib/__tests__/sheetGesture.spec.ts`

**Interfaces:**
- Consumes: nothing (pure module).
- Produces (used verbatim by Task 6):
  - `SLOP_PX: number` (8), `FLICK_VELOCITY_PX_MS: number` (0.5)
  - `travelFor(position: number, viewportH: number, stripH: number): number`
  - `positionAtTravel(travel: number, viewportH: number, stripH: number): number`
  - `dragPosition(startPos: number, deltaY: number, viewportH: number, stripH: number, min: number, max: number): number`
  - `settleDetent(position: number, velocityY: number, min: number, max: number): number`
  - `class VelocityTracker { push(y: number, t: number): void; velocity(): number }`

Position model: `p ∈ [0,2]`; segment 0→1 spans `viewportH − stripH` px of finger travel, segment 1→2 spans `viewportH` px. `deltaY` is finger movement, **down positive** (screen coordinates), which *decreases* travel. `velocityY` is finger px/ms, down positive: flick down ⇒ lower detent.

- [ ] **Step 1: Write the failing test**

```ts
// webui/src/lib/__tests__/sheetGesture.spec.ts
import { describe, it, expect } from 'vitest'
import {
    dragPosition,
    positionAtTravel,
    settleDetent,
    travelFor,
    VelocityTracker,
    FLICK_VELOCITY_PX_MS,
    SLOP_PX
} from '@/lib/sheetGesture'

// One position axis for the whole sheet: 0 collapsed, 1 playing, 2 queue.
// Segment 0→1 spans (viewportH - stripH) finger-px (the strip is already on
// screen), segment 1→2 spans a full viewportH. deltaY is screen-down-positive.
const H = 800
const STRIP = 60

describe('travelFor / positionAtTravel', () => {
    it('maps the detents to their cumulative finger travel', () => {
        expect(travelFor(0, H, STRIP)).toBe(0)
        expect(travelFor(1, H, STRIP)).toBe(740)
        expect(travelFor(2, H, STRIP)).toBe(1540)
    })

    it('is piecewise linear inside each segment', () => {
        expect(travelFor(0.5, H, STRIP)).toBe(370)
        expect(travelFor(1.5, H, STRIP)).toBe(1140)
    })

    it('round-trips through its inverse', () => {
        for (const p of [0, 0.25, 0.5, 1, 1.3, 2]) {
            expect(positionAtTravel(travelFor(p, H, STRIP), H, STRIP)).toBeCloseTo(p, 10)
        }
    })

    it('clamps the inverse to [0, 2]', () => {
        expect(positionAtTravel(-50, H, STRIP)).toBe(0)
        expect(positionAtTravel(99999, H, STRIP)).toBe(2)
    })

    it('never divides by zero when jsdom reports no dimensions', () => {
        expect(positionAtTravel(travelFor(1, 0, 0), 0, 0)).toBeCloseTo(1, 10)
    })
})

describe('dragPosition', () => {
    it('moves up (finger dy negative) toward higher positions, 1:1 in travel', () => {
        // From collapsed, lifting 370px is half the 740px expand travel.
        expect(dragPosition(0, -370, H, STRIP, 0, 1)).toBeCloseTo(0.5, 10)
    })

    it('moves down toward lower positions', () => {
        expect(dragPosition(1, 370, H, STRIP, 0, 2)).toBeCloseTo(0.5, 10)
    })

    it('crosses segments in one gesture when the range allows it', () => {
        // From playing, lifting a full viewport reaches the queue.
        expect(dragPosition(1, -H, H, STRIP, 0, 2)).toBe(2)
    })

    it('clamps to the surface range', () => {
        // The strip only travels [0, 1]: overshoot stops at the face.
        expect(dragPosition(0, -2000, H, STRIP, 0, 1)).toBe(1)
        // The queue surfaces only travel [1, 2]: a hard pull stops at the face.
        expect(dragPosition(2, 2000, H, STRIP, 1, 2)).toBe(1)
    })
})

describe('settleDetent', () => {
    it('rounds to the nearest detent when the release is slow', () => {
        expect(settleDetent(0.4, 0, 0, 1)).toBe(0)
        expect(settleDetent(0.6, 0, 0, 1)).toBe(1)
        expect(settleDetent(1.5001, 0, 1, 2)).toBe(2)
    })

    it('a flick wins over the midpoint: direction decides', () => {
        // Barely moved, but flicked up hard → open anyway.
        expect(settleDetent(0.1, -FLICK_VELOCITY_PX_MS, 0, 1)).toBe(1)
        // Nearly open, but flicked down → dismiss.
        expect(settleDetent(0.9, FLICK_VELOCITY_PX_MS, 0, 1)).toBe(0)
    })

    it('a flick from an exact detent moves one step in its direction', () => {
        expect(settleDetent(1, -1, 0, 2)).toBe(2)
        expect(settleDetent(1, 1, 0, 2)).toBe(0)
    })

    it('clamps the target to the surface range', () => {
        expect(settleDetent(1, 1, 1, 2)).toBe(1)
        expect(settleDetent(1, -1, 0, 1)).toBe(1)
    })
})

describe('VelocityTracker', () => {
    it('reports px/ms over its sample window, sign preserved', () => {
        const t = new VelocityTracker()
        t.push(500, 0)
        t.push(480, 20)
        t.push(460, 40)
        expect(t.velocity()).toBeCloseTo(-1, 10)
    })

    it('drops samples older than its window so old motion cannot fake a flick', () => {
        const t = new VelocityTracker()
        t.push(500, 0) // fast start…
        t.push(400, 10)
        // …then the finger holds still for 300ms before release.
        t.push(400, 310)
        t.push(400, 320)
        expect(Math.abs(t.velocity())).toBeLessThan(FLICK_VELOCITY_PX_MS)
    })

    it('is zero with fewer than two samples or zero elapsed time', () => {
        const t = new VelocityTracker()
        expect(t.velocity()).toBe(0)
        t.push(100, 5)
        expect(t.velocity()).toBe(0)
        t.push(200, 5)
        expect(t.velocity()).toBe(0)
    })
})

describe('constants', () => {
    it('pins the tuning values the sheet component relies on', () => {
        expect(SLOP_PX).toBe(8)
        expect(FLICK_VELOCITY_PX_MS).toBe(0.5)
    })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run (from `webui/`): `npx vitest run src/lib/__tests__/sheetGesture.spec.ts`
Expected: FAIL — cannot resolve `@/lib/sheetGesture`.

- [ ] **Step 3: Write the implementation**

```ts
// webui/src/lib/sheetGesture.ts
/**
 * Pure math for the Now Playing sheet's drag gestures (NowPlayingSheet.vue).
 *
 * One position axis for the whole sheet: 0 = collapsed (mini strip), 1 =
 * playing (player face), 2 = queue. The two segments cover different finger
 * distances — 0→1 spans the viewport minus the strip already on screen, 1→2
 * spans a full viewport — so gestures map through cumulative TRAVEL px and
 * back, keeping the sheet 1:1 under the finger in both segments.
 *
 * No DOM here: NowPlayingSheet measures heights and feeds them in, which is
 * what makes flicks and clamping unit-testable.
 */

/** Movement below this is a tap (or the slider), never a drag claim. */
export const SLOP_PX = 8
/** Finger speed (px/ms) at which direction beats the midpoint rule. */
export const FLICK_VELOCITY_PX_MS = 0.5
/** Velocity looks at most this far back, so a pause before release kills a flick. */
const VELOCITY_WINDOW_MS = 100

const segments = (viewportH: number, stripH: number): [number, number] => [
    // max(1, …): jsdom reports 0 heights; gestures are inert there but the
    // math must not divide by zero.
    Math.max(1, viewportH - stripH),
    Math.max(1, viewportH)
]

/** Cumulative finger travel (px) from collapsed to `position`. */
export function travelFor(position: number, viewportH: number, stripH: number): number {
    const [expand, queue] = segments(viewportH, stripH)
    const p = Math.min(Math.max(position, 0), 2)
    return p <= 1 ? p * expand : expand + (p - 1) * queue
}

/** Inverse of `travelFor`, clamped to [0, 2]. */
export function positionAtTravel(travel: number, viewportH: number, stripH: number): number {
    const [expand, queue] = segments(viewportH, stripH)
    if (travel <= 0) return 0
    if (travel <= expand) return travel / expand
    return Math.min(2, 1 + (travel - expand) / queue)
}

/**
 * Position after the finger moved `deltaY` px (screen coordinates: down is
 * positive, which lowers the sheet), clamped to the surface's [min, max].
 */
export function dragPosition(
    startPos: number,
    deltaY: number,
    viewportH: number,
    stripH: number,
    min: number,
    max: number
): number {
    const travel = travelFor(startPos, viewportH, stripH) - deltaY
    const position = positionAtTravel(travel, viewportH, stripH)
    return Math.min(Math.max(position, min), max)
}

/**
 * Detent index to settle on at release: a flick moves one step in its
 * direction (from an exact detent too — the epsilon keeps floor/ceil from
 * treating "at 1" as "past 1"), anything slower rounds to the nearest.
 */
export function settleDetent(
    position: number,
    velocityY: number,
    min: number,
    max: number
): number {
    let target: number
    if (velocityY <= -FLICK_VELOCITY_PX_MS) target = Math.ceil(position + 1e-6)
    else if (velocityY >= FLICK_VELOCITY_PX_MS) target = Math.floor(position - 1e-6)
    else target = Math.round(position)
    return Math.min(Math.max(target, min), max)
}

/** Finger velocity over a trailing window, from touch event timestamps. */
export class VelocityTracker {
    private samples: Array<{ y: number; t: number }> = []

    push(y: number, t: number): void {
        this.samples.push({ y, t })
        const cutoff = t - VELOCITY_WINDOW_MS
        while (this.samples.length > 2 && this.samples[0].t < cutoff) {
            this.samples.shift()
        }
    }

    velocity(): number {
        if (this.samples.length < 2) return 0
        const first = this.samples[0]
        const last = this.samples[this.samples.length - 1]
        const dt = last.t - first.t
        return dt > 0 ? (last.y - first.y) / dt : 0
    }
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx vitest run src/lib/__tests__/sheetGesture.spec.ts`
Expected: PASS (all cases).

- [ ] **Step 5: Commit**

```bash
git add webui/src/lib/sheetGesture.ts webui/src/lib/__tests__/sheetGesture.spec.ts
git commit -m "feat(webui): pure gesture math for the now-playing sheet"
```

---

### Task 2: Sheet state singleton + hash routing helpers

**Files:**
- Create: `webui/src/composables/useNowPlayingSheet.ts`
- Test: `webui/src/composables/__tests__/useNowPlayingSheet.spec.ts`

**Interfaces:**
- Consumes: `vue` refs; `vue-router` types only.
- Produces (used verbatim by Tasks 4–7):
  - `type SheetDetent = 'collapsed' | 'playing' | 'queue'`
  - `DETENTS: SheetDetent[]` (index = position), `DETENT_POSITIONS: Record<SheetDetent, number>`
  - `detentForHash(hash: string): SheetDetent`, `hashForDetent(detent: SheetDetent): string`
  - `commitDetent(router: Router, from: SheetDetent, to: SheetDetent): void`
  - `useNowPlayingSheet(): { detent: Ref<SheetDetent>; position: Ref<number>; dragging: Ref<boolean>; open: ComputedRef<boolean>; snapTo(d: SheetDetent): void; reset(): void }` — module-scoped singleton (the `usePlayer` pattern)
  - `resetNowPlayingSheetForTests(): void`

- [ ] **Step 1: Write the failing test**

```ts
// webui/src/composables/__tests__/useNowPlayingSheet.spec.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { Router } from 'vue-router'
import {
    commitDetent,
    detentForHash,
    hashForDetent,
    resetNowPlayingSheetForTests,
    useNowPlayingSheet
} from '@/composables/useNowPlayingSheet'

beforeEach(() => {
    resetNowPlayingSheetForTests()
    // vue-router keeps the previous entry's fullPath in history.state.back;
    // commitDetent reads it to choose back() over replace(). Reset per test.
    window.history.replaceState({}, '', '/library')
})

describe('hash mapping', () => {
    it('maps the three sheet addresses both ways', () => {
        expect(detentForHash('')).toBe('collapsed')
        expect(detentForHash('#playing')).toBe('playing')
        expect(detentForHash('#queue')).toBe('queue')
        expect(hashForDetent('collapsed')).toBe('')
        expect(hashForDetent('playing')).toBe('#playing')
        expect(hashForDetent('queue')).toBe('#queue')
    })

    it('reads any foreign hash as collapsed — the sheet only owns its own two', () => {
        expect(detentForHash('#section-2')).toBe('collapsed')
    })
})

describe('useNowPlayingSheet', () => {
    it('is a singleton starting collapsed', () => {
        const a = useNowPlayingSheet()
        const b = useNowPlayingSheet()
        expect(a).toBe(b)
        expect(a.detent.value).toBe('collapsed')
        expect(a.position.value).toBe(0)
        expect(a.open.value).toBe(false)
    })

    it('snapTo moves detent and position together and clears dragging', () => {
        const sheet = useNowPlayingSheet()
        sheet.dragging.value = true
        sheet.snapTo('queue')
        expect(sheet.detent.value).toBe('queue')
        expect(sheet.position.value).toBe(2)
        expect(sheet.dragging.value).toBe(false)
        expect(sheet.open.value).toBe(true)
    })

    it('reset returns to collapsed', () => {
        const sheet = useNowPlayingSheet()
        sheet.snapTo('playing')
        sheet.reset()
        expect(sheet.detent.value).toBe('collapsed')
        expect(sheet.position.value).toBe(0)
    })
})

describe('commitDetent', () => {
    const makeRouter = () => {
        const router = {
            push: vi.fn(),
            replace: vi.fn(),
            back: vi.fn(),
            resolve: vi.fn(({ hash }: { hash: string }) => ({ fullPath: `/library${hash}` }))
        }
        return router as unknown as Router & typeof router
    }

    it('pushes when going deeper, so back can walk the chain down', () => {
        const router = makeRouter()
        commitDetent(router, 'collapsed', 'playing')
        expect(router.push).toHaveBeenCalledWith({ hash: '#playing' })
        commitDetent(router, 'playing', 'queue')
        expect(router.push).toHaveBeenCalledWith({ hash: '#queue' })
        expect(router.back).not.toHaveBeenCalled()
    })

    it('pops the matching entry when going shallower — repeated swipes must not grow history', () => {
        const router = makeRouter()
        window.history.replaceState({ back: '/library#playing' }, '', '/library#queue')
        commitDetent(router, 'queue', 'playing')
        expect(router.back).toHaveBeenCalledOnce()
        expect(router.replace).not.toHaveBeenCalled()
    })

    it('rewrites in place when there is no matching entry (deep link / reload)', () => {
        const router = makeRouter()
        window.history.replaceState({ back: null }, '', '/library#playing')
        commitDetent(router, 'playing', 'collapsed')
        expect(router.replace).toHaveBeenCalledWith({ hash: '' })
        expect(router.back).not.toHaveBeenCalled()
    })

    it('never backs into a DIFFERENT page — only the exact shallower address counts', () => {
        const router = makeRouter()
        window.history.replaceState({ back: '/album/9' }, '', '/library#playing')
        commitDetent(router, 'playing', 'collapsed')
        expect(router.replace).toHaveBeenCalledWith({ hash: '' })
        expect(router.back).not.toHaveBeenCalled()
    })

    it('does nothing for a same-detent settle (spring-back)', () => {
        const router = makeRouter()
        commitDetent(router, 'playing', 'playing')
        expect(router.push).not.toHaveBeenCalled()
        expect(router.replace).not.toHaveBeenCalled()
        expect(router.back).not.toHaveBeenCalled()
    })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx vitest run src/composables/__tests__/useNowPlayingSheet.spec.ts`
Expected: FAIL — cannot resolve `@/composables/useNowPlayingSheet`.

- [ ] **Step 3: Write the implementation**

```ts
// webui/src/composables/useNowPlayingSheet.ts
import { computed, ref, type ComputedRef, type Ref } from 'vue'
import type { Router } from 'vue-router'

/**
 * State store for the mobile Now Playing sheet (NowPlayingSheet.vue) — the
 * usePlayer-style singleton, so PlayerLayout can read `open` for the inert
 * gate and QueuePanel can watch `detent` without prop-drilling through the
 * sheet.
 *
 * The sheet's REAL source of truth is the route hash ('' | #playing |
 * #queue); this store only mirrors it plus the transient gesture position.
 * Detent changes therefore go through commitDetent (which navigates) and the
 * sheet's hash watcher (which calls snapTo) — never write `detent` from
 * anywhere else.
 */
export type SheetDetent = 'collapsed' | 'playing' | 'queue'

/** Index = the detent's position on the sheet's 0..2 axis. */
export const DETENTS: SheetDetent[] = ['collapsed', 'playing', 'queue']
export const DETENT_POSITIONS: Record<SheetDetent, number> = {
    collapsed: 0,
    playing: 1,
    queue: 2
}

export function detentForHash(hash: string): SheetDetent {
    if (hash === '#playing') return 'playing'
    if (hash === '#queue') return 'queue'
    return 'collapsed'
}

export function hashForDetent(detent: SheetDetent): string {
    if (detent === 'playing') return '#playing'
    if (detent === 'queue') return '#queue'
    return ''
}

/**
 * Navigate the route hash from one detent to another. Deeper is a push, so
 * system back walks the chain down (#queue → #playing → page). Shallower
 * prefers popping the entry the matching push created — repeated swipes must
 * not grow history — and falls back to replace() when there is no such entry
 * (deep link, reload): never back() out of the app.
 *
 * All transitions are single-step by construction (the gesture ranges only
 * ever cross one detent), so one back() is always enough.
 */
export function commitDetent(router: Router, from: SheetDetent, to: SheetDetent): void {
    if (to === from) return
    const hash = hashForDetent(to)
    if (DETENT_POSITIONS[to] > DETENT_POSITIONS[from]) {
        void router.push({ hash })
        return
    }
    const target = router.resolve({ hash }).fullPath
    const back =
        typeof window !== 'undefined'
            ? (window.history.state as { back?: unknown } | null)?.back
            : null
    if (typeof back === 'string' && back === target) router.back()
    else void router.replace({ hash })
}

interface NowPlayingSheetState {
    /** Settled detent — mirrors the route hash. */
    detent: Ref<SheetDetent>
    /** Continuous 0..2 position; equals the detent's index except mid-gesture. */
    position: Ref<number>
    /** A finger owns the position: transitions off while true. */
    dragging: Ref<boolean>
    /** Anything above collapsed — PlayerLayout's inert gate. */
    open: ComputedRef<boolean>
    snapTo: (d: SheetDetent) => void
    reset: () => void
}

let state: NowPlayingSheetState | null = null

function createState(): NowPlayingSheetState {
    const detent = ref<SheetDetent>('collapsed')
    const position = ref(0)
    const dragging = ref(false)
    const open = computed(() => detent.value !== 'collapsed')
    const snapTo = (d: SheetDetent): void => {
        detent.value = d
        position.value = DETENT_POSITIONS[d]
        dragging.value = false
    }
    const reset = (): void => snapTo('collapsed')
    return { detent, position, dragging, open, snapTo, reset }
}

export function useNowPlayingSheet(): NowPlayingSheetState {
    if (!state) state = createState()
    return state
}

/** Test hook: drop the singleton so the next call starts collapsed. */
export function resetNowPlayingSheetForTests(): void {
    state = null
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx vitest run src/composables/__tests__/useNowPlayingSheet.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add webui/src/composables/useNowPlayingSheet.ts webui/src/composables/__tests__/useNowPlayingSheet.spec.ts
git commit -m "feat(webui): now-playing sheet state singleton and hash commit helpers"
```

---

### Task 3: PlayerFace component (extracted from MobilePlayView)

**Files:**
- Create: `webui/src/components/layout/PlayerFace.vue`
- Test: `webui/src/components/layout/__tests__/PlayerFace.spec.ts`

`MobilePlayView.vue` is NOT touched in this task (it dies in Task 7); the face
markup/styles are copied out so both exist temporarily.

**Interfaces:**
- Consumes: `usePlayer`, `useCurrentTrackFavorite`, `subsonicClient` (all existing).
- Produces: `<PlayerFace @collapse @show-queue />` — no props; emits `collapse` (⌄ button) and `show-queue` (⌃ button). Root element class `play-face`; the seek wrapper keeps class `play-seek` (Task 6's gesture exemption targets it).

- [ ] **Step 1: Write the failing test**

```ts
// webui/src/components/layout/__tests__/PlayerFace.spec.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

const push = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ push })
}))

const queue = ref<Array<Record<string, unknown>>>([])
const currentTrack = ref<Record<string, unknown> | null>(null)
const isPlaying = ref(false)
const seek = vi.fn()
const togglePlayPause = vi.fn()
const playNext = vi.fn()
const playPrevious = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        queue,
        currentTrack,
        isPlaying,
        currentTime: ref(30),
        duration: ref(120),
        hasNext: ref(true),
        hasPrevious: ref(true),
        seek,
        togglePlayPause,
        playNext,
        playPrevious
    })
}))

const toggleFavorite = vi.fn()
const isStarred = ref(false)
vi.mock('@/composables/useCurrentTrackFavorite', () => ({
    useCurrentTrackFavorite: () => ({ isStarred, toggleFavorite })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size: number) => `/art/${id}?size=${size}`
    }
}))

import PlayerFace from '@/components/layout/PlayerFace.vue'

beforeEach(() => {
    currentTrack.value = {
        id: '2',
        title: 'Song 2',
        artist: 'Artist',
        albumId: 'al2',
        artistId: 'ar2',
        coverArt: 'cov-2'
    }
    isStarred.value = false
    isPlaying.value = false
    vi.clearAllMocks()
})

describe('PlayerFace', () => {
    it('shows track, artist and cover art', () => {
        const w = mount(PlayerFace)
        expect(w.find('.play-title').text()).toBe('Song 2')
        expect(w.find('.play-artist').text()).toBe('Artist')
        expect(w.find('img.play-cover').attributes('src')).toBe('/art/cov-2?size=512')
    })

    it('wires the prev / play / next transport to the player', async () => {
        const w = mount(PlayerFace)
        await w.find('[aria-label="Play"]').trigger('click')
        expect(togglePlayPause).toHaveBeenCalledOnce()
        await w.find('[aria-label="Next track"]').trigger('click')
        expect(playNext).toHaveBeenCalledOnce()
        await w.find('[aria-label="Previous track"]').trigger('click')
        expect(playPrevious).toHaveBeenCalledOnce()
    })

    it('carries no shuffle/repeat — those are queue behaviour (QueuePanel)', () => {
        const w = mount(PlayerFace)
        expect(w.find('[aria-label="Shuffle"]').exists()).toBe(false)
        expect(w.find('[aria-label="Repeat"]').exists()).toBe(false)
    })

    it('seeking through the range input calls seek', async () => {
        const w = mount(PlayerFace)
        await w.find('input[type="range"]').setValue('45')
        expect(seek).toHaveBeenCalledWith(45)
    })

    it('title and artist navigate to the album and artist routes', async () => {
        const w = mount(PlayerFace)
        await w.find('.play-title').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'album', params: { id: 'al2' } })
        await w.find('.play-artist').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'artist', params: { id: 'ar2' } })
    })

    it('disables the title/artist links when the track has no ids', () => {
        currentTrack.value = { id: '1', title: 'Song 1', artist: 'Artist' }
        const w = mount(PlayerFace)
        expect(w.find('.play-title').attributes('disabled')).toBeDefined()
        expect(w.find('.play-artist').attributes('disabled')).toBeDefined()
    })

    it('double-tapping the cover flips the favorite; a single tap does not', async () => {
        const w = mount(PlayerFace)
        const art = w.find('.play-art')
        await art.trigger('click')
        expect(toggleFavorite).not.toHaveBeenCalled()
        await art.trigger('click')
        expect(toggleFavorite).toHaveBeenCalledOnce()
    })

    it('shows the heart indicator on the cover only while starred', async () => {
        const w = mount(PlayerFace)
        expect(w.find('.play-favorite-indicator').exists()).toBe(false)
        isStarred.value = true
        await w.vm.$nextTick()
        const heart = w.find('.play-art .play-favorite-indicator')
        expect(heart.classes()).toContain('pi-heart-fill')
        expect(heart.attributes('aria-hidden')).toBe('true')
    })

    // The two swipe affordances are real buttons, so neither destination is
    // gesture-only — but they only EMIT: the sheet owns navigation.
    it('the ⌄ hint emits collapse and the ⌃ hint emits show-queue', async () => {
        const w = mount(PlayerFace)
        await w.find('button.play-nav-hint').trigger('click')
        expect(w.emitted('collapse')).toHaveLength(1)
        await w.find('button.play-swipe-hint').trigger('click')
        expect(w.emitted('show-queue')).toHaveLength(1)
        expect(push).not.toHaveBeenCalled()
    })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx vitest run src/components/layout/__tests__/PlayerFace.spec.ts`
Expected: FAIL — cannot resolve `@/components/layout/PlayerFace.vue`.

- [ ] **Step 3: Create the component**

Copy the face out of `MobilePlayView.vue` with the gesture code removed (the sheet owns gestures) and the hint buttons rewired to emits:

```vue
<!-- webui/src/components/layout/PlayerFace.vue -->
<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useCurrentTrackFavorite } from '@/composables/useCurrentTrackFavorite'
import { usePlayer } from '@/composables/usePlayer'
import { subsonicClient } from '@/lib/api/subsonic'

// The Now Playing sheet's expanded face (NowPlayingSheet.vue): cover art,
// title/artist links, seek, and a prev/play/next transport — shuffle/repeat
// are queue behaviour and live in QueuePanel's heading. Deliberately free of
// gesture code: the sheet owns every drag, this face only names the two
// non-gesture twins of its swipes (⌄ collapse, ⌃ show queue) as emits.
const emit = defineEmits<{ (e: 'collapse'): void; (e: 'show-queue'): void }>()

const player = usePlayer()
const router = useRouter()
const { isStarred, toggleFavorite } = useCurrentTrackFavorite()

const currentTrack = computed(() => player.currentTrack.value)

const coverUrl = computed(() => {
    const art = currentTrack.value?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 512)
})

const formatTime = (seconds: number): string => {
    if (!seconds || !isFinite(seconds)) return '0:00'
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const onSeekInput = (event: Event): void => {
    player.seek(Number((event.target as HTMLInputElement).value))
}

// Favorite is a gesture here, not a button: double-tap the cover to toggle,
// with the starred state echoed by the corner indicator below. Detected by
// click timing because iOS Safari doesn't fire dblclick for double-taps;
// `touch-action: manipulation` on the art removes the double-tap zoom that
// would otherwise swallow the second tap.
const DOUBLE_TAP_MS = 300
let lastArtTap = 0
const onArtTap = (): void => {
    const now = Date.now()
    if (now - lastArtTap <= DOUBLE_TAP_MS) {
        lastArtTap = 0
        toggleFavorite()
        return
    }
    lastArtTap = now
}

const goAlbum = (): void => {
    const id = currentTrack.value?.albumId
    if (id) void router.push({ name: 'album', params: { id } })
}

const goArtist = (): void => {
    const id = currentTrack.value?.artistId
    if (id) void router.push({ name: 'artist', params: { id } })
}
</script>

<template>
    <section class="play-face">
        <!-- The face's only chrome, and the non-gesture twin of the dismiss
             drag: where every other view carries the hamburger, this one
             carries a chevron that collapses the sheet back onto the page. -->
        <button
            type="button"
            class="play-nav-hint"
            aria-label="Close Now Playing"
            @click="emit('collapse')"
        >
            <i class="pi pi-angle-down" aria-hidden="true"></i>
        </button>

        <div class="play-art" @click="onArtTap">
            <img v-if="coverUrl" :src="coverUrl" alt="" class="play-cover" />
            <div v-else class="play-cover play-cover--placeholder" aria-hidden="true">
                <i class="pi pi-music"></i>
            </div>
            <!-- Passive echo of the starred state, set by double-tapping the
                 cover. Decorative only (the queue rows' hearts are the
                 accessible toggle), hence aria-hidden. -->
            <i
                v-if="isStarred"
                class="pi pi-heart-fill play-favorite-indicator"
                aria-hidden="true"
            ></i>
        </div>

        <div class="play-meta">
            <button
                type="button"
                class="play-title"
                :disabled="!currentTrack?.albumId"
                @click="goAlbum"
            >
                {{ currentTrack?.title }}
            </button>
            <button
                type="button"
                class="play-artist"
                :disabled="!currentTrack?.artistId"
                @click="goArtist"
            >
                {{ currentTrack?.artist }}
            </button>
        </div>

        <div class="play-seek">
            <input
                type="range"
                class="play-range"
                aria-label="Seek"
                min="0"
                :max="player.duration.value || 0"
                step="1"
                :value="player.currentTime.value"
                @input="onSeekInput"
            />
            <div class="play-times">
                <span class="play-time">{{ formatTime(player.currentTime.value) }}</span>
                <span class="play-time">{{ formatTime(player.duration.value) }}</span>
            </div>
        </div>

        <!-- Prev / play / next only: shuffle and repeat live with the queue
             heading (QueuePanel). -->
        <div class="play-transport">
            <button
                type="button"
                class="play-btn"
                aria-label="Previous track"
                :disabled="!player.hasPrevious.value"
                @click="player.playPrevious()"
            >
                <i class="pi pi-step-backward"></i>
            </button>
            <button
                type="button"
                class="play-btn play-btn--play"
                :aria-label="player.isPlaying.value ? 'Pause' : 'Play'"
                @click="player.togglePlayPause()"
            >
                <i :class="player.isPlaying.value ? 'pi pi-pause' : 'pi pi-play'"></i>
            </button>
            <button
                type="button"
                class="play-btn"
                aria-label="Next track"
                :disabled="!player.hasNext.value"
                @click="player.playNext()"
            >
                <i class="pi pi-step-forward"></i>
            </button>
        </div>

        <button
            type="button"
            class="play-swipe-hint"
            aria-label="Show queue"
            @click="emit('show-queue')"
        >
            <i class="pi pi-angle-up" aria-hidden="true"></i>
        </button>
    </section>
</template>

<style scoped>
.play-face {
    position: relative;
    height: 100%;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1.25rem;
    /* Reserves BOTH insets: expanded, the face is the whole screen — the
       status bar (or the notch, in landscape) would otherwise sit on the nav
       chevron, and there is no mini strip below it to hold the home inset. */
    padding: calc(0.25rem + env(safe-area-inset-top)) 1.5rem
        calc(0.5rem + env(safe-area-inset-bottom));
}

.play-art {
    /* This margin pairs with .play-meta's margin-bottom: equal spare height
       above the art and below the title/artist centers the two as one group
       on a tall screen, while the seek bar and buttons anchor to the bottom
       where thumbs are — instead of the whole stack floating mid-screen. On a
       short screen the auto margins shrink to zero and the face packs as
       before. */
    margin-top: auto;
    position: relative;
    /* Full padded width — the face's 1.5rem side padding is the only border —
       capped by height so the transport keeps room on short screens. dvh, not
       vh: the cap has to track the height the panel actually gets. */
    width: min(100%, 45dvh);
    aspect-ratio: 1;
    /* Kill the browser's double-tap zoom so the favorite gesture gets both
       taps, and the 300ms click delay with it. */
    touch-action: manipulation;
    -webkit-user-select: none;
    user-select: none;
}

.play-cover {
    width: 100%;
    height: 100%;
    border-radius: var(--app-radius);
    object-fit: cover;
}

.play-cover--placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: var(--app-hover);
    color: var(--app-text-secondary);
    font-size: 3rem;
}

.play-meta {
    /* The other half of .play-art's margin-top — see the comment there. */
    margin-bottom: auto;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.25rem;
    max-width: 100%;
}

.play-title,
.play-artist {
    border: none;
    background: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
    max-width: 80vw;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.play-title {
    font-size: 1.25rem;
    font-weight: 700;
}

.play-artist {
    font-size: 0.95rem;
    color: var(--app-text-secondary);
}

.play-title:disabled,
.play-artist:disabled {
    cursor: default;
}

.play-seek {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    width: 100%;
}

.play-range {
    width: 100%;
    accent-color: var(--app-accent);
}

.play-times {
    display: flex;
    justify-content: space-between;
}

.play-time {
    font-size: 0.75rem;
    font-family: var(--app-player-time-font);
    color: var(--app-text-secondary);
}

.play-transport {
    display: flex;
    align-items: center;
    /* Three buttons only (shuffle/repeat live in the queue heading), so the
       row spreads: thumb-sized gaps instead of a tight cluster. */
    gap: 2.5rem;
}

.play-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2.75rem;
    height: 2.75rem;
    border: none;
    background: none;
    color: var(--app-text-primary);
    cursor: pointer;
    font-size: 1.15rem;
}

.play-btn:disabled {
    color: var(--app-text-secondary);
    cursor: default;
}

.play-btn--play {
    width: 3.5rem;
    height: 3.5rem;
    border-radius: 50%;
    background-color: var(--app-accent);
    /* Against the accent disc the icon matches the view's surface, not the
       page background (near-white in light theme). */
    color: var(--app-player-bg);
    font-size: 1.3rem;
}

/* The two swipe affordances, one per direction: ⌄ at the top collapses the
   sheet, ⌃ under the transport reveals the queue. Real buttons, so neither
   destination is gesture-only. */
.play-nav-hint,
.play-swipe-hint {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2.75rem;
    height: 2rem;
    border: none;
    background: none;
    color: var(--app-text-secondary);
    cursor: pointer;
    font-size: 1.15rem;
}

.play-nav-hint {
    /* Sits at the face's top edge because .play-art's margin-top: auto absorbs
       the spare height below it; a short screen must squash the artwork, not
       this. */
    flex-shrink: 0;
}

/* Favorites read by the FILL alone, not by colour (see TrackFavoriteButton /
   unified-play-experience.md). Sits on the cover's bottom-right corner, so
   it needs a shadow, not a theme colour: the backdrop is arbitrary artwork. */
.play-favorite-indicator {
    position: absolute;
    right: 0.6rem;
    bottom: 0.6rem;
    font-size: 1rem;
    color: #fff;
    filter: drop-shadow(0 1px 2px rgb(0 0 0 / 0.6));
}
</style>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx vitest run src/components/layout/__tests__/PlayerFace.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add webui/src/components/layout/PlayerFace.vue webui/src/components/layout/__tests__/PlayerFace.spec.ts
git commit -m "feat(webui): extract PlayerFace from MobilePlayView for the sheet"
```

---

### Task 4: QueuePanel component (extracted from MobilePlayView)

**Files:**
- Create: `webui/src/components/layout/QueuePanel.vue`
- Test: `webui/src/components/layout/__tests__/QueuePanel.spec.ts`

**Interfaces:**
- Consumes: `useNowPlayingSheet().detent` (Task 2), `usePlayer`, `useQueueActions`, `useQueueEdit`, `useQueueSummary`, `QueueBody`, `QueueHeaderActions`, `SavePlaylistDialog` (all existing).
- Produces: `<QueuePanel />` — no props/emits. Root class `play-queue`; heading keeps class `queue-heading` and the list wrapper class `play-queue-list` (Task 6's gesture delegation targets both). Owns its `useQueueEdit()` instance and exits edit mode whenever the sheet detent leaves `queue`.

- [ ] **Step 1: Write the failing test**

```ts
// webui/src/components/layout/__tests__/QueuePanel.spec.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'
import {
    resetNowPlayingSheetForTests,
    useNowPlayingSheet
} from '@/composables/useNowPlayingSheet'

const queue = ref<Array<Record<string, unknown>>>([{ id: '1' }, { id: '2' }, { id: '3' }])
const shuffle = ref(false)
const repeat = ref<'none' | 'all' | 'one'>('none')
const toggleShuffle = vi.fn()
const toggleRepeat = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ queue, shuffle, repeat, toggleShuffle, toggleRepeat })
}))

vi.mock('@/composables/useQueueSummary', () => ({
    useQueueSummary: () => ({ trackCount: ref(3), summary: ref('3 tracks • 3 min') })
}))

const openSaveDialog = vi.fn()
const clearQueue = vi.fn()
vi.mock('@/composables/useQueueActions', () => ({
    useQueueActions: () => ({
        showSaveDialog: ref(false),
        playlistName: ref(''),
        openSaveDialog,
        handleSave: vi.fn(),
        isSaving: ref(false),
        clearQueue
    })
}))

vi.mock('@/components/layout/QueueBody.vue', () => ({
    default: {
        name: 'QueueBody',
        props: ['variant', 'editMode'],
        template: '<div class="stub-queue-body">{{ variant }}</div>'
    }
}))

vi.mock('@/components/layout/SavePlaylistDialog.vue', () => ({
    default: { name: 'SavePlaylistDialog', template: '<div class="stub-save-dialog"></div>' }
}))

import QueuePanel from '@/components/layout/QueuePanel.vue'

const mountPanel = () =>
    mount(QueuePanel, { global: { plugins: [PrimeVue], directives: { tooltip: {} } } })

// The Popover teleports to document.body and mounted wrappers accumulate
// across tests, so always take the LAST match — that is this test's panel.
const overflowAction = (selector: string): HTMLElement => {
    const all = document.body.querySelectorAll<HTMLElement>(selector)
    const el = all[all.length - 1]
    expect(el).toBeTruthy()
    return el
}
const openOverflow = async (w: ReturnType<typeof mountPanel>) => {
    await w.find('.queue-heading-actions .queue-overflow-btn').trigger('click')
}

beforeEach(() => {
    resetNowPlayingSheetForTests()
    useNowPlayingSheet().snapTo('queue')
    shuffle.value = false
    repeat.value = 'none'
    vi.clearAllMocks()
})

describe('QueuePanel', () => {
    it('renders the queue heading with the shared summary', () => {
        const w = mountPanel()
        expect(w.find('.queue-heading h2').text()).toBe('Queue')
        expect(w.find('.queue-heading-summary').text()).toBe('3 tracks • 3 min')
        expect(w.find('.stub-queue-body').text()).toBe('sidebar')
    })

    it('shuffle and repeat sit in the heading, wired to the player', async () => {
        const w = mountPanel()
        const actions = w.find('.queue-heading-actions')
        await actions.find('[aria-label="Shuffle"]').trigger('click')
        expect(toggleShuffle).toHaveBeenCalledOnce()
        await actions.find('[aria-label="Repeat"]').trigger('click')
        expect(toggleRepeat).toHaveBeenCalledOnce()
    })

    it('shuffle and repeat read their pressed state from the player', () => {
        shuffle.value = true
        repeat.value = 'all'
        const w = mountPanel()
        expect(w.find('.queue-action-shuffle').attributes('aria-pressed')).toBe('true')
        expect(w.find('.queue-action-shuffle').classes()).toContain('is-active')
        expect(w.find('.queue-action-repeat').attributes('aria-pressed')).toBe('true')
        expect(w.find('.queue-action-repeat').classes()).toContain('is-active')
    })

    it('edit, save and clear collapse behind the heading ⋮ menu', async () => {
        const w = mountPanel()
        expect(w.find('.queue-heading-actions .queue-action-save').exists()).toBe(false)
        await openOverflow(w)
        const save = overflowAction('.queue-action-save')
        expect(save.textContent).toContain('Save as playlist')
        save.click()
        expect(openSaveDialog).toHaveBeenCalledOnce()
        overflowAction('.queue-action-clear').click()
        expect(clearQueue).toHaveBeenCalledOnce()
    })

    it('the pencil in the ⋮ menu toggles edit mode on the queue body', async () => {
        const w = mountPanel()
        const body = w.findComponent({ name: 'QueueBody' })
        expect(body.props('editMode')).toBe(false)
        await openOverflow(w)
        overflowAction('.queue-action-edit').click()
        await w.vm.$nextTick()
        expect(body.props('editMode')).toBe(true)
    })

    // Edit mode is queue-panel UI: leaving the queue detent — by swipe, hint
    // or back button, all of which move the sheet — ends the session, so
    // returning never lands on a stale selection.
    it('leaving the queue detent exits edit mode', async () => {
        const w = mountPanel()
        await openOverflow(w)
        overflowAction('.queue-action-edit').click()
        await w.vm.$nextTick()
        expect(w.findComponent({ name: 'QueueBody' }).props('editMode')).toBe(true)

        useNowPlayingSheet().snapTo('playing')
        await w.vm.$nextTick()
        expect(w.findComponent({ name: 'QueueBody' }).props('editMode')).toBe(false)
    })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx vitest run src/components/layout/__tests__/QueuePanel.spec.ts`
Expected: FAIL — cannot resolve `@/components/layout/QueuePanel.vue`.

- [ ] **Step 3: Create the component**

```vue
<!-- webui/src/components/layout/QueuePanel.vue -->
<script setup lang="ts">
import { ref, watch } from 'vue'
import Button from 'primevue/button'
import Popover from 'primevue/popover'
import QueueBody from '@/components/layout/QueueBody.vue'
import QueueHeaderActions from '@/components/layout/QueueHeaderActions.vue'
import SavePlaylistDialog from '@/components/layout/SavePlaylistDialog.vue'
import { useNowPlayingSheet } from '@/composables/useNowPlayingSheet'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueActions } from '@/composables/useQueueActions'
import { useQueueEdit } from '@/composables/useQueueEdit'
import { useQueueSummary } from '@/composables/useQueueSummary'

// The Now Playing sheet's queue panel (NowPlayingSheet.vue): the heading with
// the queue's own actions, and the shared QueueBody rows. The heading rides
// in with the panel — no fixed bar over the player face, nothing to fade.
// Gesture-free on purpose: the sheet owns the drags and reaches the heading /
// list by class (queue-heading / play-queue-list) through event delegation.
const player = usePlayer()
const { showSaveDialog, playlistName, openSaveDialog, handleSave, isSaving, clearQueue } =
    useQueueActions()
const { editMode, toggleEditMode, exitEditMode } = useQueueEdit()
const { trackCount, summary } = useQueueSummary()
const { detent } = useNowPlayingSheet()

// Edit mode is queue-panel UI: leaving the queue detent — by swipe, hint
// button or back button, all of which move the sheet — ends the editing
// session, so returning to the queue never lands on a stale selection.
watch(detent, (d) => {
    if (d !== 'queue') exitEditMode()
})

// The queue-management trio (edit/save/clear) behind the heading's ⋮ — three
// more bare glyphs next to shuffle/repeat would not read as a toolbar on a
// phone. Labeled inside the popover, since tooltips don't exist on touch.
const overflowRef = ref<InstanceType<typeof Popover> | null>(null)
const toggleOverflow = (event: Event) => overflowRef.value?.toggle(event)
</script>

<template>
    <div class="play-queue">
        <header class="queue-heading">
            <div class="queue-heading-text">
                <h2>Queue</h2>
                <span class="queue-heading-summary">{{ summary }}</span>
            </div>
            <!-- Shuffle and repeat are QUEUE behaviour, so they sit with the
                 queue heading rather than in the face's transport row. -->
            <div class="queue-heading-actions">
                <Button
                    class="queue-action-shuffle"
                    icon="pi pi-arrow-right-arrow-left"
                    text
                    rounded
                    size="small"
                    :class="{ 'is-active': player.shuffle.value }"
                    :aria-pressed="player.shuffle.value"
                    aria-label="Shuffle"
                    @click="player.toggleShuffle()"
                />
                <Button
                    class="queue-action-repeat"
                    icon="pi pi-refresh"
                    text
                    rounded
                    size="small"
                    :class="{ 'is-active': player.repeat.value !== 'none' }"
                    :aria-pressed="player.repeat.value !== 'none'"
                    aria-label="Repeat"
                    @click="player.toggleRepeat()"
                />
                <Button
                    class="queue-overflow-btn"
                    icon="pi pi-ellipsis-v"
                    text
                    rounded
                    size="small"
                    aria-label="More actions"
                    @click="toggleOverflow"
                />
                <Popover ref="overflowRef">
                    <div class="queue-overflow-panel">
                        <QueueHeaderActions
                            :edit-mode="editMode"
                            :disabled="trackCount === 0"
                            size="small"
                            labels
                            @toggle-edit="toggleEditMode"
                            @save="openSaveDialog"
                            @clear="clearQueue"
                        />
                    </div>
                </Popover>
            </div>
        </header>
        <div class="play-queue-list">
            <QueueBody variant="sidebar" :edit-mode="editMode" />
        </div>

        <SavePlaylistDialog
            v-model:visible="showSaveDialog"
            v-model:name="playlistName"
            :saving="isSaving"
            @save="handleSave"
        />
    </div>
</template>

<style scoped>
.play-queue {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
}

/* The queue's own heading, inside its panel: it slides in with the queue.
   Reserves the TOP inset — it is the topmost surface on this panel, and in a
   standalone launch the status bar overlaps it. */
.queue-heading {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-shrink: 0;
    box-sizing: border-box;
    padding: calc(0.5rem + env(safe-area-inset-top)) var(--app-content-gutter) 0.5rem;
    border-bottom: 1px solid var(--app-border);
}

.queue-heading-text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
}

/* h2, not h1: the page heading on this surface is the track on the player
   face, and the queue is a section of it. Sized like the scaffold's phone
   title. */
.queue-heading h2 {
    margin: 0;
    font-size: 1.2rem;
    font-weight: 700;
}

.queue-heading-summary {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.queue-heading-actions {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    flex-shrink: 0;
}

/* Same "pressed" convention as the queue pencil (QueueHeaderActions): a
   toggle that is on carries the soft accent fill. */
.queue-action-shuffle.is-active,
.queue-action-repeat.is-active {
    background: var(--app-accent-soft);
}

/* Menu rows in a column, same as the scaffold's overflow panel. The Popover
   teleports to body but keeps this component's scope attribute, so the scoped
   rule still reaches it. */
.queue-overflow-panel {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
}

.play-queue-list {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
    /* Bottom-most surface while the queue is up (no mini strip on screen), so
       the list reserves the home-indicator inset itself. */
    padding-bottom: env(safe-area-inset-bottom);
}

/* The list (and the QueueBody scroller inside it) CONTAINS its overscroll: a
   hard fling to the list's top must not chain into the page, and the
   deliberate queue → face pull is the sheet's own gesture (which
   preventDefaults once claimed), not native chaining. */
.play-queue-list,
.play-queue-list :deep(.queue-body) {
    overscroll-behavior-y: contain;
}
</style>
```

- [ ] **Step 4: Run test to verify it passes**

Run: `npx vitest run src/components/layout/__tests__/QueuePanel.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add webui/src/components/layout/QueuePanel.vue webui/src/components/layout/__tests__/QueuePanel.spec.ts
git commit -m "feat(webui): extract QueuePanel from MobilePlayView for the sheet"
```

---

### Task 5: NowPlayingSheet — structure, transforms, hash sync

**Files:**
- Create: `webui/src/components/layout/NowPlayingSheet.vue` (gestures arrive in Task 6; this task ships the sheet fully driven by the hash and its buttons)
- Test: `webui/src/components/layout/__tests__/NowPlayingSheet.spec.ts`
- Test: `webui/src/components/layout/__tests__/NowPlayingSheet.styles.spec.ts`

**Interfaces:**
- Consumes: Task 2's composable exports; Task 3 `PlayerFace` (emits `collapse`, `show-queue`); Task 4 `QueuePanel`; existing `MiniPlayer` — **note:** until Task 7 rewires it, `MiniPlayer` still navigates by itself; the sheet listens for its Task-7 `open` emit already (harmless now).
- Produces: `<NowPlayingSheet />`, no props. Root class `now-playing-sheet` with `.sheet-strip`, `.sheet-body`, `.sheet-track`, `.sheet-panel` structure. Task 6 adds handlers to these exact nodes; Task 7 mounts it from `MobileShell`.

- [ ] **Step 1: Write the failing component test**

```ts
// webui/src/components/layout/__tests__/NowPlayingSheet.spec.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive } from 'vue'
import {
    resetNowPlayingSheetForTests,
    useNowPlayingSheet
} from '@/composables/useNowPlayingSheet'

const push = vi.fn()
const replace = vi.fn()
const back = vi.fn()
const route = reactive({ path: '/library', fullPath: '/library', hash: '' })
const resolve = vi.fn(({ hash }: { hash: string }) => ({ fullPath: `/library${hash}` }))
vi.mock('vue-router', () => ({
    useRouter: () => ({ push, replace, back, resolve }),
    useRoute: () => route
}))

// The sheet is pure glue: strip/face/queue internals are covered by their own
// specs, so stub them down to the nodes the sheet wires.
vi.mock('@/components/layout/MiniPlayer.vue', () => ({
    default: {
        name: 'MiniPlayer',
        emits: ['open'],
        template: '<div class="stub-mini"><button class="stub-open" @click="$emit(\'open\')"></button></div>'
    }
}))
vi.mock('@/components/layout/PlayerFace.vue', () => ({
    default: {
        name: 'PlayerFace',
        emits: ['collapse', 'show-queue'],
        template:
            '<div class="stub-face"><div class="play-seek"></div>' +
            '<button class="stub-collapse" @click="$emit(\'collapse\')"></button>' +
            '<button class="stub-show-queue" @click="$emit(\'show-queue\')"></button></div>'
    }
}))
vi.mock('@/components/layout/QueuePanel.vue', () => ({
    default: {
        name: 'QueuePanel',
        template:
            '<div class="stub-queue"><header class="queue-heading"></header>' +
            '<div class="play-queue-list"><div class="stub-list"></div></div></div>'
    }
}))

import NowPlayingSheet from '@/components/layout/NowPlayingSheet.vue'

const mountSheet = () => mount(NowPlayingSheet)

const sheetTransform = (w: ReturnType<typeof mountSheet>): string =>
    (w.find('.now-playing-sheet').element as HTMLElement).style.transform
const trackTransform = (w: ReturnType<typeof mountSheet>): string =>
    (w.find('.sheet-track').element as HTMLElement).style.transform

beforeEach(() => {
    resetNowPlayingSheetForTests()
    route.hash = ''
    route.fullPath = '/library'
    window.history.replaceState({}, '', '/library')
    vi.clearAllMocks()
})

describe('NowPlayingSheet — transforms per detent', () => {
    it('rests collapsed: pushed down to its strip, track at the face', () => {
        const w = mountSheet()
        // 1 - expand fraction = 1 → the full (100% - strip) offset applies.
        expect(sheetTransform(w)).toContain('* 1)')
        // String(-0 * 50) is "0": at the face the track reads as an explicit 0.
        expect(trackTransform(w)).toBe('translateY(0%)')
    })

    it('playing: sheet fully up, track still on the face', async () => {
        const w = mountSheet()
        useNowPlayingSheet().snapTo('playing')
        await w.vm.$nextTick()
        expect(sheetTransform(w)).toContain('* 0)')
        expect(trackTransform(w)).toBe('translateY(0%)')
    })

    it('queue: sheet up and track shifted one panel', async () => {
        const w = mountSheet()
        useNowPlayingSheet().snapTo('queue')
        await w.vm.$nextTick()
        expect(sheetTransform(w)).toContain('* 0)')
        expect(trackTransform(w)).toBe('translateY(-50%)')
    })
})

describe('NowPlayingSheet — the hash is the source of truth', () => {
    it('mounts straight onto the detent the hash names', () => {
        route.hash = '#queue'
        const w = mountSheet()
        expect(useNowPlayingSheet().detent.value).toBe('queue')
        expect(trackTransform(w)).toBe('translateY(-50%)')
    })

    it('a hash change (back button, drawer-less nav) moves the sheet', async () => {
        const w = mountSheet()
        route.hash = '#playing'
        await w.vm.$nextTick()
        expect(useNowPlayingSheet().detent.value).toBe('playing')
        route.hash = ''
        await w.vm.$nextTick()
        expect(useNowPlayingSheet().detent.value).toBe('collapsed')
        expect(sheetTransform(w)).toContain('* 1)')
    })

    it('unmounting with a live sheet hash strips it — no stale #playing on desktop', async () => {
        const w = mountSheet()
        route.hash = '#playing'
        await w.vm.$nextTick()
        w.unmount()
        expect(replace).toHaveBeenCalledWith({ hash: '' })
        expect(useNowPlayingSheet().detent.value).toBe('collapsed')
    })

    it('unmounting collapsed leaves the route alone', () => {
        const w = mountSheet()
        w.unmount()
        expect(replace).not.toHaveBeenCalled()
    })
})

describe('NowPlayingSheet — buttons route through the hash', () => {
    it('the mini strip open request pushes #playing', async () => {
        const w = mountSheet()
        await w.find('.stub-open').trigger('click')
        expect(push).toHaveBeenCalledWith({ hash: '#playing' })
    })

    it('the face ⌃ pushes #queue and the ⌄ pops back to the page', async () => {
        route.hash = '#playing'
        const w = mountSheet()
        await w.find('.stub-show-queue').trigger('click')
        expect(push).toHaveBeenCalledWith({ hash: '#queue' })

        window.history.replaceState({ back: '/library' }, '', '/library#playing')
        await w.find('.stub-collapse').trigger('click')
        expect(back).toHaveBeenCalledOnce()
    })
})

describe('NowPlayingSheet — layered a11y', () => {
    // Vue may set `inert` as a DOM property (when jsdom exposes it) or as an
    // attribute; accept either so the assertion pins the intent, not the
    // serialization.
    const isInert = (w: ReturnType<typeof mountSheet>, selector: string): boolean => {
        const el = w.find(selector).element as HTMLElement & { inert?: boolean }
        return el.inert === true || el.hasAttribute('inert')
    }

    it('collapsed: the body is inert, the strip is live', () => {
        const w = mountSheet()
        expect(isInert(w, '.sheet-body')).toBe(true)
        expect(isInert(w, '.sheet-strip')).toBe(false)
        expect(w.find('.sheet-strip').classes()).not.toContain('strip-hidden')
    })

    it('expanded: the strip is inert and hidden, the body is live', async () => {
        const w = mountSheet()
        useNowPlayingSheet().snapTo('playing')
        await w.vm.$nextTick()
        expect(isInert(w, '.sheet-body')).toBe(false)
        expect(isInert(w, '.sheet-strip')).toBe(true)
        expect(w.find('.sheet-strip').classes()).toContain('strip-hidden')
    })
})
```

- [ ] **Step 2: Write the failing style-guard test**

```ts
// webui/src/components/layout/__tests__/NowPlayingSheet.styles.spec.ts
// @vitest-environment node
// Node env, not jsdom: this spec reads the component's <style> block off disk
// rather than rendering. Scoped SFC styles are never applied by vue-test-utils,
// so no mounted test can see them.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const styles = (() => {
    const source = readFileSync(
        fileURLToPath(new URL('../NowPlayingSheet.vue', import.meta.url)),
        'utf8'
    )
    const at = source.indexOf('<style')
    return source
        .slice(source.indexOf('>', at) + 1, source.lastIndexOf('</style>'))
        .replace(/\/\*[\s\S]*?\*\//g, '')
})()

/** Declaration bodies of every rule whose selector matches `pattern`. */
function ruleBodies(pattern: RegExp): string[] {
    const bodies: string[] = []
    for (const match of styles.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
        if (pattern.test(match[1])) bodies.push(match[2])
    }
    return bodies
}

describe('sheet overlay geometry', () => {
    const root = ruleBodies(/^\s*\.now-playing-sheet\s*$/m).join('\n')

    it('overlays the app shell absolutely, above content and below PrimeVue overlays', () => {
        expect(root).toMatch(/position:\s*absolute/)
        expect(root).toMatch(/inset:\s*0/)
        const z = root.match(/z-index:\s*(\d+)/)
        expect(z).toBeTruthy()
        expect(Number(z![1])).toBeLessThan(1000)
    })

    it('contains its overscroll so a drag never chains into pull-to-refresh', () => {
        expect(root).toMatch(/overscroll-behavior:\s*contain/)
    })
})

describe('player-bar palette on the sheet root', () => {
    const root = ruleBodies(/^\s*\.now-playing-sheet\s*$/m).join('\n')

    it('paints the sheet with the player surface and text', () => {
        expect(root).toMatch(/background-color:\s*var\(--app-player-bg\)/)
        expect(root).toMatch(/color:\s*var\(--app-player-text\)/)
    })

    it('remaps the app tokens the shared children colour themselves with', () => {
        expect(root).toMatch(/--app-text-primary:\s*var\(--app-player-text\)/)
        expect(root).toMatch(/--app-text-secondary:\s*var\(--app-player-dim\)/)
        expect(root).toMatch(/--app-hover:/)
        expect(root).toMatch(/--app-border:/)
        expect(root).toMatch(/--app-accent-soft:/)
    })

    it('leaves the accent alone — it is the "this is playing" signal', () => {
        expect(root).not.toMatch(/--app-accent:\s/)
    })
})

describe('motion is CSS-owned and finger-gated', () => {
    it('animates the sheet and track transforms', () => {
        const root = ruleBodies(/^\s*\.now-playing-sheet\s*$/m).join('\n')
        const track = ruleBodies(/\.sheet-track\s*$/m).join('\n')
        expect(root).toMatch(/transition:[^;]*transform/)
        expect(track).toMatch(/transition:[^;]*transform/)
    })

    it('turns every transition off while a finger owns the motion', () => {
        const dragging = ruleBodies(/\.is-dragging/).join('\n')
        expect(dragging).toMatch(/transition:\s*none/)
    })

    it('honours prefers-reduced-motion without any JS', () => {
        expect(styles).toMatch(/@media \(prefers-reduced-motion: reduce\)/)
        const reduced = styles.match(/@media \(prefers-reduced-motion: reduce\)[\s\S]*?\n\}/)?.[0]
        expect(reduced).toMatch(/transition:\s*none/)
    })
})
```

- [ ] **Step 3: Run both to verify they fail**

Run: `npx vitest run src/components/layout/__tests__/NowPlayingSheet.spec.ts src/components/layout/__tests__/NowPlayingSheet.styles.spec.ts`
Expected: FAIL — cannot resolve `../NowPlayingSheet.vue`.

- [ ] **Step 4: Create the component (route-driven; no touch handlers yet)**

```vue
<!-- webui/src/components/layout/NowPlayingSheet.vue -->
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import MiniPlayer from '@/components/layout/MiniPlayer.vue'
import PlayerFace from '@/components/layout/PlayerFace.vue'
import QueuePanel from '@/components/layout/QueuePanel.vue'
import {
    commitDetent,
    detentForHash,
    useNowPlayingSheet,
    type SheetDetent
} from '@/composables/useNowPlayingSheet'

// The phone's Now Playing, as one always-mounted bottom sheet over the route
// content (rendered by MobileShell; spec:
// docs/superpowers/specs/2026-08-16-mobile-sheet-navigation-design.md).
// Three detents — collapsed (mini strip) / playing (face) / queue — addressed
// by the hash of WHATEVER route is underneath: '' / #playing / #queue. The
// hash is the single source of truth: buttons and gestures commit it
// (commitDetent), this component's watcher animates toward it, and system
// back therefore walks queue → playing → page with no popstate machinery.
// The content view never unmounts, so a dismiss drag reveals the live page.
const router = useRouter()
const route = useRoute()
const { detent, position, dragging, snapTo } = useNowPlayingSheet()

const root = ref<HTMLElement | null>(null)
const strip = ref<HTMLElement | null>(null)

// --- transforms -----------------------------------------------------------
// position 0..1 raises the sheet from its strip to full height; 1..2 shifts
// the two-panel track from the face to the queue. The strip height stays in
// CSS (calc against the same tokens MiniPlayer sizes itself with), so no
// resize observer is needed for rendering — JS only measures for gestures.
const expandF = computed(() => Math.min(Math.max(position.value, 0), 1))
const queueF = computed(() => Math.min(Math.max(position.value - 1, 0), 1))

const sheetStyle = computed(() => ({
    transform: `translateY(calc((100% - (var(--app-mini-player-height) + env(safe-area-inset-bottom))) * ${1 - expandF.value}))`
}))
const trackStyle = computed(() => ({
    transform: `translateY(${-queueF.value * 50}%)`
}))
// The strip cross-fades out over the first half of the rise, while the face
// underneath is already lit — the bar melts into the player it becomes.
const stripStyle = computed(() => ({
    opacity: String(Math.max(0, 1 - expandF.value * 2))
}))
const stripHidden = computed(() => expandF.value >= 0.5)

// --- route sync ------------------------------------------------------------
// Buttons (mini tap, face ⌄/⌃) go THROUGH the hash: commitDetent navigates,
// the watcher below is the one place that moves the sheet. Back button and
// deep links arrive through the same watcher for free.
const requestDetent = (to: SheetDetent): void => {
    commitDetent(router, detent.value, to)
}

watch(
    () => route.hash,
    (hash) => {
        const target = detentForHash(hash)
        if (target !== detent.value) snapTo(target)
    }
)

// Transitions are suppressed until after the first paint, so arriving
// addressed to a detent (reload, shared link) lands there with no animation.
const ready = ref(false)
onMounted(() => {
    snapTo(detentForHash(route.hash))
    requestAnimationFrame(() => {
        ready.value = true
    })
})

// Unmounting (queue emptied, shell flipped to desktop) with a live sheet hash
// would leave a stale #playing on a chrome that has no sheet: strip it.
onUnmounted(() => {
    if (detentForHash(route.hash) !== 'collapsed') void router.replace({ hash: '' })
    snapTo('collapsed')
})
</script>

<template>
    <div
        ref="root"
        class="now-playing-sheet"
        :class="{ 'is-dragging': dragging || !ready }"
        :style="sheetStyle"
    >
        <!-- Collapsed, everything above the strip is off-screen; inert keeps
             it out of the tab order and the AT tree without unmounting it. -->
        <div class="sheet-body" :inert="detent === 'collapsed'">
            <div class="sheet-track" :style="trackStyle">
                <section class="sheet-panel sheet-panel-face">
                    <PlayerFace
                        @collapse="requestDetent('collapsed')"
                        @show-queue="requestDetent('queue')"
                    />
                </section>
                <section class="sheet-panel sheet-panel-queue">
                    <QueuePanel />
                </section>
            </div>
        </div>
        <!-- After the body in the DOM so it paints above the face's top edge
             while collapsed; inert + pointer-events off once it has faded. -->
        <div
            ref="strip"
            class="sheet-strip"
            :class="{ 'strip-hidden': stripHidden }"
            :style="stripStyle"
            :inert="stripHidden"
        >
            <MiniPlayer @open="requestDetent('playing')" />
        </div>
    </div>
</template>

<style scoped>
.now-playing-sheet {
    /* Absolute against .player-shell (PlayerLayout gives it position:
       relative), NOT position: fixed: the shell already owns the one
       dvh-measured box (appShell.spec.ts), and a fixed box would re-measure
       the viewport on its own terms while the URL bar moves. Below 1000 so
       every PrimeVue overlay (dialogs, popovers, toasts) stays on top. */
    position: absolute;
    inset: 0;
    z-index: 100;
    overflow: hidden;
    /* A drag at the sheet's edge must never chain into the page: on Android
       the viewport is next in line and would pull-to-refresh mid-gesture. */
    overscroll-behavior: contain;
    /* Now Playing keeps the player-bar palette (the dark blue surface) in
       BOTH themes — the transport belongs to the player chrome, not the
       page. Everything inside (queue heading, queue rows, transport) colours
       itself with the app tokens, so remap those for the subtree rather than
       forking the children; custom properties inherit through the DOM, so
       this reaches their scoped rules.
       --app-accent is deliberately NOT remapped: it is the "this is playing"
       signal and already clears 5.2:1 on the player background in both
       themes. */
    background-color: var(--app-player-bg);
    color: var(--app-player-text);
    --app-text-primary: var(--app-player-text);
    --app-text-secondary: var(--app-player-dim);
    --app-hover: color-mix(in srgb, var(--app-player-text) 12%, transparent);
    --app-border: color-mix(in srgb, var(--app-player-text) 20%, transparent);
    /* The light-theme soft accent is mixed for a white surface and all but
       vanishes here; strengthen it so the now-playing strip stays readable. */
    --app-accent-soft: color-mix(in srgb, var(--app-accent) 20%, transparent);
    /* A release animates the transform; a finger owning it does not
       (.is-dragging below). The curve is scroll-snap's shape: quick to
       leave, settling at the end. */
    transition: transform 0.28s cubic-bezier(0.32, 0.72, 0, 1);
}

.sheet-body {
    position: absolute;
    inset: 0;
    overflow: hidden;
}

/* Two full-screen panels stacked in a 200%-tall track; shifting the track by
   half its own height (-50%) is exactly one panel. */
.sheet-track {
    height: 200%;
    display: flex;
    flex-direction: column;
    transition: transform 0.28s cubic-bezier(0.32, 0.72, 0, 1);
}

.sheet-panel {
    height: 50%;
    min-height: 0;
    display: flex;
    flex-direction: column;
}

.sheet-strip {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    z-index: 2;
    transition: opacity 0.15s linear;
}

.sheet-strip.strip-hidden {
    pointer-events: none;
}

/* The finger owns the motion: everything snaps to where it is put, frame for
   frame. Also covers the pre-first-paint mount so a reload lands with no
   entrance animation. */
.now-playing-sheet.is-dragging,
.now-playing-sheet.is-dragging .sheet-track,
.now-playing-sheet.is-dragging .sheet-strip {
    transition: none;
}

.now-playing-sheet.is-dragging {
    will-change: transform;
}

/* Nothing waits on transitionend, so reduced motion is pure CSS: the sheet
   simply jumps between detents. */
@media (prefers-reduced-motion: reduce) {
    .now-playing-sheet,
    .now-playing-sheet .sheet-track,
    .now-playing-sheet .sheet-strip {
        transition: none;
    }
}
</style>
```

- [ ] **Step 5: Run the two specs to verify they pass**

Run: `npx vitest run src/components/layout/__tests__/NowPlayingSheet.spec.ts src/components/layout/__tests__/NowPlayingSheet.styles.spec.ts`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add webui/src/components/layout/NowPlayingSheet.vue webui/src/components/layout/__tests__/NowPlayingSheet.spec.ts webui/src/components/layout/__tests__/NowPlayingSheet.styles.spec.ts
git commit -m "feat(webui): hash-addressed now-playing sheet with detent transforms"
```

---

### Task 6: NowPlayingSheet — gestures

**Files:**
- Modify: `webui/src/components/layout/NowPlayingSheet.vue`
- Test: `webui/src/components/layout/__tests__/NowPlayingSheet.gestures.spec.ts`

**Interfaces:**
- Consumes: Task 1's `dragPosition`, `settleDetent`, `SLOP_PX`, `VelocityTracker`; Task 5's DOM structure.
- Produces: touch handling on `.sheet-strip` (lift, range [0,1]), `.sheet-panel-face` (both directions, range [0,2], `.play-seek` exempt), `.sheet-panel-queue` (pull-down from list top OR from `.queue-heading` at any scroll, range [1,2], `preventDefault` once claimed), plus capture-phase click swallowing after a claimed drag.

- [ ] **Step 1: Write the failing test**

```ts
// webui/src/components/layout/__tests__/NowPlayingSheet.gestures.spec.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive } from 'vue'
import {
    resetNowPlayingSheetForTests,
    useNowPlayingSheet
} from '@/composables/useNowPlayingSheet'

const push = vi.fn()
const replace = vi.fn()
const back = vi.fn()
const route = reactive({ path: '/library', fullPath: '/library', hash: '' })
const resolve = vi.fn(({ hash }: { hash: string }) => ({ fullPath: `/library${hash}` }))
vi.mock('vue-router', () => ({
    useRouter: () => ({ push, replace, back, resolve }),
    useRoute: () => route
}))

// Velocity depends on real event timestamps, which jsdom makes effectively
// random — a fast test run reads as a flick. Position decides here; flick
// behavior is covered by sheetGesture's own unit spec.
vi.mock('@/lib/sheetGesture', async (importOriginal) => {
    const actual = await importOriginal<typeof import('@/lib/sheetGesture')>()
    return { ...actual, settleDetent: (p: number, _v: number, min: number, max: number) =>
        Math.min(Math.max(Math.round(p), min), max) }
})

vi.mock('@/components/layout/MiniPlayer.vue', () => ({
    default: {
        name: 'MiniPlayer',
        emits: ['open'],
        template: '<div class="stub-mini"><button class="stub-open" @click="$emit(\'open\')"></button></div>'
    }
}))
vi.mock('@/components/layout/PlayerFace.vue', () => ({
    default: {
        name: 'PlayerFace',
        emits: ['collapse', 'show-queue'],
        template: '<div class="stub-face"><div class="play-seek"></div></div>'
    }
}))
vi.mock('@/components/layout/QueuePanel.vue', () => ({
    default: {
        name: 'QueuePanel',
        template:
            '<div class="stub-queue"><header class="queue-heading"></header>' +
            '<div class="play-queue-list"><div class="stub-list"></div></div></div>'
    }
}))

import NowPlayingSheet from '@/components/layout/NowPlayingSheet.vue'

const H = 800
const STRIP = 60

const mountSheet = () => {
    const w = mount(NowPlayingSheet)
    Object.defineProperty(w.find('.now-playing-sheet').element, 'offsetHeight', {
        value: H,
        configurable: true
    })
    Object.defineProperty(w.find('.sheet-strip').element, 'offsetHeight', {
        value: STRIP,
        configurable: true
    })
    return w
}

const touch = async (
    w: ReturnType<typeof mountSheet>,
    selector: string,
    kind: 'touchstart' | 'touchmove',
    y: number,
    x = 0
) => {
    await w.find(selector).trigger(kind, { touches: [{ clientY: y, clientX: x }] })
}
const release = async (w: ReturnType<typeof mountSheet>, selector: string) => {
    await w.find(selector).trigger('touchend')
}

const sheet = () => useNowPlayingSheet()

beforeEach(() => {
    resetNowPlayingSheetForTests()
    route.hash = ''
    route.fullPath = '/library'
    window.history.replaceState({}, '', '/library')
    vi.clearAllMocks()
})

describe('strip lift (collapsed → playing)', () => {
    it('follows the finger with the transition off, and nothing is decided mid-drag', async () => {
        const w = mountSheet()
        await touch(w, '.sheet-strip', 'touchstart', 700)
        await touch(w, '.sheet-strip', 'touchmove', 330) // 370px = half of 740 travel
        expect(sheet().position.value).toBeCloseTo(0.5, 5)
        expect(sheet().dragging.value).toBe(true)
        expect(w.find('.now-playing-sheet').classes()).toContain('is-dragging')
        expect(push).not.toHaveBeenCalled()
    })

    it('released past the midpoint it settles open and pushes #playing', async () => {
        const w = mountSheet()
        await touch(w, '.sheet-strip', 'touchstart', 700)
        await touch(w, '.sheet-strip', 'touchmove', 200)
        await release(w, '.sheet-strip')
        expect(sheet().detent.value).toBe('playing')
        expect(sheet().position.value).toBe(1)
        expect(sheet().dragging.value).toBe(false)
        expect(push).toHaveBeenCalledWith({ hash: '#playing' })
    })

    it('released short it springs back and navigates nowhere', async () => {
        const w = mountSheet()
        await touch(w, '.sheet-strip', 'touchstart', 700)
        await touch(w, '.sheet-strip', 'touchmove', 600)
        await release(w, '.sheet-strip')
        expect(sheet().position.value).toBe(0)
        expect(push).not.toHaveBeenCalled()
    })

    it('ignores movement inside the slop — a wobbly tap is still a tap', async () => {
        const w = mountSheet()
        await touch(w, '.sheet-strip', 'touchstart', 700)
        await touch(w, '.sheet-strip', 'touchmove', 695)
        expect(sheet().dragging.value).toBe(false)
        expect(sheet().position.value).toBe(0)
    })

    it('a downward drag on the strip claims nothing — the bar only goes one way', async () => {
        const w = mountSheet()
        await touch(w, '.sheet-strip', 'touchstart', 700)
        await touch(w, '.sheet-strip', 'touchmove', 760)
        expect(sheet().dragging.value).toBe(false)
        expect(sheet().position.value).toBe(0)
    })

    it('swallows the click the browser delivers after a claimed drag', async () => {
        const w = mountSheet()
        await touch(w, '.sheet-strip', 'touchstart', 700)
        await touch(w, '.sheet-strip', 'touchmove', 600)
        await release(w, '.sheet-strip')
        await w.find('.stub-open').trigger('click')
        // The drag's release-click must not ALSO open the sheet.
        expect(push).not.toHaveBeenCalled()
        // Only once: the next real tap works again.
        await w.find('.stub-open').trigger('click')
        expect(push).toHaveBeenCalledWith({ hash: '#playing' })
    })
})

describe('face drags (playing → collapsed / queue)', () => {
    const mountAtPlaying = () => {
        route.hash = '#playing'
        return mountSheet()
    }

    it('dragging down follows the finger toward collapsed', async () => {
        const w = mountAtPlaying()
        await touch(w, '.sheet-panel-face', 'touchstart', 100)
        await touch(w, '.sheet-panel-face', 'touchmove', 470) // 370 = half of 740
        expect(sheet().position.value).toBeCloseTo(0.5, 5)
    })

    it('released low it collapses — back() when the page entry is right below', async () => {
        window.history.replaceState({ back: '/library' }, '', '/library#playing')
        const w = mountAtPlaying()
        await touch(w, '.sheet-panel-face', 'touchstart', 100)
        await touch(w, '.sheet-panel-face', 'touchmove', 700)
        await release(w, '.sheet-panel-face')
        expect(sheet().detent.value).toBe('collapsed')
        expect(back).toHaveBeenCalledOnce()
        expect(replace).not.toHaveBeenCalled()
    })

    it('released low on a deep link it rewrites in place instead of leaving the app', async () => {
        window.history.replaceState({ back: null }, '', '/library#playing')
        const w = mountAtPlaying()
        await touch(w, '.sheet-panel-face', 'touchstart', 100)
        await touch(w, '.sheet-panel-face', 'touchmove', 700)
        await release(w, '.sheet-panel-face')
        expect(replace).toHaveBeenCalledWith({ hash: '' })
        expect(back).not.toHaveBeenCalled()
    })

    it('dragging up reveals the queue and pushes #queue on release', async () => {
        const w = mountAtPlaying()
        await touch(w, '.sheet-panel-face', 'touchstart', 700)
        await touch(w, '.sheet-panel-face', 'touchmove', 100) // 600 of 800 queue travel
        expect(sheet().position.value).toBeCloseTo(1.75, 5)
        await release(w, '.sheet-panel-face')
        expect(sheet().detent.value).toBe('queue')
        expect(push).toHaveBeenCalledWith({ hash: '#queue' })
    })

    it('a drag that starts on the seek bar never claims — off-axis seeking stays a seek', async () => {
        const w = mountAtPlaying()
        await touch(w, '.play-seek', 'touchstart', 100)
        await touch(w, '.play-seek', 'touchmove', 700)
        expect(sheet().dragging.value).toBe(false)
        expect(sheet().position.value).toBe(1)
    })

    it('a horizontal-dominant move never claims', async () => {
        const w = mountAtPlaying()
        await touch(w, '.sheet-panel-face', 'touchstart', 100, 0)
        await touch(w, '.sheet-panel-face', 'touchmove', 130, 200)
        expect(sheet().dragging.value).toBe(false)
    })
})

describe('queue drags (queue → playing)', () => {
    const mountAtQueue = () => {
        route.hash = '#queue'
        return mountSheet()
    }

    it('a pull that starts with the list at its top follows the finger back to the face', async () => {
        window.history.replaceState({ back: '/library#playing' }, '', '/library#queue')
        const w = mountAtQueue()
        await touch(w, '.play-queue-list', 'touchstart', 100)
        await touch(w, '.play-queue-list', 'touchmove', 500) // 400 of 800
        expect(sheet().position.value).toBeCloseTo(1.5, 5)
        await touch(w, '.play-queue-list', 'touchmove', 700)
        await release(w, '.play-queue-list')
        expect(sheet().detent.value).toBe('playing')
        expect(back).toHaveBeenCalledOnce()
    })

    it('does not arm while the list is scrolled down — that pull scrolls the list', async () => {
        const w = mountAtQueue()
        const list = w.find('.stub-list')
        ;(list.element as HTMLElement).scrollTop = 50
        await touch(w, '.stub-list', 'touchstart', 100)
        await touch(w, '.stub-list', 'touchmove', 700)
        expect(sheet().dragging.value).toBe(false)
        expect(sheet().position.value).toBe(2)
    })

    it('dragging the heading works at any list position', async () => {
        window.history.replaceState({ back: '/library#playing' }, '', '/library#queue')
        const w = mountAtQueue()
        const list = w.find('.stub-list')
        ;(list.element as HTMLElement).scrollTop = 300
        await touch(w, '.queue-heading', 'touchstart', 100)
        await touch(w, '.queue-heading', 'touchmove', 600)
        expect(sheet().position.value).toBeCloseTo(1.375, 5)
        await release(w, '.queue-heading')
        expect(sheet().detent.value).toBe('playing')
        expect(back).toHaveBeenCalledOnce()
    })

    it('the queue surface never travels below the face', async () => {
        const w = mountAtQueue()
        await touch(w, '.queue-heading', 'touchstart', 0)
        await touch(w, '.queue-heading', 'touchmove', 4000)
        expect(sheet().position.value).toBe(1)
    })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `npx vitest run src/components/layout/__tests__/NowPlayingSheet.gestures.spec.ts`
Expected: FAIL — drags do nothing (`position` never moves), no handlers exist yet.

- [ ] **Step 3: Add the gesture code to `NowPlayingSheet.vue`**

Add to the `<script setup>` block (below the transforms, above the route sync):

```ts
import {
    dragPosition,
    settleDetent,
    SLOP_PX,
    VelocityTracker
} from '@/lib/sheetGesture'
import { DETENTS } from '@/composables/useNowPlayingSheet'
// (merge these into the existing import statements)

// --- gestures ---------------------------------------------------------------
// One drag controller for four surfaces; each surface arms a direction and a
// range, and every claimed drag maps the finger 1:1 through sheetGesture's
// travel model. A claim needs SLOP_PX of dominant-vertical movement, so taps,
// the transport buttons and the seek slider never notice a wobble.
type DragSurface = 'strip' | 'face' | 'queue' | 'heading'
const RANGES: Record<DragSurface, [number, number]> = {
    strip: [0, 1],
    face: [0, 2],
    queue: [1, 2],
    heading: [1, 2]
}

interface ActiveDrag {
    surface: DragSurface
    startY: number
    startX: number
    startPos: number
    min: number
    max: number
    claimed: boolean
    denied: boolean
    viewportH: number
    stripH: number
    tracker: VelocityTracker
}

let drag: ActiveDrag | null = null
// A drag and a tap are the same touch until the finger moves: once it has,
// the click the browser still delivers on release is not a tap any more.
let swallowClick = false

const queuePanelEl = ref<HTMLElement | null>(null)

// True when anything between the touch target and the queue panel has been
// scrolled: that pull belongs to the list, not to the sheet.
const queueScrolledDown = (target: EventTarget | null): boolean => {
    let el = target instanceof HTMLElement ? target : null
    while (el && el !== queuePanelEl.value) {
        if (el.scrollTop > 0) return true
        el = el.parentElement
    }
    return false
}

const beginDrag = (surface: DragSurface, event: TouchEvent): void => {
    const touch = event.touches[0]
    if (!touch) return
    const denied =
        (surface === 'face' &&
            event.target instanceof Element &&
            !!event.target.closest('.play-seek')) ||
        (surface === 'queue' && queueScrolledDown(event.target))
    const [min, max] = RANGES[surface]
    const tracker = new VelocityTracker()
    tracker.push(touch.clientY, event.timeStamp)
    drag = {
        surface,
        startY: touch.clientY,
        startX: touch.clientX,
        startPos: position.value,
        min,
        max,
        claimed: false,
        denied,
        viewportH: 1,
        stripH: 0,
        tracker
    }
}

const onStripTouchStart = (event: TouchEvent): void => beginDrag('strip', event)
const onFaceTouchStart = (event: TouchEvent): void => beginDrag('face', event)
// The heading is the queue's escape hatch AT ANY list position (the list pull
// only arms at its top); same range, different arming.
const onQueueTouchStart = (event: TouchEvent): void => {
    const onHeading =
        event.target instanceof Element && !!event.target.closest('.queue-heading')
    beginDrag(onHeading ? 'heading' : 'queue', event)
}

const moveDrag = (event: TouchEvent): void => {
    const active = drag
    if (!active || active.denied) return
    const touch = event.touches[0]
    if (!touch) return
    const dy = touch.clientY - active.startY
    if (!active.claimed) {
        const dx = touch.clientX - active.startX
        if (Math.abs(dy) <= SLOP_PX || Math.abs(dy) <= Math.abs(dx)) return
        // Direction gates: the strip only lifts, the queue surfaces only pull
        // down (up is the list's business); the face goes both ways.
        const gate =
            active.surface === 'strip' ? dy < 0 : active.surface === 'face' ? true : dy > 0
        if (!gate) {
            active.denied = true
            return
        }
        active.claimed = true
        // Measured at claim time, not per frame: the drag needs one stable
        // travel mapping, and mid-gesture URL-bar movement would warp it.
        active.viewportH = root.value?.offsetHeight || 1
        active.stripH = strip.value?.offsetHeight ?? 0
        dragging.value = true
        swallowClick = true
    }
    // Once claimed, the queue surfaces own the touch outright: without
    // preventDefault the list (at its top) still elastic-scrolls against the
    // drag on iOS. Their touchmove is bound WITHOUT .passive for exactly this.
    if (event.cancelable && (active.surface === 'queue' || active.surface === 'heading')) {
        event.preventDefault()
    }
    active.tracker.push(touch.clientY, event.timeStamp)
    position.value = dragPosition(
        active.startPos,
        dy,
        active.viewportH,
        active.stripH,
        active.min,
        active.max
    )
}

const endDrag = (): void => {
    const active = drag
    drag = null
    if (!active?.claimed) return
    dragging.value = false
    settleTo(settleDetent(position.value, active.tracker.velocity(), active.min, active.max))
}

// Settle: the position is already at the target, so set the detent FIRST and
// let commitDetent navigate — the hash watcher then sees hash and detent
// agree and does nothing.
const settleTo = (index: number): void => {
    const to = DETENTS[index]
    const from = detent.value
    position.value = index
    detent.value = to
    commitDetent(router, from, to)
}

const onRootClickCapture = (event: MouseEvent): void => {
    if (!swallowClick) return
    swallowClick = false
    event.preventDefault()
    event.stopPropagation()
}
```

And change the template's three interactive nodes to (structure and other attributes unchanged):

```vue
    <div
        ref="root"
        class="now-playing-sheet"
        :class="{ 'is-dragging': dragging || !ready }"
        :style="sheetStyle"
        @click.capture="onRootClickCapture"
    >
        <div class="sheet-body" :inert="detent === 'collapsed'">
            <div class="sheet-track" :style="trackStyle">
                <section
                    class="sheet-panel sheet-panel-face"
                    @touchstart.passive="onFaceTouchStart"
                    @touchmove.passive="moveDrag"
                    @touchend.passive="endDrag"
                    @touchcancel.passive="endDrag"
                >
                    <PlayerFace
                        @collapse="requestDetent('collapsed')"
                        @show-queue="requestDetent('queue')"
                    />
                </section>
                <!-- touchmove deliberately NOT passive here: a claimed pull
                     preventDefaults so the list cannot fight the drag. -->
                <section
                    ref="queuePanelEl"
                    class="sheet-panel sheet-panel-queue"
                    @touchstart.passive="onQueueTouchStart"
                    @touchmove="moveDrag"
                    @touchend.passive="endDrag"
                    @touchcancel.passive="endDrag"
                >
                    <QueuePanel />
                </section>
            </div>
        </div>
        <div
            ref="strip"
            class="sheet-strip"
            :class="{ 'strip-hidden': stripHidden }"
            :style="stripStyle"
            :inert="stripHidden"
            @touchstart.passive="onStripTouchStart"
            @touchmove.passive="moveDrag"
            @touchend.passive="endDrag"
            @touchcancel.passive="endDrag"
        >
            <MiniPlayer @open="requestDetent('playing')" />
        </div>
    </div>
```

- [ ] **Step 4: Run the gesture spec and the Task 5 specs to verify all pass**

Run: `npx vitest run src/components/layout/__tests__/NowPlayingSheet.gestures.spec.ts src/components/layout/__tests__/NowPlayingSheet.spec.ts src/components/layout/__tests__/NowPlayingSheet.styles.spec.ts`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add webui/src/components/layout/NowPlayingSheet.vue webui/src/components/layout/__tests__/NowPlayingSheet.gestures.spec.ts
git commit -m "feat(webui): finger-following drags across all four sheet surfaces"
```

---

### Task 7: Integration — shell, mini player, home alias, inert; delete MobilePlayView

**Files:**
- Modify: `webui/src/layouts/MobileShell.vue`
- Modify: `webui/src/layouts/PlayerLayout.vue`
- Modify: `webui/src/components/layout/MiniPlayer.vue`
- Modify: `webui/src/views/HomeView.vue`
- Modify: `webui/src/components/layout/__tests__/MiniPlayer.spec.ts` (rewrite)
- Modify: `webui/src/views/__tests__/HomeView.spec.ts`
- Modify: `webui/src/components/layout/__tests__/mobile-chrome.safeArea.spec.ts`
- Delete: `webui/src/components/layout/MobilePlayView.vue`
- Delete: `webui/src/components/layout/__tests__/MobilePlayView.spec.ts`
- Delete: `webui/src/components/layout/__tests__/MobilePlayView.layoutStyles.spec.ts`
- Delete: `webui/src/components/layout/__tests__/MobilePlayView.paletteStyles.spec.ts`
- Delete: `webui/src/lib/motion.ts` (its two consumers — MobilePlayView and MiniPlayer's lift — are gone; verify with `grep -rn "lib/motion" webui/src` first, keep it if anything else took a dependency meanwhile)

**Interfaces:**
- Consumes: Task 5/6 `<NowPlayingSheet />`; Task 2 `useNowPlayingSheet().open`.
- Produces: `MiniPlayer` emits `open` instead of navigating (`defineEmits<{ (e: 'open'): void }>()`), all drag/lift/tap-swallow code removed; mobile `/` is a pure alias.

- [ ] **Step 1: Rewrite `MiniPlayer.vue` as the dumb bar**

Remove: `useRouter` import and usage, `prefersReducedMotion` import, all of `DRAG_COMMIT_PX`/`DRAG_RESIST`/`LEAVE_MS`/`LEAVE_CLEARANCE_PX`, `bar`/`dragY`/`dragging`/`leaving` refs, `startY`/`armed`/`tapSwallowed`/`leaveTimer`, `finishLeave`/`liftAway`/`onTransitionEnd`/`onTouchStart`/`onTouchMove`/`onTouchEnd`/`swallowedTap`, `onUnmounted`; the root's touch/transitionend bindings, `:style` transform, `is-dragging`/`is-leaving` classes; the `transition`, `.is-dragging`, `.is-leaving`, and `::after` style rules. The sheet owns the lift gesture and the click swallowing now.

The script becomes:

```ts
import { computed } from 'vue'
import { usePlayer } from '@/composables/usePlayer'
import { subsonicClient } from '@/lib/api/subsonic'

// The Now Playing sheet's collapsed strip (NowPlayingSheet.vue). Dumb on
// purpose: the sheet owns the lift gesture, the strip cross-fade and the
// click-after-drag swallowing — this bar only renders the track and emits
// `open` for a tap. It never navigates itself, so it works no matter which
// route sits under the sheet.
const emit = defineEmits<{ (e: 'open'): void }>()

const player = usePlayer()

const currentTrack = computed(() => player.currentTrack.value)

const coverUrl = computed(() => {
    const art = currentTrack.value?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 96)
})

const progressPercent = computed(() => {
    if (!player.duration.value) return 0
    return (player.currentTime.value / player.duration.value) * 100
})
```

Template: root `<div class="mini-player">` with no gesture bindings; the open button becomes `@click="emit('open')"`; the play/pause and next buttons call `player.togglePlayPause()` / `player.playNext()` directly. Everything else (progress hairline, cover, meta, structure comments) stays.

- [ ] **Step 2: Rewrite `MiniPlayer.spec.ts` for the reduced scope**

Replace the whole file with:

```ts
// webui/src/components/layout/__tests__/MiniPlayer.spec.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import MiniPlayer from '../MiniPlayer.vue'

const togglePlayPause = vi.fn()
const playNext = vi.fn()
const isPlaying = ref(false)
const currentTrack = ref<{ title: string; artist: string; coverArt?: string } | null>({
    title: 'Karma Police',
    artist: 'Radiohead',
    coverArt: 'cov-1'
})

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        currentTrack,
        isPlaying,
        currentTime: ref(30),
        duration: ref(120),
        togglePlayPause,
        playNext
    })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size: number) => `/art/${id}?size=${size}`
    }
}))

beforeEach(() => {
    togglePlayPause.mockClear()
    playNext.mockClear()
    isPlaying.value = false
})

// The bar is the sheet's collapsed strip: gestures, navigation and
// click-swallowing all live in NowPlayingSheet — this bar only renders and
// emits `open` for a tap.
describe('MiniPlayer', () => {
    it('shows title, artist and cover of the current track', () => {
        const mp = mount(MiniPlayer)
        expect(mp.text()).toContain('Karma Police')
        expect(mp.text()).toContain('Radiohead')
        expect(mp.find('img.mini-cover').attributes('src')).toBe('/art/cov-1?size=96')
    })

    it('play button toggles playback without emitting open', async () => {
        const mp = mount(MiniPlayer)
        await mp.find('[aria-label="Play"]').trigger('click')
        expect(togglePlayPause).toHaveBeenCalledOnce()
        expect(mp.emitted('open')).toBeUndefined()
    })

    it('shows Pause while playing', () => {
        isPlaying.value = true
        expect(mount(MiniPlayer).find('[aria-label="Pause"]').exists()).toBe(true)
    })

    it('next button skips without emitting open', async () => {
        const mp = mount(MiniPlayer)
        await mp.find('[aria-label="Next track"]').trigger('click')
        expect(playNext).toHaveBeenCalledOnce()
        expect(mp.emitted('open')).toBeUndefined()
    })

    it('tapping the bar emits open — the sheet decides what that means', async () => {
        const mp = mount(MiniPlayer)
        await mp.find('[aria-label="Open Now Playing"]').trigger('click')
        expect(mp.emitted('open')).toHaveLength(1)
    })

    // The open target is a SIBLING under the transport, not a role="button"
    // wrapper around it: nested, Enter/Space on Pause bubbled into the wrapper
    // and navigated (Space never pausing at all under .prevent).
    it('keyboard-activating the transport never emits open', async () => {
        const mp = mount(MiniPlayer)
        const pause = mp.find('[aria-label="Play"]')
        await pause.trigger('keydown.enter')
        await pause.trigger('keydown.space')
        expect(mp.emitted('open')).toBeUndefined()
        expect(mp.find('.mini-player').attributes('role')).toBeUndefined()
    })

    it('renders the progress hairline from currentTime/duration', () => {
        const mp = mount(MiniPlayer)
        expect(mp.find('.mini-progress-fill').attributes('style')).toContain('width: 25%')
    })
})
```

- [ ] **Step 3: Swap the shell chrome**

Replace `webui/src/layouts/MobileShell.vue` with:

```vue
<script setup lang="ts">
import NowPlayingSheet from '@/components/layout/NowPlayingSheet.vue'
import { usePlayer } from '@/composables/usePlayer'

// Mobile-only docked chrome; PlayerLayout owns the shared skeleton (route
// outlet included) so a shell swap never unmounts the active view. Rendered
// as a fragment so the spacer stays a direct flex child of the shell column.
//
// The sheet is all of it: Now Playing, the queue and the mini strip are one
// always-mounted overlay (NowPlayingSheet, addressed by the route hash), and
// it overlays rather than docks — so the spacer below reserves the strip's
// height in the flex column, keeping list bottoms clear of the bar.
const { queue } = usePlayer()
</script>

<template>
    <div v-if="queue.length > 0" class="mini-spacer" aria-hidden="true"></div>
    <NowPlayingSheet v-if="queue.length > 0" />
</template>

<style scoped>
.mini-spacer {
    height: calc(var(--app-mini-player-height) + env(safe-area-inset-bottom));
    flex-shrink: 0;
}
</style>
```

- [ ] **Step 4: Give the sheet its containing block and the covered content an inert gate**

In `webui/src/layouts/PlayerLayout.vue`:

1. Add to the imports: `import { useNowPlayingSheet } from '@/composables/useNowPlayingSheet'`
2. Add below the `useScrollbarWidth()` line:

```ts
// While the sheet is above collapsed it covers the whole shell: inert takes
// the covered content out of the tab order and the AT tree — the lightweight
// replacement for the focus trap the old overlay needed.
const sheet = useNowPlayingSheet()
```

3. Change `<div class="body-row">` to:

```vue
<div class="body-row" :inert="shell === 'mobile' && sheet.open.value">
```

4. Add `position: relative;` to the `.player-shell` rule (the sheet's `position: absolute; inset: 0` needs the shell as its containing block):

```css
.player-shell {
    position: relative;
    display: flex;
    flex-direction: column;
    width: 100%;
    overflow: hidden;
    background-color: var(--app-background);
}
```

- [ ] **Step 5: Make mobile `/` a pure alias**

Replace `webui/src/views/HomeView.vue` with:

```vue
<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import QueueView from '@/components/layout/QueueView.vue'
import { usePlayer } from '@/composables/usePlayer'
import { useViewport } from '@/composables/useViewport'

// `/` is Now Playing in both shells. Desktop renders the queue list; the
// mobile shell has no page here at all — Now Playing is the sheet
// (NowPlayingSheet), addressed by hash on whatever route sits underneath. So
// on mobile `/` is only an ADDRESS: it replaces itself with the landing page
// carrying the sheet's hash (#playing) when something is queued, or the bare
// landing page when not. replace(), not push(): the target stands in for `/`,
// so back must not bounce through it.
const { shell } = useViewport()
const player = usePlayer()
const router = useRouter()

const hasQueue = computed(() => player.queue.value.length > 0)

watch(
    [shell, hasQueue],
    ([currentShell, filled]) => {
        if (currentShell !== 'mobile') return
        void router.replace(filled ? { name: 'browse', hash: '#playing' } : { name: 'browse' })
    },
    { immediate: true }
)
</script>

<template>
    <QueueView v-if="shell === 'desktop'" variant="full" />
</template>
```

Update `webui/src/views/__tests__/HomeView.spec.ts`: the existing mobile cases assert `replace` targets. Change the expected values — queue non-empty on mobile now expects `replace({ name: 'browse', hash: '#playing' })` (previously it asserted `MobilePlayView` renders), queue empty keeps `replace({ name: 'browse' })`. Delete the `MobilePlayView` component stub/mock and any `MobilePlayView`-rendering assertions; keep the desktop `QueueView variant="full"` case as is. If an assertion references the removed component, the whole case is obsolete — replace it with:

```ts
it('mobile with a queue: `/` aliases to the landing page with the sheet open', async () => {
    // (reuse the file's existing shell/queue mocks: shell 'mobile', queue non-empty)
    mountView()
    expect(replace).toHaveBeenCalledWith({ name: 'browse', hash: '#playing' })
})
```

- [ ] **Step 6: Delete the dead files**

```bash
cd /home/bott/.datos/edit/programacion-privado/aether
grep -rn "MobilePlayView" webui/src --include='*.vue' --include='*.ts' | grep -v __tests__ | grep -v components/layout/MobilePlayView
# Expected: no output besides HomeView (already rewritten). Then:
git rm webui/src/components/layout/MobilePlayView.vue \
    webui/src/components/layout/__tests__/MobilePlayView.spec.ts \
    webui/src/components/layout/__tests__/MobilePlayView.layoutStyles.spec.ts \
    webui/src/components/layout/__tests__/MobilePlayView.paletteStyles.spec.ts
grep -rn "lib/motion" webui/src --include='*.vue' --include='*.ts'
# Expected: no output. Then:
git rm webui/src/lib/motion.ts
```

(If the `lib/motion` grep finds a consumer that appeared since this plan was written, keep `motion.ts` and skip its removal.)

- [ ] **Step 7: Update the safe-area style guard**

In `webui/src/components/layout/__tests__/mobile-chrome.safeArea.spec.ts`, keep the mini-player and browse-page cases unchanged and replace the two `MobilePlayView` cases with:

```ts
    // The mini strip is off-screen while the sheet is up, so the face and the
    // queue panel are the bottom-most surfaces of their detents and reserve
    // the home-indicator inset themselves.
    it('the player face reserves both insets', () => {
        const src = read('../PlayerFace.vue')
        expect(src).toContain('calc(0.5rem + env(safe-area-inset-bottom))')
        expect(src).toContain('padding: calc(0.25rem + env(safe-area-inset-top)) 1.5rem')
    })

    it('the queue panel reserves the top inset on its heading and the bottom under its list', () => {
        const src = read('../QueuePanel.vue')
        expect(src).toContain(
            'padding: calc(0.5rem + env(safe-area-inset-top)) var(--app-content-gutter) 0.5rem'
        )
        expect(src).toContain('padding-bottom: env(safe-area-inset-bottom)')
    })

    // The spacer reserves the strip's height in the shell column, since the
    // sheet overlays instead of docking.
    it('the shell spacer reserves the strip height including the bottom inset', () => {
        const src = read('../../../layouts/MobileShell.vue')
        expect(src).toContain(
            'height: calc(var(--app-mini-player-height) + env(safe-area-inset-bottom))'
        )
    })
```

- [ ] **Step 8: Run the full suite and fix collateral**

Run: `npm test` (from `webui/`; runs `vue-tsc --noEmit` then all of vitest).
Expected: PASS. Known collateral to fix if it appears, all mechanical:
- Any spec importing `MobilePlayView` or `@/lib/motion` → covered by the deletions above; if another file still references them, `vue-tsc` names it — update that import to the new components.
- `PlayerLayout.shellSwitch.spec.ts` mounts `PlayerLayout`; `useNowPlayingSheet` has no external dependencies and defaults to collapsed, so no new mock should be needed. If the spec asserts on `MiniPlayer` presence inside `MobileShell`, update it to assert `.mini-spacer` / `NowPlayingSheet` presence instead.

- [ ] **Step 9: Commit**

```bash
git add -A webui/src
git commit -m "feat(webui): mount the now-playing sheet in the mobile shell and retire MobilePlayView"
```

---

### Task 8: Documentation + final gate

**Files:**
- Modify: `docs/agents/frontend.md`
- Modify: `CLAUDE.md`
- Modify: `TODO.md`

- [ ] **Step 1: Update `docs/agents/frontend.md`**

In the *Shells* section, replace the two paragraphs describing mobile chrome ("Mobile chrome components live in `components/layout/` … while the drawer is shell chrome." — the paragraph mentioning `MiniPlayer` docking and hiding on `/`) and the whole "**Now Playing on the mobile shell is a first-class route, not an overlay.**" paragraph with:

```markdown
Mobile chrome is **one component**: `NowPlayingSheet` (`components/layout/`),
an always-mounted bottom sheet rendered by `MobileShell` over the route
content (plus a `.mini-spacer` flex child reserving the collapsed strip's
height, since the sheet overlays rather than docks). The sheet has three
detents — collapsed (mini strip) / playing (`PlayerFace`) / queue
(`QueuePanel`) — addressed by the **hash of whatever route is underneath**:
`''` / `#playing` / `#queue`. The hash is the single source of truth
(spec: `docs/superpowers/specs/2026-08-16-mobile-sheet-navigation-design.md`):
buttons and gestures commit it via `commitDetent` (deeper = `push`, so system
back walks `#queue` → `#playing` → page; shallower = `back()` when
vue-router's `history.state.back` matches the target, else `replace()`), and
the sheet's hash watcher animates toward it. Do not add parallel open/close
state, and do not turn Now Playing back into a route view — the content view
staying mounted underneath is what makes the dismiss drag reveal real UI.

Gestures (all finger-following, pure math in `lib/sheetGesture.ts`, state in
the `useNowPlayingSheet` singleton): lift the strip to open; drag the face up
for the queue or down to dismiss; pull the queue list down from its top — or
drag the queue heading at any scroll position — to return to the face. Claims
need 8px of dominant-vertical movement, the seek bar never arms, a claimed
drag swallows its release click, and settles honour flick velocity. While the
sheet is above collapsed, `PlayerLayout` sets `inert` on `.body-row`; the
collapsed sheet body and the faded strip are likewise `inert`. Reduced motion
is pure CSS (nothing waits on `transitionend`). On mobile, `/` is an alias:
`HomeView` `replace()`s to `/browse#playing` (queue filled) or `/browse`
(empty). The sheet strips a live sheet hash on unmount (queue emptied,
rotation to desktop). `MiniPlayer` is the sheet's dumb collapsed strip — it
renders and emits `open`, nothing more.
```

Also update the sentence in the routes/views part of the doc that says the mini player is "hidden on the Now Playing route", and any remaining reference to `MobilePlayView` (there is one in the media-session/queue-summary paragraph: "on the phone under the queue panel's title" stays true; change the component name to `QueuePanel`). Grep to catch them all:

```bash
grep -n "MobilePlayView\|PlayerSheet\|mini player is hidden" docs/agents/frontend.md
```

- [ ] **Step 2: Update `CLAUDE.md`**

In the main-content-views table, replace the `HomeView` row's description with:

```markdown
| `HomeView` | `/` | Now Playing. Desktop renders `QueueView variant="full"`; on the mobile shell `/` is only an alias — it replaces itself with `/browse#playing` (queue filled) or `/browse` (empty). The phone's Now Playing is `NowPlayingSheet`, an always-mounted bottom sheet addressed by the `#playing`/`#queue` hash on the current route (see `docs/agents/frontend.md`) |
```

- [ ] **Step 3: Tick the TODO**

In `TODO.md`, under `## Frontend - mobile`, mark `- [] better handling of back acctions` as done (`- [x]`) — back now walks queue → now playing → page by construction.

- [ ] **Step 4: Full gate**

```bash
cd webui && npm test && make lint
```
Expected: PASS (type-check, all vitest suites, eslint).

- [ ] **Step 5: Manual gate (chrome-devtools emulation, then a real phone)**

Emulate phone portrait (e.g. 390×844) against the LAN dev server and verify:
1. Content page + strip → slow lift follows the finger, release past midpoint opens, short release springs back; fast flick up opens from a short pull.
2. Face: drag up reveals queue (follows finger); drag down reveals the live page underneath and collapses; seek bar drags never move the sheet; double-tap favorite still works.
3. Queue: pull from list top returns to face following the finger; heading drag works with the list scrolled; scrolled-list pulls scroll the list only.
4. Back button: from `#queue` → face; from `#playing` → page; queue empty → plain page history.
5. URL: swipes update the hash; reload on `#playing` and `#queue` lands on that detent with no animation; repeated open/close does not grow history (forward button stays dead after a settle-back).
6. `/` typed manually → lands on `/browse#playing` (with queue) or `/browse` (empty).
7. Rotate to tablet-landscape (desktop shell): sheet unmounts, hash stripped, desktop unchanged.
8. Emulate `prefers-reduced-motion: reduce`: detent changes jump without sliding.
9. Real phone pass (LAN): all four legs + Android back + pull-to-refresh never triggers mid-gesture.

- [ ] **Step 6: Commit**

```bash
git add docs/agents/frontend.md CLAUDE.md TODO.md
git commit -m "docs: describe the hash-addressed mobile now-playing sheet"
```
