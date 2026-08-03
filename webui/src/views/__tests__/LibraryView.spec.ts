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
const starredAlbumTotal = ref(4)
const starredArtistTotal = ref(2)
vi.mock('@/composables/useStarred', () => ({
    useStarredAlbums: () => ({
        total: computed(() => starredAlbumTotal.value),
        letters: ref([]),
        items: ref([]),
        isLoading: ref(false),
        error: ref(null)
    }),
    useStarredArtists: () => ({
        total: computed(() => starredArtistTotal.value),
        letters: ref([]),
        items: ref([]),
        isLoading: ref(false),
        error: ref(null)
    })
}))

const AlbumListStub = {
    name: 'AlbumListView',
    props: ['folderId', 'favoritesOnly'],
    template: '<div class="album-list-stub" />'
}
const AlbumGridStub = {
    name: 'AlbumGrid',
    props: ['folderId', 'favoritesOnly'],
    template: '<div class="album-grid-stub" />'
}
const ArtistListStub = {
    name: 'ArtistListView',
    props: ['folderId', 'favoritesOnly'],
    template: '<div class="artist-list-stub" />'
}
const ArtistGridStub = {
    name: 'ArtistGrid',
    props: ['folderId', 'favoritesOnly'],
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
            directives: { tooltip: {} },
            stubs: {
                AlbumListView: AlbumListStub,
                AlbumGrid: AlbumGridStub,
                ArtistListView: ArtistListStub,
                ArtistGrid: ArtistGridStub,
                DiscoveryFeed: DiscoveryFeedStub
            }
        }
    })

// PrimeVue's SelectButton renders a ToggleButton per option, so finding by
// component type would match the layout/tab toggles too — target the stable hook
// class, and drive it with a real click rather than a synthetic emit.
const favoritesToggle = (w: ReturnType<typeof mountView>) =>
    w.find('.library-favorites-filter')

// PrimeVue renders the on/off icon as a span, not an <i>.
const favoriteIcon = (w: ReturnType<typeof mountView>) =>
    favoritesToggle(w).find('.p-togglebutton-icon')

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
    starredAlbumTotal.value = 4
    starredArtistTotal.value = 2
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

// The favorites filter is URL state (?favorites=1) like the layout, so it survives
// a reload and is linkable, and it applies to Albums and Artists but not Discover.
describe('LibraryView favorites filter', () => {
    it('is off by default and passes favoritesOnly=false to the body', () => {
        const w = mountView()
        // Outline heart while off, filled while on — the app-wide favorites signal.
        expect(favoriteIcon(w).classes()).toContain('pi-heart')
        expect(favoriteIcon(w).classes()).not.toContain('pi-heart-fill')
        expect(favoritesToggle(w).attributes('aria-pressed')).toBe('false')
        expect(w.findComponent(AlbumGridStub).props('favoritesOnly')).toBe(false)
    })

    it('fills the heart while the filter is on', () => {
        route.query = { favorites: '1' }
        const w = mountView()
        expect(favoriteIcon(w).classes()).toContain('pi-heart-fill')
        expect(favoritesToggle(w).attributes('aria-pressed')).toBe('true')
    })

    it('labels the toggle by what clicking it will do', () => {
        expect(favoritesToggle(mountView()).attributes('aria-label')).toBe('Show favorites only')
        route.query = { favorites: '1' }
        expect(favoritesToggle(mountView()).attributes('aria-label')).toBe('Show all')
    })

    it('reads ?favorites=1 and passes it to whichever body is active', () => {
        route.query = { favorites: '1' }
        expect(mountView().findComponent(AlbumGridStub).props('favoritesOnly')).toBe(true)

        route.query = { favorites: '1', view: 'list' }
        expect(mountView().findComponent(AlbumListStub).props('favoritesOnly')).toBe(true)

        route.hash = '#artists'
        route.query = { favorites: '1' }
        expect(mountView().findComponent(ArtistGridStub).props('favoritesOnly')).toBe(true)

        route.query = { favorites: '1', view: 'list' }
        expect(mountView().findComponent(ArtistListStub).props('favoritesOnly')).toBe(true)
    })

    it('writes ?favorites=1 on enable and drops the key on disable, keeping the hash', async () => {
        const w = mountView()
        await favoritesToggle(w).trigger('click')
        expect(replace).toHaveBeenCalledWith({
            hash: '#albums',
            query: { favorites: '1' }
        })

        replace.mockReset()
        route.query = { favorites: '1' }
        const on = mountView()
        await favoritesToggle(on).trigger('click')
        expect(replace).toHaveBeenCalledWith({ hash: '#albums', query: {} })
    })

    it('preserves the layout when the filter is toggled', async () => {
        route.query = { view: 'list' }
        const w = mountView()
        await favoritesToggle(w).trigger('click')
        expect(replace).toHaveBeenCalledWith({
            hash: '#albums',
            query: { view: 'list', favorites: '1' }
        })
    })

    // "6 favorites", not "6 favorite albums": the active tab names the type, and the
    // root header has no room for the longer form.
    it('summarises the favorites count, pluralised', () => {
        route.query = { favorites: '1' }
        const albums = mountView().text()
        expect(albums).toContain('4 favorites')
        // Never the unfiltered library count while filtered.
        expect(albums).not.toContain('1240')

        starredAlbumTotal.value = 1
        const single = mountView().text()
        expect(single).toContain('1 favorite')
        expect(single).not.toContain('1 favorites')

        route.hash = '#artists'
        const artists = mountView().text()
        expect(artists).toContain('2 favorites')
        expect(artists).not.toContain('87')
    })

    it('omits the summary when nothing is favorited', () => {
        route.query = { favorites: '1' }
        starredAlbumTotal.value = 0
        const text = mountView().text()
        expect(text).not.toContain('0 favorite')
        // And it must not fall back to the unfiltered library count.
        expect(text).not.toContain('1240')
    })

    it('hides the filter on Discover and ignores a stale ?favorites=1 there', () => {
        route.params = {}
        route.hash = '#discover'
        route.query = { favorites: '1' }
        const w = mountView()
        expect(favoritesToggle(w).exists()).toBe(false)
        // The feed is unfiltered, so the count is the feed's, not a favorites count.
        expect(w.findComponent(DiscoveryFeedStub).exists()).toBe(true)
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
