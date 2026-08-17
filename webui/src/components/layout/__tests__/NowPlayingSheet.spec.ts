import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { reactive } from 'vue'
import {
    resetNowPlayingSheetForTests,
    useNowPlayingSheet
} from '@/composables/useNowPlayingSheet'

const push = vi.fn()
const replace = vi.fn()
const back = vi.fn()
const route = reactive({ path: '/library', fullPath: '/library', hash: '' })
const currentRoute = { value: route }
const resolve = vi.fn(({ hash }: { hash: string }) => ({ fullPath: `/library${hash}` }))
vi.mock('vue-router', () => ({
    useRouter: () => ({ push, replace, back, resolve, currentRoute }),
    useRoute: () => route
}))

// The sheet is pure glue: strip/face/queue internals are covered by their own
// specs, so stub them down to the nodes the sheet wires.
vi.mock('@/components/layout/MiniPlayer.vue', () => ({
    default: {
        name: 'MiniPlayer',
        emits: ['open'],
        template: '<div class="stub-mini"><button class="stub-open" @click="$emit(\'open\')"></button></div>'
    }
}))
vi.mock('@/components/layout/PlayerFace.vue', () => ({
    default: {
        name: 'PlayerFace',
        emits: ['collapse', 'show-queue'],
        template:
            '<div class="stub-face"><div class="play-seek"></div>' +
            '<button class="stub-collapse" @click="$emit(\'collapse\')"></button>' +
            '<button class="stub-show-queue" @click="$emit(\'show-queue\')"></button></div>'
    }
}))
vi.mock('@/components/layout/QueuePanel.vue', () => ({
    default: {
        name: 'QueuePanel',
        template:
            '<div class="stub-queue"><header class="queue-heading"></header>' +
            '<div class="play-queue-list"><div class="stub-list"></div></div></div>'
    }
}))

import NowPlayingSheet from '@/components/layout/NowPlayingSheet.vue'

const mountSheet = () => mount(NowPlayingSheet)

const sheetTransform = (w: ReturnType<typeof mountSheet>): string =>
    (w.find('.now-playing-sheet').element as HTMLElement).style.transform
const trackTransform = (w: ReturnType<typeof mountSheet>): string =>
    (w.find('.sheet-track').element as HTMLElement).style.transform

beforeEach(() => {
    resetNowPlayingSheetForTests()
    route.hash = ''
    route.fullPath = '/library'
    window.history.replaceState({}, '', '/library')
    vi.clearAllMocks()
})

describe('NowPlayingSheet — transforms per detent', () => {
    it('rests collapsed: pushed down to its strip, track at the face', () => {
        const w = mountSheet()
        // 1 - expand fraction = 1 → the full (100% - strip) offset applies.
        expect(sheetTransform(w)).toContain('* 1)')
        // String(-0 * 50) is "0": at the face the track reads as an explicit 0.
        expect(trackTransform(w)).toBe('translateY(0%)')
    })

    it('playing: sheet fully up, track still on the face', async () => {
        const w = mountSheet()
        useNowPlayingSheet().snapTo('playing')
        await w.vm.$nextTick()
        expect(sheetTransform(w)).toContain('* 0)')
        expect(trackTransform(w)).toBe('translateY(0%)')
    })

    it('queue: sheet up and track shifted one panel', async () => {
        const w = mountSheet()
        useNowPlayingSheet().snapTo('queue')
        await w.vm.$nextTick()
        expect(sheetTransform(w)).toContain('* 0)')
        expect(trackTransform(w)).toBe('translateY(-50%)')
    })
})

