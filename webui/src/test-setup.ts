import { enableAutoUnmount } from '@vue/test-utils'
import { afterEach } from 'vitest'

enableAutoUnmount(afterEach)

// jsdom does not implement ResizeObserver; stub it for component tests that use
// PrimeVue TabList (which binds a ResizeObserver in its mounted hook).
if (typeof ResizeObserver === 'undefined') {
    global.ResizeObserver = class ResizeObserver {
        observe() {}
        unobserve() {}
        disconnect() {}
    }
}

// jsdom does not implement matchMedia; stub it so useTheme()'s initTheme()
// can query prefers-color-scheme without throwing.
if (typeof window !== 'undefined' && typeof window.matchMedia !== 'function') {
    window.matchMedia = (query: string) =>
        ({
            matches: false,
            media: query,
            onchange: null,
            addEventListener() {},
            removeEventListener() {},
            addListener() {},
            removeListener() {},
            dispatchEvent() {
                return false
            }
        }) as MediaQueryList
}
