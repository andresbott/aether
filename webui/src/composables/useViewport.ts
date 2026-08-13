import { computed, ref, type ComputedRef, type Ref } from 'vue'
import { BP_DESKTOP_MIN, BP_PHONE_MAX } from '@/lib/breakpoints'

export type ViewportTier = 'phone' | 'tablet' | 'desktop'
export type ViewportShell = 'desktop' | 'mobile'

interface ViewportState {
    shell: ComputedRef<ViewportShell>
    tier: ComputedRef<ViewportTier>
    isTouch: Ref<boolean>
}

// The -0.02px keeps 768px itself out of the phone tier (a 768-wide viewport is
// a tablet). Fractional widths between the two queries don't occur in practice.
const QUERIES = {
    desktop: `(min-width: ${BP_DESKTOP_MIN}px)`,
    phone: `(max-width: ${BP_PHONE_MAX - 0.02}px)`,
    landscape: '(orientation: landscape)',
    coarse: '(pointer: coarse)'
} as const

// Module-scoped singleton, same pattern as usePlayer: every caller shares one
// set of media-query listeners and one reactive answer.
let state: ViewportState | null = null

function track(query: string, apply: (matches: boolean) => void): void {
    const mql = window.matchMedia(query)
    apply(mql.matches)
    mql.addEventListener('change', (e) => apply(e.matches))
}

function createState(): ViewportState {
    const isDesktopWidth = ref(true)
    const isPhoneWidth = ref(false)
    const landscape = ref(true)
    const isTouch = ref(false)

    // jsdom (and nothing else we support) lacks matchMedia: default to the
    // desktop shell rather than throwing in every mounted test (spec §6).
    if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
        track(QUERIES.desktop, (m) => (isDesktopWidth.value = m))
        track(QUERIES.phone, (m) => (isPhoneWidth.value = m))
        track(QUERIES.landscape, (m) => (landscape.value = m))
        track(QUERIES.coarse, (m) => (isTouch.value = m))
    }

    const tier = computed<ViewportTier>(() => {
        if (isDesktopWidth.value) return 'desktop'
        if (isPhoneWidth.value) return 'phone'
        return 'tablet'
    })

    // The one shell decision everything keys off: desktop width → desktop,
    // phone width → mobile, tablet → orientation picks (spec §2.1).
    const shell = computed<ViewportShell>(() => {
        if (tier.value === 'desktop') return 'desktop'
        if (tier.value === 'phone') return 'mobile'
        return landscape.value ? 'desktop' : 'mobile'
    })

    return { shell, tier, isTouch }
}

export function useViewport(): ViewportState {
    if (!state) state = createState()
    return state
}

/** Test hook: drop the singleton so the next call re-reads matchMedia. */
export function resetViewportForTests(): void {
    state = null
}
