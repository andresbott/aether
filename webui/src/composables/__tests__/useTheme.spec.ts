import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { nextTick } from 'vue'

type ChangeHandler = (e: { matches: boolean }) => void

// Build a controllable matchMedia mock. `setMatches` drives the media query
// and fires the subscribed `change` handler, emulating an OS theme switch.
function installMatchMedia(initialMatches: boolean) {
    let matches = initialMatches
    let handler: ChangeHandler | null = null

    window.matchMedia = ((query: string) =>
        ({
            get matches() {
                return matches
            },
            media: query,
            onchange: null,
            addEventListener: (_: string, cb: ChangeHandler) => {
                handler = cb
            },
            removeEventListener: () => {
                handler = null
            },
            addListener: () => {},
            removeListener: () => {},
            dispatchEvent: () => false
        }) as unknown as MediaQueryList) as typeof window.matchMedia

    return {
        setMatches(value: boolean) {
            matches = value
            handler?.({ matches: value })
        }
    }
}

// useTheme is a module-level singleton; reset the module registry before each
// test so `initialized` and the shared refs start fresh.
beforeEach(() => {
    vi.resetModules()
    document.documentElement.classList.remove('dark-mode')
})

afterEach(() => {
    document.documentElement.classList.remove('dark-mode')
})

describe('useTheme', () => {
    it('defaults to auto', async () => {
        installMatchMedia(false)
        const { useTheme } = await import('../useTheme')
        const { mode } = useTheme()
        expect(mode.value).toBe('auto')
    })

    it('follows the system preference when auto', async () => {
        const mm = installMatchMedia(true)
        const { useTheme } = await import('../useTheme')
        const { isDark } = useTheme()

        await nextTick()
        expect(isDark.value).toBe(true)
        expect(document.documentElement.classList.contains('dark-mode')).toBe(true)

        mm.setMatches(false)
        await nextTick()
        expect(isDark.value).toBe(false)
        expect(document.documentElement.classList.contains('dark-mode')).toBe(false)
    })

    it('forces dark regardless of the system preference', async () => {
        installMatchMedia(false)
        const { useTheme } = await import('../useTheme')
        const { mode, isDark } = useTheme()

        mode.value = 'dark'
        await nextTick()
        expect(isDark.value).toBe(true)
        expect(document.documentElement.classList.contains('dark-mode')).toBe(true)
    })

    it('forces light regardless of the system preference', async () => {
        installMatchMedia(true)
        const { useTheme } = await import('../useTheme')
        const { mode, isDark } = useTheme()

        mode.value = 'light'
        await nextTick()
        expect(isDark.value).toBe(false)
        expect(document.documentElement.classList.contains('dark-mode')).toBe(false)
    })
})
