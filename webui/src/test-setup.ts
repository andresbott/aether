import { config, enableAutoUnmount } from '@vue/test-utils'
import Tooltip from 'primevue/tooltip'
import { afterEach } from 'vitest'

enableAutoUnmount(afterEach)

// v-tooltip is registered on the real app in main.ts; component tests mount
// single components, so register it globally too. Without this every component
// using v-tooltip floods stderr with "Failed to resolve directive: tooltip"
// plus a full props dump. Specs that assert on tooltip content still pass their
// own `directives: { tooltip: ... }` recorder, which takes precedence.
config.global.directives = { ...config.global.directives, tooltip: Tooltip }

// jsdom does not implement ResizeObserver; stub it for component tests that use
// PrimeVue TabList (which binds a ResizeObserver in its mounted hook).
if (typeof ResizeObserver === 'undefined') {
    global.ResizeObserver = class ResizeObserver {
        observe() {}
        unobserve() {}
        disconnect() {}
    }
}

// jsdom does not implement HTMLMediaElement playback methods; stub them so
// usePlayer()'s play/pause/load calls don't spew "Not implemented" errors.
if (typeof window !== 'undefined' && typeof window.HTMLMediaElement !== 'undefined') {
    window.HTMLMediaElement.prototype.play = function () {
        return Promise.resolve()
    }
    window.HTMLMediaElement.prototype.pause = function () {}
    window.HTMLMediaElement.prototype.load = function () {}
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
