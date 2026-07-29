import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { computed, ref } from 'vue'
import PrimeVue from 'primevue/config'
import type { Album, Playlist } from '@/types/subsonic'

const backSpy = vi.fn()
const replaceSpy = vi.fn()
const route = { query: {} as Record<string, unknown> }
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ back: backSpy, replace: replaceSpy })
}))

const albums = ref<Album[]>([])
const playlists = ref<Playlist[]>([])
const refetchAlbums = vi.fn()

vi.mock('@/composables/useDiscovery', async () => {
    const actual = await vi.importActual<typeof import('@/composables/useDiscovery')>(
        '@/composables/useDiscovery'
    )
    return {
        ...actual,
        useDiscoverySection: (key: string | (() => string)) => ({
            section: computed(() =>
                actual.findSection(typeof key === 'function' ? key() : key)
            ),
            albums: computed(() => albums.value),
            playlists: computed(() => playlists.value),
            isLoading: computed(() => false),
            isError: computed(() => false),
            refetchAlbums
        })
    }
})

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => false,
        getCoverArtUrl: () => '',
        getPlaylist: vi.fn(),
        scrobble: vi.fn()
    }
}))
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum: vi.fn() }) }))
vi.mock('@/composables/useAlbumDrag', () => ({
    useAlbumDrag: () => ({ start: vi.fn(), end: vi.fn() })
}))
vi.mock('@/composables/useSubsonicQueries', () => ({
    useTogglePlaylistStar: () => ({ mutate: vi.fn() })
}))

import DiscoverySectionView from '@/views/DiscoverySectionView.vue'

const album = (id: string): Album => ({ id, name: `Album ${id}` })
const playlist = (id: string): Playlist => ({
    id,
    name: `List ${id}`,
    songCount: 1,
    duration: 60,
    created: '2026-01-01T00:00:00Z'
})

const stubs = { RouterLink: { template: '<a><slot /></a>' } }
const mountView = (section = 'favorites') =>
    mount(DiscoverySectionView, {
        props: { section },
        global: { plugins: [PrimeVue], directives: { tooltip: {} }, stubs }
    })

beforeEach(() => {
    albums.value = []
    playlists.value = []
    backSpy.mockReset()
    replaceSpy.mockReset()
    refetchAlbums.mockReset()
    route.query = {}
})

describe('DiscoverySectionView', () => {
    it('renders the section title', () => {
        expect(mountView('favorites').text()).toContain('Favorites')
    })

    it('summarises the album and playlist counts, pluralised', () => {
        albums.value = [album('al-1'), album('al-2')]
        playlists.value = [playlist('pl-1')]
        const summary = mountView().find('.scaffold-summary').text()
        expect(summary).toContain('2 albums')
        expect(summary).toContain('1 playlist')
        expect(summary).not.toContain('1 playlists')
    })

    it('omits the summary element entirely when the section is empty', () => {
        expect(mountView().find('.scaffold-summary').exists()).toBe(false)
    })

    it('shows a shuffle button for random and no load-more', () => {
        albums.value = [album('al-1')]
        const w = mountView('random')
        expect(w.find('.section-shuffle').exists()).toBe(true)
        expect(w.find('.section-load-more').exists()).toBe(false)
    })

    it('refetches when shuffle is clicked', async () => {
        albums.value = [album('al-1')]
        const w = mountView('random')
        await w.find('.section-shuffle').trigger('click')
        expect(refetchAlbums).toHaveBeenCalled()
    })

    it('redirects to discovery for an unknown section', () => {
        mountView('nope')
        expect(replaceSpy).toHaveBeenCalledWith({ name: 'discover' })
    })

    it('re-derives data when the section prop changes without remounting', async () => {
        const w = mountView('favorites')
        expect(w.text()).toContain('Favorites')
        expect(w.find('.section-shuffle').exists()).toBe(false)

        await w.setProps({ section: 'random' })
        expect(w.text()).toContain('Random')
        expect(w.find('.section-shuffle').exists()).toBe(true)
    })
})
