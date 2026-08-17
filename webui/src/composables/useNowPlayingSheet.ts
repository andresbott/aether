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
