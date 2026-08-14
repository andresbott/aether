import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'

const queue = ref<any[]>([])
const currentIndex = ref(0)
const isPlaying = ref(false)

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        queue,
        currentIndex,
        isPlaying,
        playQueueItem: vi.fn(),
        removeFromQueue: vi.fn(),
        removeManyFromQueue: vi.fn(),
        togglePlayPause: vi.fn(),
        insertIntoQueue: vi.fn()
    })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '', getAlbum: vi.fn() }
}))

vi.mock('@/composables/useSubsonicQueries', () => ({
    useToggleStar: () => ({ mutate: vi.fn() })
}))

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

vi.mock('sortablejs', () => ({ default: { create: vi.fn(() => ({ destroy: vi.fn() })) } }))

vi.mock('@/components/library/SongDetail.vue', () => ({
    default: {
        name: 'SongDetail',
        props: ['song', 'card'],
        template: '<div class="stub-song-detail">{{ song.title }}</div>'
    }
}))

import QueueBody from '@/components/layout/QueueBody.vue'

const song = (id: string) => ({
    id,
    title: `Song ${id}`,
    artist: 'Artist',
    album: 'Album',
    duration: 60
})

const mountBody = () =>
    mount(QueueBody, {
        props: { variant: 'sidebar' as const, editMode: false },
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

// jsdom has no layout, so the geometry the scroll math reads is stubbed onto
// the mounted elements. Values are set synchronously after mount(), before the
// nextTick that runs the scroll, so the deferred callback sees them.
const stubGeometry = (
    w: ReturnType<typeof mountBody>,
    opts: { rowTop: number; rowHeight: number; scrollerHeight: number; scrollTop?: number }
) => {
    const scroller = w.find('.queue-body').element as HTMLElement
    const row = w.find('.current-block').element as HTMLElement
    scroller.getBoundingClientRect = () => ({ top: 0 }) as DOMRect
    row.getBoundingClientRect = () => ({ top: opts.rowTop }) as DOMRect
    Object.defineProperty(scroller, 'clientHeight', {
        value: opts.scrollerHeight,
        configurable: true
    })
    Object.defineProperty(row, 'offsetHeight', { value: opts.rowHeight, configurable: true })
    scroller.scrollTop = opts.scrollTop ?? 0
    const scrollTo = vi.fn()
    ;(scroller as unknown as Record<string, unknown>).scrollTo = scrollTo
    return scrollTo
}

let scrollIntoView: ReturnType<typeof vi.fn>

beforeEach(() => {
    queue.value = [song('1'), song('2'), song('3')]
    currentIndex.value = 1
    isPlaying.value = false
    scrollIntoView = vi.fn()
    ;(Element.prototype as unknown as Record<string, unknown>).scrollIntoView = scrollIntoView
})

afterEach(() => {
    delete (Element.prototype as unknown as Record<string, unknown>).scrollIntoView
})

// Revealing the current track must move ONLY the queue's own scroller.
// scrollIntoView also scrolls every scrollable ancestor: inside MobilePlayView
// the queue is the hidden second panel of a snap container, and the ancestor
// scroll dragged it over the player face — a mini-player tap landed on
// `/#queue` instead of Now Playing.
describe('QueueBody current-track scrolling', () => {
    it('centers the current track on mount by scrolling its own scroller only', async () => {
        const w = mountBody()
        const scrollTo = stubGeometry(w, { rowTop: 300, rowHeight: 80, scrollerHeight: 400 })
        await w.vm.$nextTick()
        // rowTop 300 centered in a 400px viewport with an 80px row → 140.
        expect(scrollTo).toHaveBeenCalledWith({ top: 140, behavior: 'smooth' })
        expect(scrollIntoView).not.toHaveBeenCalled()
    })

    it('scrolls minimally when the track advances below the visible area', async () => {
        const w = mountBody()
        await w.vm.$nextTick() // let the mount scroll pass with jsdom's zero rects
        const scrollTo = stubGeometry(w, { rowTop: 500, rowHeight: 80, scrollerHeight: 400 })
        currentIndex.value = 2
        await w.vm.$nextTick()
        await w.vm.$nextTick()
        // Row bottom 580 just clears the 400px viewport → align it to the bottom.
        expect(scrollTo).toHaveBeenCalledWith({ top: 180, behavior: 'smooth' })
        expect(scrollIntoView).not.toHaveBeenCalled()
    })

    it('does not scroll at all while the advancing track is already visible', async () => {
        const w = mountBody()
        await w.vm.$nextTick()
        const scrollTo = stubGeometry(w, { rowTop: 100, rowHeight: 80, scrollerHeight: 400 })
        currentIndex.value = 2
        await w.vm.$nextTick()
        await w.vm.$nextTick()
        expect(scrollTo).not.toHaveBeenCalled()
        expect(scrollIntoView).not.toHaveBeenCalled()
    })
})
