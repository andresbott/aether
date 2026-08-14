import { describe, it, expect, vi, afterEach } from 'vitest'

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
        removeEventListener: (_: 'change', fn: Listener) => {
            const list = listeners.get(query) ?? []
            listeners.set(query, list.filter((handler) => handler !== fn))
        }
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
const Q_TALL = '(min-height: 600px)'
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
        // [desktopWidth, phoneWidth, landscape, tall, expectedShell, expectedTier]
        [true, false, true, true, 'desktop', 'desktop'],   // wide screen
        [true, false, false, true, 'desktop', 'desktop'],  // wide, portrait monitor
        [false, true, true, false, 'mobile', 'phone'],     // narrow phone landscape stays mobile
        [false, true, false, true, 'mobile', 'phone'],     // phone portrait
        [false, false, true, true, 'desktop', 'tablet'],   // tablet landscape → desktop
        // A LANDSCAPE PHONE: modern phones on their side land in the tablet
        // width band (iPhone 15: 852x393) but are nowhere near tablet-tall.
        [false, false, true, false, 'mobile', 'tablet'],   // landscape phone stays mobile
        [false, false, false, true, 'mobile', 'tablet']    // tablet portrait → mobile
    ])(
        'desktop=%s phone=%s landscape=%s tall=%s → shell=%s tier=%s',
        async (desktop, phone, landscape, tall, expectedShell, expectedTier) => {
            const matching = new Set<string>()
            if (desktop) matching.add(Q_DESKTOP)
            if (phone) matching.add(Q_PHONE)
            if (landscape) matching.add(Q_LANDSCAPE)
            if (tall) matching.add(Q_TALL)
            installMatchMedia(matching)
            const { useViewport } = await load()
            const vp = useViewport()
            expect(vp.shell.value).toBe(expectedShell)
            expect(vp.tier.value).toBe(expectedTier)
        }
    )

    it('flips shell reactively when a tablet rotates', async () => {
        const mm = installMatchMedia(new Set([Q_LANDSCAPE, Q_TALL])) // tablet landscape
        const { useViewport } = await load()
        const vp = useViewport()
        expect(vp.shell.value).toBe('desktop')
        mm.set(Q_LANDSCAPE, false) // rotate to portrait
        expect(vp.shell.value).toBe('mobile')
    })

    it('keeps a rotating phone on the mobile shell', async () => {
        // Pixel 8 portrait: 412x915 → phone width, tall.
        const mm = installMatchMedia(new Set([Q_PHONE, Q_TALL]))
        const { useViewport } = await load()
        const vp = useViewport()
        expect(vp.shell.value).toBe('mobile')
        // Rotate: 915x412 → tablet width band, landscape, NOT tall.
        mm.set(Q_PHONE, false)
        mm.set(Q_TALL, false)
        mm.set(Q_LANDSCAPE, true)
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

// Guards the suite-wide default: the global test setup (src/test-setup.ts)
// resolves media queries against jsdom's real 1024x768 viewport, so every
// mounted test that does not stub useViewport itself runs the DESKTOP chrome
// (spec §6). A stub answering `false` to everything would silently make the
// whole suite mobile — this test fails if that regresses.
describe('useViewport under the global test setup (no local stubbing)', () => {
    it('resolves the desktop shell from jsdom’s real viewport', async () => {
        const { useViewport } = await load() // resetViewportForTests() drops any cached singleton
        const vp = useViewport()
        expect(vp.shell.value).toBe('desktop')
        expect(vp.tier.value).toBe('desktop')
    })
})
