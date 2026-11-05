import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
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
import { useAlbumDragData } from '@/composables/albumDragData'

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
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { RouterLink: RouterLinkStub }
        }
    })

describe('QueueBody append zone', () => {
    beforeEach(() => {
        queue.value = [song('a'), song('b'), song('c')]
        currentIndex.value = 0
        useAlbumDragData().clearAlbumDrag()
    })

    it('is absent until a drag into the queue starts, then shows with its count', async () => {
        const w = mountBody()
        expect(w.find('.queue-drop-end').exists()).toBe(false)
        useAlbumDragData().setAlbumDrag({ albumId: 'al1', albumName: 'LP', count: 7 })
        await w.vm.$nextTick()
        const zone = w.find('.queue-drop-end')
        expect(zone.exists()).toBe(true)
        expect(zone.text()).toContain('Add to end of queue')
        expect(zone.find('.queue-drop-end__count').text()).toBe('+7')
        // It is the last thing in the scroller, so sticky bottom pins it there.
        const scroller = w.find('.queue-body').element
        expect(scroller.lastElementChild).toBe(zone.element)
        useAlbumDragData().clearAlbumDrag()
        await w.vm.$nextTick()
        expect(w.find('.queue-drop-end').exists()).toBe(false)
    })

    it('is not offered on an empty queue, whose whole body is already the target', async () => {
        queue.value = []
        const w = mountBody()
        useAlbumDragData().setAlbumDrag({ albumId: 'al1', albumName: 'LP', count: 2 })
        await w.vm.$nextTick()
        expect(w.find('.queue-empty--drop-active').exists()).toBe(true)
        expect(w.find('.queue-drop-end').exists()).toBe(false)
    })
})
