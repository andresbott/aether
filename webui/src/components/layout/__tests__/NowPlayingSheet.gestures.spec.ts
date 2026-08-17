// webui/src/components/layout/__tests__/NowPlayingSheet.gestures.spec.ts
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

// Velocity depends on real event timestamps, which jsdom makes effectively
// random — a fast test run reads as a flick. Position decides here; flick
// behavior is covered by sheetGesture's own unit spec.
vi.mock('@/lib/sheetGesture', async (importOriginal) => {
    const actual = await importOriginal<typeof import('@/lib/sheetGesture')>()
    return { ...actual, settleDetent: (p: number, _v: number, min: number, max: number) =>
        Math.min(Math.max(Math.round(p), min), max) }
})

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
        template: '<div class="stub-face"><div class="play-seek"></div></div>'
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

const H = 800
const STRIP = 60

const mountSheet = () => {
    const w = mount(NowPlayingSheet)
    Object.defineProperty(w.find('.now-playing-sheet').element, 'offsetHeight', {
        value: H,
        configurable: true
    })
    Object.defineProperty(w.find('.sheet-strip').element, 'offsetHeight', {
        value: STRIP,
        configurable: true
    })
    return w
}

const touch = async (
    w: ReturnType<typeof mountSheet>,
    selector: string,
    kind: 'touchstart' | 'touchmove',
    y: number,
    x = 0
) => {
    await w.find(selector).trigger(kind, { touches: [{ clientY: y, clientX: x }] })
}
const release = async (w: ReturnType<typeof mountSheet>, selector: string) => {
    await w.find(selector).trigger('touchend')
}

const sheet = () => useNowPlayingSheet()

beforeEach(() => {
    resetNowPlayingSheetForTests()
    route.hash = ''
    route.fullPath = '/library'
    window.history.replaceState({}, '', '/library')
    vi.clearAllMocks()
})

describe('strip lift (collapsed → playing)', () => {
    it('follows the finger with the transition off, and nothing is decided mid-drag', async () => {
        const w = mountSheet()
        await touch(w, '.sheet-strip', 'touchstart', 700)
        await touch(w, '.sheet-strip', 'touchmove', 330) // 370px = half of 740 travel
        expect(sheet().position.value).toBeCloseTo(0.5, 5)
        expect(sheet().dragging.value).toBe(true)
        expect(w.find('.now-playing-sheet').classes()).toContain('is-dragging')
        expect(push).not.toHaveBeenCalled()
    })

    it('released past the midpoint it settles open and pushes #playing', async () => {
        const w = mountSheet()
        await touch(w, '.sheet-strip', 'touchstart', 700)
        await touch(w, '.sheet-strip', 'touchmove', 200)
        await release(w, '.sheet-strip')
        expect(sheet().detent.value).toBe('playing')
        expect(sheet().position.value).toBe(1)
        expect(sheet().dragging.value).toBe(false)
        expect(push).toHaveBeenCalledWith({ hash: '#playing' })
    })

    it('released short it springs back and navigates nowhere', async () => {
        const w = mountSheet()
        await touch(w, '.sheet-strip', 'touchstart', 700)
        await touch(w, '.sheet-strip', 'touchmove', 600)
        await release(w, '.sheet-strip')
        expect(sheet().position.value).toBe(0)
        expect(push).not.toHaveBeenCalled()
    })

    it('ignores movement inside the slop — a wobbly tap is still a tap', async () => {
        const w = mountSheet()
        await touch(w, '.sheet-strip', 'touchstart', 700)
        await touch(w, '.sheet-strip', 'touchmove', 695)
        expect(sheet().dragging.value).toBe(false)
        expect(sheet().position.value).toBe(0)
    })

    it('a downward drag on the strip claims nothing — the bar only goes one way', async () => {
        const w = mountSheet()
        await touch(w, '.sheet-strip', 'touchstart', 700)
        await touch(w, '.sheet-strip', 'touchmove', 760)
        expect(sheet().dragging.value).toBe(false)
        expect(sheet().position.value).toBe(0)
    })

    it('swallows the click the browser delivers after a claimed drag', async () => {
        const w = mountSheet()
        await touch(w, '.sheet-strip', 'touchstart', 700)
        await touch(w, '.sheet-strip', 'touchmove', 600)
        await release(w, '.sheet-strip')
        await w.find('.stub-open').trigger('click')
        // The drag's release-click must not ALSO open the sheet.
        expect(push).not.toHaveBeenCalled()
        // Only once: the next real tap works again.
        await w.find('.stub-open').trigger('click')
        expect(push).toHaveBeenCalledWith({ hash: '#playing' })
    })

    it('does not leak swallowClick when a claimed drag ends via touchcancel', async () => {
        const w = mountSheet()
        await touch(w, '.sheet-strip', 'touchstart', 700)
        await touch(w, '.sheet-strip', 'touchmove', 600)
        // Claimed drag ends via touchcancel instead of touchend → no click.
        await w.find('.sheet-strip').trigger('touchcancel')
        // The NEXT tap must still work — swallowClick should be reset by the
        // new touchstart, not left armed from the canceled drag.
        await touch(w, '.stub-open', 'touchstart', 700)
        await release(w, '.stub-open')
        await w.find('.stub-open').trigger('click')
        expect(push).toHaveBeenCalledWith({ hash: '#playing' })
    })
})

