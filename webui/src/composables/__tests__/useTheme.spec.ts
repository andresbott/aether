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
const THEME_CLASSES = ['dark-mode', 'theme-winamp', 'theme-crt']

beforeEach(() => {
    vi.resetModules()
    localStorage.clear()
    document.documentElement.classList.remove(...THEME_CLASSES)
})

afterEach(() => {
    document.documentElement.classList.remove(...THEME_CLASSES)
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

describe('useTheme hidden themes', () => {
    it('keeps them out of the picker until unlocked', async () => {
        installMatchMedia(false)
        const { useTheme } = await import('../useTheme')
        const { options, hiddenUnlocked, unlockHiddenThemes } = useTheme()

        expect(hiddenUnlocked.value).toBe(false)
        expect(options.value.map((o) => o.value)).toEqual(['auto', 'light', 'dark'])

        unlockHiddenThemes()
        expect(hiddenUnlocked.value).toBe(true)
        expect(options.value.map((o) => o.value)).toEqual([
            'auto',
            'light',
            'dark',
            'winamp',
            'crt'
        ])
    })

    it('remembers the unlock across reloads', async () => {
        installMatchMedia(false)
        const first = await import('../useTheme')
        first.useTheme().unlockHiddenThemes()

        // Fresh module registry = a page reload with the same localStorage.
        vi.resetModules()
        const second = await import('../useTheme')
        expect(second.useTheme().hiddenUnlocked.value).toBe(true)
    })

    it('cycles winamp → crt → winamp', async () => {
        installMatchMedia(false)
        const { useTheme } = await import('../useTheme')
        const { mode, cycleHiddenTheme } = useTheme()

        expect(cycleHiddenTheme().value).toBe('winamp')
        expect(mode.value).toBe('winamp')
        expect(cycleHiddenTheme().value).toBe('crt')
        expect(cycleHiddenTheme().value).toBe('winamp')
    })

    it('applies dark-mode plus exactly one theme class', async () => {
        installMatchMedia(false)
        const { useTheme } = await import('../useTheme')
        const { mode, isDark } = useTheme()

        mode.value = 'crt'
        await nextTick()
        expect(isDark.value).toBe(true)
        const list = document.documentElement.classList
        expect(list.contains('dark-mode')).toBe(true)
        expect(list.contains('theme-crt')).toBe(true)
        expect(list.contains('theme-winamp')).toBe(false)

        mode.value = 'winamp'
        await nextTick()
        expect(list.contains('theme-crt')).toBe(false)
        expect(list.contains('theme-winamp')).toBe(true)
    })

    it('drops the theme class when returning to a standard mode', async () => {
        installMatchMedia(false)
        const { useTheme } = await import('../useTheme')
        const { mode, isDark } = useTheme()

        mode.value = 'winamp'
        await nextTick()
        mode.value = 'light'
        await nextTick()

        expect(isDark.value).toBe(false)
        expect(document.documentElement.classList.contains('theme-winamp')).toBe(false)
        expect(document.documentElement.classList.contains('dark-mode')).toBe(false)
    })
})
