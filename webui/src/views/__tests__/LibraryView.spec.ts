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
    useAlbumList: () => ({ data: ref([]), isLoading: ref(false) }),
    useArtists: () => ({ data: ref([]), isLoading: ref(false) }),
    useMusicFolders: () => ({ data: ref([{ id: 1, name: 'Main' }]) })
}))

const AlbumListViewStub = { name: 'AlbumListView', props: ['folderId'], template: '<div class="album-list-stub" />' }

import LibraryView from '@/views/LibraryView.vue'

const mountView = () =>
    mount(LibraryView, {
        global: {
            plugins: [PrimeVue],
            stubs: {
                AlbumListView: AlbumListViewStub,
                AlbumCard: true,
                ArtistCard: true,
                RouterLink: true
            }
        }
    })

beforeEach(() => {
    replace.mockReset()
    route.hash = '#albums'
    route.query = {}
})

describe('LibraryView album layout toggle', () => {
    it('shows the card grid by default (no list view)', () => {
        const w = mountView()
        expect(w.find('.album-grid').exists()).toBe(true)
        expect(w.findComponent(AlbumListViewStub).exists()).toBe(false)
    })

    it('renders AlbumListView when query.view is list', () => {
        route.query = { view: 'list' }
        const w = mountView()
        expect(w.findComponent(AlbumListViewStub).exists()).toBe(true)
        expect(w.findComponent(AlbumListViewStub).props('folderId')).toBe(1)
    })
})
