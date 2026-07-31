import { describe, it, expect, vi, beforeEach } from 'vitest'
import { computed, ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { DiscoveryFeedEntry } from '@/types/subsonic'

const route = {
    params: { folderId: '1' } as Record<string, string>,
    hash: '#albums',
    query: {} as Record<string, string>
}
const replace = vi.fn()
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace })
}))

const foldersRef = ref<
    Array<{ id: number; name: string; showArtists?: boolean; defaultView?: 'albums' | 'artists' }>
>([{ id: 1, name: 'Main' }])
vi.mock('@/composables/useSubsonicQueries', () => ({
    useMusicFolders: () => ({ data: foldersRef })
}))
const discoveryItems = ref<DiscoveryFeedEntry[]>([])
vi.mock('@/composables/useDiscovery', () => ({
    useDiscoveryFeed: () => ({
        items: computed(() => discoveryItems.value),
        isLoading: ref(false),
        isError: ref(false),
        hasNextPage: ref(false),
        isFetchingNextPage: ref(false),
        fetchNextPage: vi.fn()
    })
}))
vi.mock('@/composables/useAlbumIndex', () => ({
    useAlbumIndex: () => ({
        total: ref(1240),
        letters: ref([]),
        isLoading: ref(false),
        error: ref(null)
    })
}))
vi.mock('@/composables/useArtistTable', () => ({
    useArtistTable: () => ({
        total: ref(87),
        letters: ref([]),
        items: ref([]),
        isLoading: ref(false),
        error: ref(null)
    })
}))

const AlbumListStub = {
    name: 'AlbumListView',
    props: ['folderId'],
    template: '<div class="album-list-stub" />'
}
const AlbumGridStub = {
    name: 'AlbumGrid',
    props: ['folderId'],
    template: '<div class="album-grid-stub" />'
}
const ArtistListStub = {
    name: 'ArtistListView',
    props: ['folderId'],
    template: '<div class="artist-list-stub" />'
}
const ArtistGridStub = {
    name: 'ArtistGrid',
    props: ['folderId'],
    template: '<div class="artist-grid-stub" />'
}
const DiscoveryFeedStub = {
    name: 'DiscoveryFeed',
    props: ['layout'],
    template: '<div class="discovery-feed-stub" :data-layout="layout" />'
}

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
                ArtistGrid: ArtistGridStub,
                DiscoveryFeed: DiscoveryFeedStub
            }
        }
    })

const albumEntry = (rank: number): DiscoveryFeedEntry => ({
    type: 'album',
    rank,
    reason: 'favorite',
    album: { id: `al-${rank}`, name: `Album ${rank}`, rank, reason: 'favorite' }
})

beforeEach(() => {
    replace.mockReset()
    route.params = { folderId: '1' }
    route.hash = '#albums'
    route.query = {}
    foldersRef.value = [{ id: 1, name: 'Main' }]
    discoveryItems.value = []
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
        expect(w.findComponent(ArtistGridStub).props('folderId')).toBe(1)
        expect(w.text()).toContain('87 artists')
    })

    it('artists + list layout → ArtistListView', () => {
        route.hash = '#artists'
        route.query = { view: 'list' }
        const w = mountView()
        expect(w.findComponent(ArtistListStub).exists()).toBe(true)
        expect(w.findComponent(ArtistListStub).props('folderId')).toBe(1)
    })

    it('shows the layout toggle on every tab', () => {
        route.hash = '#albums'
        const albumsView = mountView()
        expect(albumsView.findAllComponents(SelectButton).length).toBe(2)
        route.hash = '#artists'
        const artistsView = mountView()
        expect(artistsView.findAllComponents(SelectButton).length).toBe(2)
        route.params = {}
        route.hash = '#discover'
        const discoverView = mountView()
        expect(discoverView.findAllComponents(SelectButton).length).toBe(2)
    })

    it('toggling layout preserves the hash', async () => {
        const w = mountView()
        w.findAllComponents(SelectButton)[0].vm.$emit('update:modelValue', 'list')
        await w.vm.$nextTick()
        expect(replace).toHaveBeenCalledWith(
            expect.objectContaining({
                hash: '#albums',
                query: expect.objectContaining({ view: 'list' })
            })
        )
    })

    it('hides the Artists tab and forces albums when the folder has showArtists=false', () => {
        foldersRef.value = [{ id: 1, name: 'Main', showArtists: false }]
        route.hash = '#artists'
        const w = mountView()
        expect(w.findComponent(AlbumGridStub).exists()).toBe(true)
        expect(w.findComponent(ArtistGridStub).exists()).toBe(false)
        // Only the layout toggle remains.
        expect(w.findAllComponents(SelectButton).length).toBe(1)
    })

    it('keeps the Artists tab when showArtists is true or unset', () => {
        foldersRef.value = [{ id: 1, name: 'Main', showArtists: true }]
        const w = mountView()
        expect(w.findAllComponents(SelectButton).length).toBe(2)
    })
})