describe('NowPlayingSheet — the hash is the source of truth', () => {
    it('mounts straight onto the detent the hash names', () => {
        route.hash = '#queue'
        const w = mountSheet()
        expect(useNowPlayingSheet().detent.value).toBe('queue')
        expect(trackTransform(w)).toBe('translateY(-50%)')
    })

    it('a hash change (back button, drawer-less nav) moves the sheet', async () => {
        const w = mountSheet()
        route.hash = '#playing'
        await w.vm.$nextTick()
        expect(useNowPlayingSheet().detent.value).toBe('playing')
        route.hash = ''
        await w.vm.$nextTick()
        expect(useNowPlayingSheet().detent.value).toBe('collapsed')
        expect(sheetTransform(w)).toContain('* 1)')
    })

    it('unmounting with a live sheet hash strips it — no stale #playing on desktop', async () => {
        const w = mountSheet()
        route.hash = '#playing'
        await w.vm.$nextTick()
        w.unmount()
        // Deferred a microtask: flush it.
        await Promise.resolve()
        expect(replace).toHaveBeenCalledWith({ hash: '' })
        expect(useNowPlayingSheet().detent.value).toBe('collapsed')
    })

    it('unmounting collapsed leaves the route alone', async () => {
        const w = mountSheet()
        w.unmount()
        await Promise.resolve()
        expect(replace).not.toHaveBeenCalled()
    })

    it('does not replace when the route hash already changed by the time the microtask runs', async () => {
        const w = mountSheet()
        route.hash = '#playing'
        await w.vm.$nextTick()
        // Concurrent navigation changed the hash before unmount's microtask runs
        // (e.g. MobileBrowseView's desktop redirect on rotation).
        currentRoute.value = { ...route, hash: '' }
        w.unmount()
        await Promise.resolve()
        // The hash is already collapsed: no replace needed.
        expect(replace).not.toHaveBeenCalled()
    })
})

describe('NowPlayingSheet — buttons route through the hash', () => {
    it('the mini strip open request pushes #playing', async () => {
        const w = mountSheet()
        await w.find('.stub-open').trigger('click')
        expect(push).toHaveBeenCalledWith({ hash: '#playing' })
    })

    it('the face ⌃ pushes #queue and the ⌄ pops back to the page', async () => {
        route.hash = '#playing'
        const w = mountSheet()
        await w.find('.stub-show-queue').trigger('click')
        expect(push).toHaveBeenCalledWith({ hash: '#queue' })

        window.history.replaceState({ back: '/library' }, '', '/library#playing')
        await w.find('.stub-collapse').trigger('click')
        expect(back).toHaveBeenCalledOnce()
    })
})

describe('NowPlayingSheet — layered a11y', () => {
    // Vue may set `inert` as a DOM property (when jsdom exposes it) or as an
    // attribute; accept either so the assertion pins the intent, not the
    // serialization.
    const isInert = (w: ReturnType<typeof mountSheet>, selector: string): boolean => {
        const el = w.find(selector).element as HTMLElement & { inert?: boolean }
        return el.inert === true || el.hasAttribute('inert')
    }

    it('collapsed: the body is inert, the strip is live', () => {
        const w = mountSheet()
        expect(isInert(w, '.sheet-body')).toBe(true)
        expect(isInert(w, '.sheet-strip')).toBe(false)
        expect(w.find('.sheet-strip').classes()).not.toContain('strip-hidden')
    })

    it('expanded: the strip is inert and hidden, the body is live', async () => {
        const w = mountSheet()
        useNowPlayingSheet().snapTo('playing')
        await w.vm.$nextTick()
        expect(isInert(w, '.sheet-body')).toBe(false)
        expect(isInert(w, '.sheet-strip')).toBe(true)
        expect(w.find('.sheet-strip').classes()).toContain('strip-hidden')
    })

    it('playing: face panel is live, queue panel is inert', async () => {
        const w = mountSheet()
        useNowPlayingSheet().snapTo('playing')
        await w.vm.$nextTick()
        expect(isInert(w, '.sheet-panel-face')).toBe(false)
        expect(isInert(w, '.sheet-panel-queue')).toBe(true)
    })

    it('queue: queue panel is live, face panel is inert', async () => {
        const w = mountSheet()
        useNowPlayingSheet().snapTo('queue')
        await w.vm.$nextTick()
        expect(isInert(w, '.sheet-panel-face')).toBe(true)
        expect(isInert(w, '.sheet-panel-queue')).toBe(false)
    })

    it('collapsed: both panels are inert (body is inert covers them)', async () => {
        const w = mountSheet()
        // Already collapsed, but explicitly set it.
        useNowPlayingSheet().snapTo('collapsed')
        await w.vm.$nextTick()
        expect(isInert(w, '.sheet-body')).toBe(true)
        // Panels still have their own inert, but the body inert covers them.
        expect(isInert(w, '.sheet-panel-queue')).toBe(true)
    })
})
