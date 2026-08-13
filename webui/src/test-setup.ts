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

// jsdom does not implement matchMedia; stub it so useTheme()'s initTheme() can
// query prefers-color-scheme without throwing.
//
// A stub that answers `false` to everything is not neutral: through
// useViewport() it reads as "not desktop width, not phone width, portrait" →
// tier 'tablet' + portrait → shell 'mobile', so every mounted test would run
// the mobile chrome. Spec §6 requires the suite to run the DESKTOP shell.
// So resolve the width/orientation queries against jsdom's real viewport
// (1024x768 by default) and let the real useViewport code path decide; that
// yields tier 'desktop' / shell 'desktop'. Everything else (prefers-color-scheme,
// pointer: coarse, …) stays false.
if (typeof window !== 'undefined') {
    const resolve = (query: string): boolean => {
        const min = /^\(min-width:\s*([\d.]+)px\)$/.exec(query)
        if (min) return window.innerWidth >= Number.parseFloat(min[1])
        const max = /^\(max-width:\s*([\d.]+)px\)$/.exec(query)
        if (max) return window.innerWidth <= Number.parseFloat(max[1])
        if (query === '(orientation: landscape)') return window.innerWidth >= window.innerHeight
        return false
    }

    window.matchMedia = (query: string) =>
        ({
            // getter, not a snapshot: a spec that reassigns window.innerWidth
            // sees the new answer without reinstalling the stub.
            get matches() {
                return resolve(query)
            },
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
