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