// All Music (no folderId) is the only route that offers Discovery: the ranking is
// cross-collection, so a per-library feed would be meaningless.
describe('LibraryView Discover tab', () => {
    beforeEach(() => {
        route.params = {}
        route.hash = ''
    })

    it('defaults to the Discover feed on All Music', () => {
        const w = mountView()
        expect(w.findComponent(DiscoveryFeedStub).exists()).toBe(true)
        expect(w.findComponent(AlbumGridStub).exists()).toBe(false)
    })

    it('offers Discover, Albums and Artists on All Music', () => {
        const w = mountView()
        const tabs = w.findAllComponents(SelectButton)[1]
        expect(tabs.props('options')).toEqual([
            { label: 'Discover', value: 'discover' },
            { label: 'Albums', value: 'albums' },
            { label: 'Artists', value: 'artists' }
        ])
    })

    it('passes the layout through to the feed', () => {
        expect(mountView().findComponent(DiscoveryFeedStub).props('layout')).toBe('grid')
        route.query = { view: 'list' }
        expect(mountView().findComponent(DiscoveryFeedStub).props('layout')).toBe('list')
    })

    it('summarises the feed item count, pluralised', () => {
        discoveryItems.value = [albumEntry(0), albumEntry(1)]
        expect(mountView().text()).toContain('2 items')
        discoveryItems.value = [albumEntry(0)]
        const single = mountView().text()
        expect(single).toContain('1 item')
        expect(single).not.toContain('1 items')
    })

    it('omits the summary when the feed is empty', () => {
        expect(mountView().text()).not.toContain('0 item')
    })

    it('leaves Discover for Albums when the hash says albums', () => {
        route.hash = '#albums'
        const w = mountView()
        expect(w.findComponent(AlbumGridStub).exists()).toBe(true)
        expect(w.findComponent(DiscoveryFeedStub).exists()).toBe(false)
    })

    it('omits the Discover tab inside a single library', () => {
        route.params = { folderId: '1' }
        const tabs = mountView().findAllComponents(SelectButton)[1]
        expect(tabs.props('options')).toEqual([
            { label: 'Albums', value: 'albums' },
            { label: 'Artists', value: 'artists' }
        ])
    })

    // A folder deep-linked to #discover must not land on a tab its toggle cannot
    // reach — it falls back to that folder's albums.
    it('falls back to albums when a folder is deep-linked to #discover', () => {
        route.params = { folderId: '1' }
        route.hash = '#discover'
        const w = mountView()
        expect(w.findComponent(AlbumGridStub).exists()).toBe(true)
        expect(w.findComponent(DiscoveryFeedStub).exists()).toBe(false)
    })

    // Discovery is not a per-library setting, so a folder's default_view still
    // decides where that folder opens.
    it('honours a folder default_view of artists', () => {
        route.params = { folderId: '1' }
        route.hash = ''
        foldersRef.value = [{ id: 1, name: 'Main', defaultView: 'artists' }]
        expect(mountView().findComponent(ArtistGridStub).exists()).toBe(true)
    })
})
