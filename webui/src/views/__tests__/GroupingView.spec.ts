import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

const grouping = ref<any>(null)
const isLoading = ref(false)
const error = ref<any>(null)

vi.mock('@/composables/useSubsonicQueries', () => ({
    useGrouping: () => ({ data: grouping, isLoading, error })
}))

const getAlbum = vi.fn((id: string) =>
    Promise.resolve(
        (
            {
                'al-1': { id: 'al-1', song: [{ id: 's1' }, { id: 's2' }] },
                'al-2': { id: 'al-2', song: [{ id: 's3' }] }
            } as Record<string, unknown>
        )[id] ?? null
    )
)
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size?: number) => `/cover/${id}?size=${size}`,
        getAlbum: (id: string) => getAlbum(id)
    }
}))

const playAlbum = vi.fn()
const addMultipleToQueue = vi.fn()
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ playAlbum, addMultipleToQueue })
}))
vi.mock('vue-router', () => ({ useRouter: () => ({ back: vi.fn() }) }))

const ScaffoldStub = {
    name: 'ContentScaffold',
    props: ['title'],
    template: '<div class="scaffold"><slot /></div>'
}
const AlbumCardStub = {
    name: 'AlbumCard',
    props: ['album'],
    template: '<div class="album-card-stub" :data-id="album.id" />'
}

import GroupingView from '@/views/GroupingView.vue'

const mountView = () =>
    mount(GroupingView, {
        props: { id: 'gr-1' },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { ContentScaffold: ScaffoldStub, AlbumCard: AlbumCardStub, RouterLink: true }
        }
    })

beforeEach(() => {
    grouping.value = {
        id: 'gr-1',
        name: 'Doctor Who',
        coverArt: 'gr-1',
        albumCount: 2,
        songCount: 3,
        year: 2005,
        album: [
            { id: 'al-1', name: 'Series 1' },
            { id: 'al-2', name: 'Series 2' }
        ]
    }
    isLoading.value = false
    error.value = null
    playAlbum.mockClear()
    addMultipleToQueue.mockClear()
})

describe('GroupingView', () => {
    it('shows the name, the summary and one card per album in the given order', () => {
        const w = mountView()
        expect(w.text()).toContain('Doctor Who')
        expect(w.text()).toContain('2 albums')
        expect(w.text()).toContain('3 songs')
        expect(w.text()).toContain('2005')
        expect(w.findAll('.album-card-stub').map((c) => c.attributes('data-id'))).toEqual([
            'al-1',
            'al-2'
        ])
    })

    it('plays and queues every album front to back', async () => {
        const w = mountView()
        await w.find('.hero-action-play').trigger('click')
        await flushPromises()
        expect(playAlbum).toHaveBeenCalledWith([{ id: 's1' }, { id: 's2' }, { id: 's3' }])

        await w.find('.hero-action-queue').trigger('click')
        await flushPromises()
        expect(addMultipleToQueue).toHaveBeenCalledWith([{ id: 's1' }, { id: 's2' }, { id: 's3' }])
    })

    it('says so when the grouping cannot be loaded', () => {
        grouping.value = null
        expect(mountView().text()).toContain('Collection not found')
    })
})
