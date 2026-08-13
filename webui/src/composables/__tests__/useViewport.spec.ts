import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'

// matchMedia stub: tests flip which queries match and fire `change` events.
type Listener = (e: { matches: boolean }) => void

function installMatchMedia(matching: Set<string>) {
    const listeners = new Map<string, Listener[]>()
    vi.stubGlobal('matchMedia', (query: string) => ({
        matches: matching.has(query),
        media: query,
        addEventListener: (_: 'change', fn: Listener) => {
            listeners.set(query, [...(listeners.get(query) ?? []), fn])
        },
        removeEventListener: () => {}
    }))
    return {
        set(query: string, matches: boolean) {
            if (matches) matching.add(query)
            else matching.delete(query)
            for (const fn of listeners.get(query) ?? []) fn({ matches })
        }
    }
}

const Q_DESKTOP = '(min-width: 1024px)'
const Q_PHONE = '(max-width: 767.98px)'
const Q_LANDSCAPE = '(orientation: landscape)'
const Q_COARSE = '(pointer: coarse)'

async function load() {
    const mod = await import('../useViewport')
    mod.resetViewportForTests()
    return mod
}

afterEach(() => {
    vi.unstubAllGlobals()
})

describe('useViewport shell decision table', () => {
    it.each([
        // [desktopWidth, phoneWidth, landscape, expectedShell, expectedTier]
        [true, false, true, 'desktop', 'desktop'],   // wide screen
        [true, false, false, 'desktop', 'desktop'],  // wide, portrait monitor
        [false, true, true, 'mobile', 'phone'],      // phone landscape stays mobile
        [false, true, false, 'mobile', 'phone'],     // phone portrait
        [false, false, true, 'desktop', 'tablet'],   // tablet landscape → desktop
        [false, false, false, 'mobile', 'tablet']    // tablet portrait → mobile
    ])(
        'desktop=%s phone=%s landscape=%s → shell=%s tier=%s',
        async (desktop, phone, landscape, expectedShell, expectedTier) => {
            const matching = new Set<string>()
            if (desktop) matching.add(Q_DESKTOP)
            if (phone) matching.add(Q_PHONE)
            if (landscape) matching.add(Q_LANDSCAPE)
            installMatchMedia(matching)
            const { useViewport } = await load()
            const vp = useViewport()
            expect(vp.shell.value).toBe(expectedShell)
            expect(vp.tier.value).toBe(expectedTier)
        }
    )

    it('flips shell reactively when a tablet rotates', async () => {
        const mm = installMatchMedia(new Set([Q_LANDSCAPE])) // tablet landscape
        const { useViewport } = await load()
        const vp = useViewport()
        expect(vp.shell.value).toBe('desktop')
        mm.set(Q_LANDSCAPE, false) // rotate to portrait
        expect(vp.shell.value).toBe('mobile')
    })

    it('reports isTouch from (pointer: coarse) independently of shell', async () => {
        installMatchMedia(new Set([Q_DESKTOP, Q_COARSE]))
        const { useViewport } = await load()
        const vp = useViewport()
        expect(vp.shell.value).toBe('desktop')
        expect(vp.isTouch.value).toBe(true)
    })

    it('defaults to the desktop shell when matchMedia is unavailable', async () => {
        vi.stubGlobal('matchMedia', undefined)
        const { useViewport } = await load()
        const vp = useViewport()
        expect(vp.shell.value).toBe('desktop')
        expect(vp.isTouch.value).toBe(false)
    })

    it('is a singleton: two calls share state', async () => {
        installMatchMedia(new Set([Q_PHONE]))
        const { useViewport } = await load()
        expect(useViewport().shell).toBe(useViewport().shell)
    })
})
