import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { computed, ref } from 'vue'
import type { Album, Playlist } from '@/types/subsonic'

const albums = ref<Album[]>([])
const playlists = ref<Playlist[]>([])
const albumsLoading = ref(false)
const albumsError = ref(false)
const playlistsLoading = ref(false)
const playlistsError = ref(false)
const albumSize = ref(12)

vi.mock('@/composables/useDiscovery', async () => {
    const actual = await vi.importActual<typeof import('@/composables/useDiscovery')>(
        '@/composables/useDiscovery'
    )
    return {
        ...actual,
        // The real composable takes a MaybeRefOrGetter and returns `section` as a
        // ComputedRef — the stub matches that shape.
        useDiscoverySection: (
            key: string | (() => string),
            size: number | (() => number) | { value: number }
        ) => {
            // Extract the size value: could be a number, a function, or a constant
            const sizeValue = typeof size === 'function' ? size() : size
            albumSize.value = typeof sizeValue === 'object' && 'value' in sizeValue ? sizeValue.value : sizeValue
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
                reshuffle: vi.fn()
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
    useTogglePlaylistStar: () => ({ mutate: vi.fn() })
}))

import DiscoverySection from '@/components/library/DiscoverySection.vue'

const album = (id: string): Album => ({ id, name: `Album ${id}` })
const playlist = (id: string): Playlist => ({
    id,
    name: `List ${id}`,
    songCount: 1,
    duration: 60,
    created: '2026-01-01T00:00:00Z'
})

const stubs = { RouterLink: { template: '<a><slot /></a>' } }
const mountSection = (layout: 'grid' | 'list' = 'grid') =>
    mount(DiscoverySection, {
        props: { sectionKey: 'recently-added', layout },
        global: { stubs, directives: { tooltip: {} } }
    })

beforeEach(() => {
    albums.value = []
    playlists.value = []
    albumsLoading.value = false
    albumsError.value = false
    playlistsLoading.value = false
    playlistsError.value = false
    albumSize.value = 12
})

describe('DiscoverySection', () => {
    it('renders the section title and a show-all link', () => {
        albums.value = [album('al-1')]
        const w = mountSection()
        expect(w.find('.section-title').text()).toContain('Recently added')
        expect(w.find('.section-show-all').exists()).toBe(true)
    })

    it('caps albums at 12 and playlists at 6', () => {
        albums.value = Array.from({ length: 20 }, (_, i) => album(`al-${i}`))
        playlists.value = Array.from({ length: 10 }, (_, i) => playlist(`pl-${i}`))
        const w = mountSection()
        expect(w.findAll('.album-card')).toHaveLength(12)
        expect(w.findAll('.playlist-card')).toHaveLength(6)
    })

    it('renders rows instead of cards in list layout', () => {
        albums.value = [album('al-1')]
        playlists.value = [playlist('pl-1')]
        const w = mountSection('list')
        expect(w.find('.album-row').exists()).toBe(true)
        expect(w.find('.album-card').exists()).toBe(false)
    })

    it('shows a loading state while loading', () => {
        albumsLoading.value = true
        playlistsLoading.value = true
        const w = mountSection()
        expect(w.find('.section-loading').exists()).toBe(true)
        expect(w.find('.section-empty').exists()).toBe(false)
    })

    it('shows an error state that is distinct from the empty state', () => {
        albumsError.value = true
        playlistsError.value = true
        const w = mountSection()
        expect(w.find('.section-error').exists()).toBe(true)
        expect(w.find('.section-empty').exists()).toBe(false)
    })

    it('shows an empty state when the section has no content and no error', () => {
        const w = mountSection()
        expect(w.find('.section-empty').exists()).toBe(true)
        expect(w.find('.section-error').exists()).toBe(false)
    })

    it('renders the album block alone when the section has no playlists', () => {
        albums.value = [album('al-1')]
        const w = mountSection()
        expect(w.find('.section-albums').exists()).toBe(true)
        expect(w.find('.section-playlists').exists()).toBe(false)
        expect(w.find('.section-empty').exists()).toBe(false)
    })

    it('renders albums even when the playlist query fails', () => {
        albums.value = [album('al-1'), album('al-2')]
        playlistsError.value = true
        const w = mountSection()
        expect(w.find('.section-albums').exists()).toBe(true)
        expect(w.findAll('.album-card')).toHaveLength(2)
        expect(w.find('.section-playlists-error').exists()).toBe(true)
        expect(w.find('.section-error').exists()).toBe(false)
    })

    // The mirror of the test above, and the case that let a real bug through: an
    // album-only failure with nothing to show in either block must surface the
    // album error, never "Nothing here yet".
    it('shows the album error when albums fail and there are no playlists', () => {
        albumsError.value = true
        const w = mountSection()
        expect(w.find('.section-albums-error').exists()).toBe(true)
        expect(w.find('.section-empty').exists()).toBe(false)
        expect(w.find('.section-error').exists()).toBe(false)
    })

    it('shows the album loading state when albums load and there are no playlists', () => {
        albumsLoading.value = true
        const w = mountSection()
        expect(w.find('.section-albums-loading').exists()).toBe(true)
        expect(w.find('.section-empty').exists()).toBe(false)
    })

    it('passes SHELF_ALBUM_COUNT (12) to the composable', () => {
        mountSection()
        expect(albumSize.value).toBe(12)
    })
})
