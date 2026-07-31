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
const albumsLoading = ref(false)
const albumsError = ref(false)
const playlistsLoading = ref(false)
const playlistsError = ref(false)
const reshuffle = vi.fn()
const albumSize = ref(100)

vi.mock('@/composables/useDiscovery', async () => {
    const actual = await vi.importActual<typeof import('@/composables/useDiscovery')>(
        '@/composables/useDiscovery'
    )
    return {
        ...actual,
        useDiscoverySection: (
            key: string | (() => string),
            size: number | (() => number) | { value: number }
        ) => {
            // Extract the size value: could be a number, a function, or a computed ref
            const sizeValue =
                typeof size === 'function'
                    ? size()
                    : typeof size === 'object' && 'value' in size
                      ? size.value
                      : size
            albumSize.value = sizeValue
            return {
                section: computed(() =>
                    actual.findSection(typeof key === 'function' ? key() : key)
                ),
                albums: computed(() => albums.value),
                playlists: computed(() => playlists.value),
                albumsLoading: computed(() => albumsLoading.value),
                albumsError: computed(() => albumsError.value),
                playlistsLoading: computed(() => playlistsLoading.value),
                playlistsError: computed(() => playlistsError.value),
                reshuffle
            }
        }
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
    useTogglePlaylistStar: () => ({ mutate: vi.fn() }),
    // AlbumCard's favorite toggle; this spec mounts the real card.
    useToggleStar: () => ({ mutate: vi.fn() })
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
    albumsLoading.value = false
    albumsError.value = false
    playlistsLoading.value = false
    playlistsError.value = false
    backSpy.mockReset()
    replaceSpy.mockReset()
    reshuffle.mockReset()
    route.query = {}
    albumSize.value = 100
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

    it('reshuffles when shuffle is clicked', async () => {
        albums.value = [album('al-1')]
        const w = mountView('random')
        await w.find('.section-shuffle').trigger('click')
        expect(reshuffle).toHaveBeenCalled()
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

    it('passes SECTION_PAGE_ALBUM_COUNT (100) for non-random sections', () => {
        mountView('favorites')
        expect(albumSize.value).toBe(100)
    })

    it('passes RANDOM_PAGE_ALBUM_COUNT (200) for the random section', () => {
        mountView('random')
        expect(albumSize.value).toBe(200)
    })

    it('renders albums even when the playlist query fails', () => {
        albums.value = [album('al-1'), album('al-2')]
        playlistsError.value = true
        const w = mountView('favorites')
        expect(w.findAll('.album-card')).toHaveLength(2)
        expect(w.text()).toContain('Could not load playlists')
        expect(w.text()).not.toContain('Could not load this section')
    })
})
