import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

const route = { params: { folderId: '1' }, hash: '#albums', query: {} as Record<string, string> }
const replace = vi.fn()
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace })
}))

vi.mock('@/composables/useSubsonicQueries', () => ({
    useMusicFolders: () => ({ data: ref([{ id: 1, name: 'Main' }]) })
}))
vi.mock('@/composables/useAlbumIndex', () => ({
    useAlbumIndex: () => ({ total: ref(1240), letters: ref([]), isLoading: ref(false), error: ref(null) })
}))
vi.mock('@/composables/useArtistTable', () => ({
    useArtistTable: () => ({ total: ref(87), letters: ref([]), items: ref([]), isLoading: ref(false), error: ref(null) })
}))

const AlbumListStub = { name: 'AlbumListView', props: ['folderId'], template: '<div class="album-list-stub" />' }
const AlbumGridStub = { name: 'AlbumGrid', props: ['folderId'], template: '<div class="album-grid-stub" />' }
const ArtistListStub = { name: 'ArtistListView', props: ['folderId'], template: '<div class="artist-list-stub" />' }
const ArtistGridStub = { name: 'ArtistGrid', props: ['folderId'], template: '<div class="artist-grid-stub" />' }

import LibraryView from '@/views/LibraryView.vue'
import SelectButton from 'primevue/selectbutton'

const mountView = () =>
    mount(LibraryView, {
        global: {
            plugins: [PrimeVue],
            stubs: {
                AlbumListView: AlbumListStub,
                AlbumGrid: AlbumGridStub,
                ArtistListView: ArtistListStub,
                ArtistGrid: ArtistGridStub
            }
        }
    })

beforeEach(() => {
    replace.mockReset()
    route.hash = '#albums'
    route.query = {}
})

describe('LibraryView', () => {
    it('albums + default layout → AlbumGrid, and shows the album summary', () => {
        const w = mountView()
        expect(w.findComponent(AlbumGridStub).exists()).toBe(true)
        expect(w.findComponent(AlbumListStub).exists()).toBe(false)
        expect(w.text()).toContain('1240 albums')
    })

    it('albums + list layout → AlbumListView', () => {
        route.query = { view: 'list' }
        const w = mountView()
        expect(w.findComponent(AlbumListStub).exists()).toBe(true)
        expect(w.findComponent(AlbumListStub).props('folderId')).toBe(1)
    })

    it('artists + default layout → ArtistGrid with the artist summary', () => {
        route.hash = '#artists'
        const w = mountView()
        expect(w.findComponent(ArtistGridStub).exists()).toBe(true)
        expect(w.text()).toContain('87 artists')
    })

    it('artists + list layout → ArtistListView', () => {
        route.hash = '#artists'
        route.query = { view: 'list' }
        const w = mountView()
        expect(w.findComponent(ArtistListStub).exists()).toBe(true)
    })

    it('shows the layout toggle on BOTH tabs', () => {
        route.hash = '#albums'
        const albumsView = mountView()
        expect(albumsView.findAllComponents(SelectButton).length).toBe(2)
        route.hash = '#artists'
        const artistsView = mountView()
        expect(artistsView.findAllComponents(SelectButton).length).toBe(2)
    })

    it('toggling layout preserves the hash', async () => {
        const w = mountView()
        w.findAllComponents(SelectButton)[0].vm.$emit('update:modelValue', 'list')
        await w.vm.$nextTick()
        expect(replace).toHaveBeenCalledWith(
            expect.objectContaining({ hash: '#albums', query: expect.objectContaining({ view: 'list' }) })
        )
    })
})