describe('face drags (playing → collapsed / queue)', () => {
    const mountAtPlaying = () => {
        route.hash = '#playing'
        return mountSheet()
    }

    it('dragging down follows the finger toward collapsed', async () => {
        const w = mountAtPlaying()
        await touch(w, '.sheet-panel-face', 'touchstart', 100)
        await touch(w, '.sheet-panel-face', 'touchmove', 470) // 370 = half of 740
        expect(sheet().position.value).toBeCloseTo(0.5, 5)
    })

    it('released low it collapses — back() when the page entry is right below', async () => {
        window.history.replaceState({ back: '/library' }, '', '/library#playing')
        const w = mountAtPlaying()
        await touch(w, '.sheet-panel-face', 'touchstart', 100)
        await touch(w, '.sheet-panel-face', 'touchmove', 700)
        await release(w, '.sheet-panel-face')
        expect(sheet().detent.value).toBe('collapsed')
        expect(back).toHaveBeenCalledOnce()
        expect(replace).not.toHaveBeenCalled()
    })

    it('released low on a deep link it rewrites in place instead of leaving the app', async () => {
        window.history.replaceState({ back: null }, '', '/library#playing')
        const w = mountAtPlaying()
        await touch(w, '.sheet-panel-face', 'touchstart', 100)
        await touch(w, '.sheet-panel-face', 'touchmove', 700)
        await release(w, '.sheet-panel-face')
        expect(replace).toHaveBeenCalledWith({ hash: '' })
        expect(back).not.toHaveBeenCalled()
    })

    it('dragging up reveals the queue and pushes #queue on release', async () => {
        const w = mountAtPlaying()
        await touch(w, '.sheet-panel-face', 'touchstart', 700)
        await touch(w, '.sheet-panel-face', 'touchmove', 100) // 600 of 800 queue travel
        expect(sheet().position.value).toBeCloseTo(1.75, 5)
        await release(w, '.sheet-panel-face')
        expect(sheet().detent.value).toBe('queue')
        expect(push).toHaveBeenCalledWith({ hash: '#queue' })
    })

    it('a drag that starts on the seek bar never claims — off-axis seeking stays a seek', async () => {
        const w = mountAtPlaying()
        await touch(w, '.play-seek', 'touchstart', 100)
        await touch(w, '.play-seek', 'touchmove', 700)
        expect(sheet().dragging.value).toBe(false)
        expect(sheet().position.value).toBe(1)
    })

    it('a horizontal-dominant move never claims', async () => {
        const w = mountAtPlaying()
        await touch(w, '.sheet-panel-face', 'touchstart', 100, 0)
        await touch(w, '.sheet-panel-face', 'touchmove', 130, 200)
        expect(sheet().dragging.value).toBe(false)
    })
})

describe('queue drags (queue → playing)', () => {
    const mountAtQueue = () => {
        route.hash = '#queue'
        return mountSheet()
    }

    it('a pull that starts with the list at its top follows the finger back to the face', async () => {
        window.history.replaceState({ back: '/library#playing' }, '', '/library#queue')
        const w = mountAtQueue()
        await touch(w, '.play-queue-list', 'touchstart', 100)
        await touch(w, '.play-queue-list', 'touchmove', 500) // 400 of 800
        expect(sheet().position.value).toBeCloseTo(1.5, 5)
        await touch(w, '.play-queue-list', 'touchmove', 700)
        await release(w, '.play-queue-list')
        expect(sheet().detent.value).toBe('playing')
        expect(back).toHaveBeenCalledOnce()
    })

    it('does not arm while the list is scrolled down — that pull scrolls the list', async () => {
        const w = mountAtQueue()
        const list = w.find('.stub-list')
        ;(list.element as HTMLElement).scrollTop = 50
        await touch(w, '.stub-list', 'touchstart', 100)
        await touch(w, '.stub-list', 'touchmove', 700)
        expect(sheet().dragging.value).toBe(false)
        expect(sheet().position.value).toBe(2)
    })

    it('dragging the heading works at any list position', async () => {
        window.history.replaceState({ back: '/library#playing' }, '', '/library#queue')
        const w = mountAtQueue()
        const list = w.find('.stub-list')
        ;(list.element as HTMLElement).scrollTop = 300
        await touch(w, '.queue-heading', 'touchstart', 100)
        await touch(w, '.queue-heading', 'touchmove', 600)
        expect(sheet().position.value).toBeCloseTo(1.375, 5)
        await release(w, '.queue-heading')
        expect(sheet().detent.value).toBe('playing')
        expect(back).toHaveBeenCalledOnce()
    })

    it('the queue surface never travels below the face', async () => {
        const w = mountAtQueue()
        await touch(w, '.queue-heading', 'touchstart', 0)
        await touch(w, '.queue-heading', 'touchmove', 4000)
        expect(sheet().position.value).toBe(1)
    })
})
