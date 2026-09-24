import { describe, it, expect, vi, beforeEach } from 'vitest'
import { computed, ref, toValue } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { setActivePinia, createPinia } from 'pinia'
import PrimeVue from 'primevue/config'
import type { CatalogView, DiscoveryFeedEntry, MusicFolder, ReleaseTypeCount } from '@/types/subsonic'
import { useUiStore } from '@/store/uiStore'

// The browse mode is a path segment (route.params.mode), not a URL hash:
// /library (discover), /library/releases, /library/5/artists.
const route = {
    params: { folderId: '1', mode: 'releases' } as Record<string, string>,
    hash: '',
    query: {} as Record<string, string>
}
const replace = vi.fn()
const push = vi.fn()
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace, push })
}))

// The default library offers Artists and Releases and opens on Releases.
const mainLibrary = (): MusicFolder => ({
    id: 1,
    name: 'Main',
    views: ['artists', 'releases'],
    defaultView: 'releases'
})
// undefined = the music folders are still loading.
const foldersRef = ref<MusicFolder[] | undefined>([mainLibrary()])
const catalogRef = ref<CatalogView | undefined>(undefined)
// undefined = not fetched yet (or failed): the view then offers every type.
const releaseTypesRef = ref<ReleaseTypeCount[] | undefined>(undefined)
let releaseTypesFolder: MaybeRefOrGetter<number | undefined> = undefined
vi.mock('@/composables/useSubsonicQueries', () => ({
    useMusicFolders: () => ({ data: foldersRef }),
    useCatalogView: () => ({ data: catalogRef }),
    useReleaseTypes: (folderId: MaybeRefOrGetter<number | undefined>) => {
        releaseTypesFolder = folderId
        return { data: releaseTypesRef }
    }
}))
const discoveryItems = ref<DiscoveryFeedEntry[]>([])
// What LibraryView asked the feed for: the folder, and whether to fetch at all.
let discoveryFolder: MaybeRefOrGetter<number | undefined> = undefined
let discoveryEnabled: MaybeRefOrGetter<boolean> | undefined = undefined
vi.mock('@/composables/useDiscovery', () => ({
    useDiscoveryFeed: (
        folder?: MaybeRefOrGetter<number | undefined>,
        options?: { enabled?: MaybeRefOrGetter<boolean> }
    ) => {
        discoveryFolder = folder
        discoveryEnabled = options?.enabled
        return {
            items: computed(() => discoveryItems.value),
            isLoading: ref(false),
            isError: ref(false),
            hasNextPage: ref(false),
            isFetchingNextPage: ref(false),
            fetchNextPage: vi.fn()
        }
    }
}))
// The desktop shell has the sidebar to switch the root's views; the mobile one
// does not.
const shellRef = ref<'desktop' | 'mobile'>('desktop')
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({ shell: shellRef, tier: computed(() => 'desktop'), isTouch: ref(false) })
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
    props: ['folderId', 'favoritesOnly', 'releaseType'],
    template: '<div class="album-list-stub" />'
}
const AlbumGridStub = {
    name: 'AlbumGrid',
    props: ['folderId', 'favoritesOnly', 'releaseType'],
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
    props: ['layout', 'folderId'],
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
                DiscoveryFeed: DiscoveryFeedStub,
            }
        }
    })

// The view switcher: its SelectButton, told apart by its hook class.
const viewSwitcher = (w: ReturnType<typeof mountView>) =>
    w.findAllComponents(SelectButton).find((sb) => sb.classes().includes('library-view-switch'))

// The views it offers and the one selected, or null when it is not rendered.
const viewSwitch = (w: ReturnType<typeof mountView>) => {
    const sb = viewSwitcher(w)
    if (!sb) return null
    return {
        views: (sb.props('options') as Array<{ value: string }>).map((o) => o.value),
        selected: sb.props('modelValue')
    }
}

const chooseView = async (w: ReturnType<typeof mountView>, view: string) => {
    viewSwitcher(w)!.vm.$emit('update:modelValue', view)
    await w.vm.$nextTick()
}

// Every SelectButton but the view switcher: the filters and the layout toggle.
const filterSelects = (w: ReturnType<typeof mountView>) =>
    w.findAllComponents(SelectButton).filter((sb) => !sb.classes().includes('library-view-switch'))

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
    setActivePinia(createPinia())
    replace.mockReset()
    push.mockReset()
    route.params = { folderId: '1', mode: 'releases' }
    route.hash = ''
    route.query = {}
    foldersRef.value = [mainLibrary()]
    releaseTypesRef.value = undefined
    releaseTypesFolder = undefined
    discoveryItems.value = []
    discoveryFolder = undefined
    discoveryEnabled = undefined
    shellRef.value = 'desktop'
    catalogRef.value = undefined
    starredAlbumTotal.value = 4
    starredArtistTotal.value = 2
})

describe('LibraryView', () => {
    it('releases + default layout → AlbumGrid, and shows the release summary', () => {
        const w = mountView()
        expect(w.findComponent(AlbumGridStub).exists()).toBe(true)
        expect(w.findComponent(AlbumListStub).exists()).toBe(false)
        expect(w.text()).toContain('1240 releases')
    })

    it('releases + list layout → AlbumListView', () => {
        const ui = useUiStore()
        ui.setLibraryViewMode('releases', 'list')
        const w = mountView()
        expect(w.findComponent(AlbumListStub).exists()).toBe(true)
        expect(w.findComponent(AlbumListStub).props('folderId')).toBe(1)
    })

    it('artists + default layout → ArtistGrid with the artist summary', () => {
        route.params = { folderId: '1', mode: 'artists' }
        const w = mountView()
        expect(w.findComponent(ArtistGridStub).exists()).toBe(true)
        expect(w.findComponent(ArtistGridStub).props('folderId')).toBe(1)
        expect(w.text()).toContain('87 artists')
    })

    it('artists + list layout → ArtistListView', () => {
        route.params = { folderId: '1', mode: 'artists' }
        const ui = useUiStore()
        ui.setLibraryViewMode('artists', 'list')
        const w = mountView()
        expect(w.findComponent(ArtistListStub).exists()).toBe(true)
        expect(w.findComponent(ArtistListStub).props('folderId')).toBe(1)
    })

    it('shows the layout toggle on every tab', () => {
        route.params = { folderId: '1', mode: 'releases' }
        const releasesView = mountView()
        // Layout toggle + release-type tabs (releases mode, favorites off).
        expect(filterSelects(releasesView).length).toBe(2)
        route.params = { folderId: '1', mode: 'artists' }
        const artistsView = mountView()
        expect(filterSelects(artistsView).length).toBe(1)
        route.params = {}
        const discoverView = mountView()
        expect(filterSelects(discoverView).length).toBe(1)
    })

    it('toggling layout only affects the current type', async () => {
        const ui = useUiStore()
        const w = mountView()
        // SelectButton[0] is the release-type tabs, [1] is the layout toggle.
        filterSelects(w)[1].vm.$emit('update:modelValue', 'list')
        await w.vm.$nextTick()
        expect(ui.getLibraryViewMode('releases')).toBe('list')
        expect(ui.getLibraryViewMode('artists')).toBe('grid')
        expect(ui.getLibraryViewMode('discover')).toBe('grid')
    })

    it('switching tabs shows each type own layout mode', async () => {
        const ui = useUiStore()
        ui.setLibraryViewMode('releases', 'list')
        ui.setLibraryViewMode('artists', 'grid')

        route.params = { folderId: '1', mode: 'releases' }
        const releasesView = mountView()
        expect(releasesView.findComponent(AlbumListStub).exists()).toBe(true)

        route.params = { folderId: '1', mode: 'artists' }
        const artistsView = mountView()
        expect(artistsView.findComponent(ArtistGridStub).exists()).toBe(true)
    })

    it('shows the landing view instead of one the library does not offer', () => {
        foldersRef.value = [{ id: 1, name: 'Main', views: ['releases'], defaultView: 'releases' }]
        route.params = { folderId: '1', mode: 'artists' }
        const w = mountView()
        expect(w.findComponent(AlbumGridStub).exists()).toBe(true)
        expect(w.findComponent(ArtistGridStub).exists()).toBe(false)
        // Falls back to Releases: release-type tabs + layout toggle.
        expect(filterSelects(w).length).toBe(2)
    })
})

// A library's views come from the musicFolderViews extension; the view shows the
// one the path names when the library offers it, else the one it opens on.
describe('LibraryView library views', () => {
    it("opens a library's bare path on its default view", () => {
        route.params = { folderId: '1' }
        foldersRef.value = [{ ...mainLibrary(), defaultView: 'artists' }]
        expect(mountView().findComponent(ArtistGridStub).exists()).toBe(true)
    })

    it('opens on the first view when the default is not one of them', () => {
        route.params = { folderId: '1' }
        foldersRef.value = [{ id: 1, name: 'Main', views: ['artists', 'releases'], defaultView: 'discover' }]
        expect(mountView().findComponent(ArtistGridStub).exists()).toBe(true)
    })

    it("shows a library's own Discover feed", () => {
        route.params = { folderId: '1', mode: 'discover' }
        foldersRef.value = [{ id: 1, name: 'Main', views: ['discover', 'releases'], defaultView: 'releases' }]
        const w = mountView()
        expect(w.findComponent(DiscoveryFeedStub).props('folderId')).toBe(1)
        // The header's count reads the same, folder-scoped feed.
        expect(toValue(discoveryFolder)).toBe(1)
        expect(toValue(discoveryEnabled)).toBe(true)
    })

    it('leaves the Discover feed unfetched while another view is shown', () => {
        mountView()
        expect(toValue(discoveryEnabled)).toBe(false)
    })

    it('rewrites a view the library does not offer to its bare path, keeping the query', () => {
        route.params = { folderId: '1', mode: 'discover' }
        route.query = { favorites: '1' }
        mountView()
        expect(replace).toHaveBeenCalledWith({
            name: 'library-folder',
            params: { folderId: '1' },
            query: { favorites: '1' }
        })
    })

    it('leaves the path alone for a view the library offers', () => {
        route.params = { folderId: '1', mode: 'artists' }
        mountView()
        expect(replace).not.toHaveBeenCalled()
    })

    // Before the folders arrive every view looks unoffered: nothing is rewritten,
    // and no body is mounted to fetch a list the real view might not show.
    it('waits for the music folders before rendering or rewriting anything', () => {
        foldersRef.value = undefined
        route.params = { folderId: '1', mode: 'discover' }
        const w = mountView()
        expect(replace).not.toHaveBeenCalled()
        expect(w.findComponent(DiscoveryFeedStub).exists()).toBe(false)
        expect(w.findComponent(AlbumGridStub).exists()).toBe(false)
        expect(toValue(discoveryEnabled)).toBe(false)
    })

    // An id that names no library answers empty lists on /rest; the view shows
    // its empty release list rather than guessing another view.
    it('shows the empty release list of an id that names no library', () => {
        route.params = { folderId: '9' }
        const w = mountView()
        expect(w.findComponent(AlbumGridStub).props('folderId')).toBe(9)
        expect(replace).not.toHaveBeenCalled()
    })
})

// The view switcher is a button group in the header, shown wherever the
// sidebar is not already switching views. Choosing a view navigates to its
// canonical path: the landing view is the bare path, every other a segment.
describe('LibraryView view switcher', () => {
    it("lists a library's views in display order and marks the one shown", () => {
        route.params = { folderId: '1', mode: 'artists' }
        foldersRef.value = [
            { id: 1, name: 'Main', views: ['discover', 'artists', 'releases'], defaultView: 'releases' }
        ]
        expect(viewSwitch(mountView())).toEqual({
            views: ['discover', 'artists', 'releases'],
            selected: 'artists'
        })
    })

    it('navigates to the chosen view, the landing one at the bare path', async () => {
        route.params = { folderId: '1', mode: 'artists' }
        foldersRef.value = [
            { id: 1, name: 'Main', views: ['discover', 'artists', 'releases'], defaultView: 'releases' }
        ]
        const w = mountView()
        await chooseView(w, 'discover')
        expect(push).toHaveBeenLastCalledWith('/library/1/discover')
        await chooseView(w, 'releases')
        expect(push).toHaveBeenLastCalledWith('/library/1')
    })

    it('marks the landing view on the bare path', () => {
        route.params = { folderId: '1' }
        expect(viewSwitch(mountView())).toEqual({ views: ['artists', 'releases'], selected: 'releases' })
    })

    // A cold load of a library's URL mounts before the music folders arrive: the
    // switcher, which depends on them, must appear when they do.
    it('shows the switcher once the music folders arrive after mount', async () => {
        foldersRef.value = undefined
        route.params = { folderId: '1' }
        const w = mountView()
        expect(viewSwitch(w)).toBeNull()

        foldersRef.value = [mainLibrary()]
        await flushPromises()
        expect(viewSwitch(w)?.views).toEqual(['artists', 'releases'])
    })

    it('shows no switcher in a library with a single view', () => {
        foldersRef.value = [{ id: 1, name: 'Main', views: ['releases'], defaultView: 'releases' }]
        expect(viewSwitch(mountView())).toBeNull()
    })

    it('leaves the root to the sidebar on the desktop shell', () => {
        route.params = {}
        expect(viewSwitch(mountView())).toBeNull()
    })

    // A root set to one sidebar entry has only its page to switch views with.
    it('switches the root views itself when the root has one sidebar entry', () => {
        route.params = { mode: 'artists' }
        catalogRef.value = { splitViews: false }
        expect(viewSwitch(mountView())).toEqual({ views: ['discover', 'artists', 'releases'], selected: 'artists' })
    })

    // A split library has a sidebar entry per view, so it needs no switcher —
    // except on the phone, which has no sidebar.
    it('leaves a split library to the sidebar on the desktop shell', async () => {
        route.params = { folderId: '1' }
        foldersRef.value = [{ ...mainLibrary(), splitViews: true }]
        expect(viewSwitch(mountView())).toBeNull()

        shellRef.value = 'mobile'
        expect(viewSwitch(mountView())?.views).toEqual(['artists', 'releases'])
    })

    // Phones have no sidebar, so without the switcher the root's Artists and
    // Releases views could not be reached.
    it('switches the root views itself on the mobile shell', async () => {
        shellRef.value = 'mobile'
        route.params = { mode: 'releases' }
        const w = mountView()
        expect(viewSwitch(w)).toEqual({ views: ['discover', 'artists', 'releases'], selected: 'releases' })
        await chooseView(w, 'discover')
        expect(push).toHaveBeenLastCalledWith('/library')
        await chooseView(w, 'artists')
        expect(push).toHaveBeenLastCalledWith('/library/artists')
    })
})

// The favorites filter is URL state (?favorites=1) like the layout, so it survives
// a reload and is linkable, and it applies to Releases and Artists but not Discover.
describe('LibraryView favorites filter', () => {
    it('is off by default and passes favoritesOnly=false to the body', () => {
        const w = mountView()
        // Outline heart while off, filled while on — the app-wide favorites signal.
        expect(favoriteIcon(w).classes()).toContain('mso-favorite')
        expect(favoriteIcon(w).classes()).not.toContain('ms-favorite')
        expect(favoritesToggle(w).attributes('aria-pressed')).toBe('false')
        expect(w.findComponent(AlbumGridStub).props('favoritesOnly')).toBe(false)
    })

    it('fills the heart while the filter is on', () => {
        route.query = { favorites: '1' }
        const w = mountView()
        expect(favoriteIcon(w).classes()).toContain('ms-favorite')
        expect(favoritesToggle(w).attributes('aria-pressed')).toBe('true')
    })

    it('labels the toggle by what clicking it will do', () => {
        expect(favoritesToggle(mountView()).attributes('aria-label')).toBe('Show favorites only')
        route.query = { favorites: '1' }
        expect(favoritesToggle(mountView()).attributes('aria-label')).toBe('Show all')
    })

    it('reads ?favorites=1 and passes it to whichever body is active', () => {
        const ui = useUiStore()
        route.query = { favorites: '1' }
        expect(mountView().findComponent(AlbumGridStub).props('favoritesOnly')).toBe(true)

        route.query = { favorites: '1' }
        ui.setLibraryViewMode('releases', 'list')
        expect(mountView().findComponent(AlbumListStub).props('favoritesOnly')).toBe(true)

        route.params = { folderId: '1', mode: 'artists' }
        route.query = { favorites: '1' }
        ui.setLibraryViewMode('artists', 'grid')
        expect(mountView().findComponent(ArtistGridStub).props('favoritesOnly')).toBe(true)

        route.query = { favorites: '1' }
        ui.setLibraryViewMode('artists', 'list')
        expect(mountView().findComponent(ArtistListStub).props('favoritesOnly')).toBe(true)
    })

    it('writes ?favorites=1 on enable and drops the key on disable, keeping the path', async () => {
        const w = mountView()
        await favoritesToggle(w).trigger('click')
        expect(replace).toHaveBeenCalledWith({ query: { favorites: '1' } })

        replace.mockReset()
        route.query = { favorites: '1' }
        const on = mountView()
        await favoritesToggle(on).trigger('click')
        expect(replace).toHaveBeenCalledWith({ query: {} })
    })

    it('layout state is independent of the favorites filter', async () => {
        const ui = useUiStore()
        ui.setLibraryViewMode('releases', 'list')
        const w = mountView()
        await favoritesToggle(w).trigger('click')
        expect(replace).toHaveBeenCalledWith({ query: { favorites: '1' } })
        // Layout is not in query, still in store
        expect(ui.getLibraryViewMode('releases')).toBe('list')
    })

    // "6 favorites", not "6 favorite releases": the active tab names the type, and the
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

        route.params = { folderId: '1', mode: 'artists' }
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
        route.query = { favorites: '1' }
        const w = mountView()
        expect(favoritesToggle(w).exists()).toBe(false)
        // The feed is unfiltered, so the count is the feed's, not a favorites count.
        expect(w.findComponent(DiscoveryFeedStub).exists()).toBe(true)
    })
})

// The release-type filter is URL state (?releaseType=), like favorites, and is
// meaningful only in Releases mode with favorites off.
describe('LibraryView release-type tabs', () => {
    // The release-type row's labels, or null when the row is not rendered — the
    // layout toggle is the other SelectButton, told apart by its "All" option.
    const typeTabs = (w: ReturnType<typeof mountView>) => {
        const row = w
            .findAllComponents(SelectButton)
            .find((sb) =>
                (sb.props('options') as Array<{ label: string }>).some((o) => o.label === 'All')
            )
        return row ? (row.props('options') as Array<{ label: string }>).map((o) => o.label) : null
    }

    // Until the carried types arrive (or if they cannot be fetched), nothing is
    // known to be missing, so every type is offered.
    it('renders every release-type tab in Releases mode until the carried types are known', () => {
        route.params = { folderId: '1', mode: 'releases' }
        route.query = {}
        expect(typeTabs(mountView())).toEqual([
            'All',
            'Albums',
            'Singles',
            'EPs',
            'Broadcast',
            'Other'
        ])
    })

    // A type with no release would only ever list nothing, so it is not offered.
    // Secondary types (Compilation) never get a tab, and the server's spelling
    // may differ in case from the vocabulary's.
    it('offers only the types the library carries, asking for that library', () => {
        releaseTypesRef.value = [
            { name: 'Album', albumCount: 12 },
            { name: 'Compilation', albumCount: 3 },
            { name: 'single', albumCount: 4 }
        ]
        const w = mountView()
        expect(typeTabs(w)).toEqual(['All', 'Albums', 'Singles'])
        expect(toValue(releaseTypesFolder)).toBe(1)
    })

    // A linked or stale ?releaseType= naming a type the library no longer carries
    // keeps its tab, so the view shows which filter is on and it can be left.
    it('keeps the active type even when the library carries none of it', () => {
        releaseTypesRef.value = [{ name: 'Album', albumCount: 12 }]
        route.query = { releaseType: 'Broadcast' }
        expect(typeTabs(mountView())).toEqual(['All', 'Albums', 'Broadcast'])
    })

    // With no typed release a lone "All" would narrow nothing, so the row goes.
    it('drops the row when the library carries no release type', () => {
        releaseTypesRef.value = []
        const w = mountView()
        expect(typeTabs(w)).toBeNull()
        // Only the layout toggle is left.
        expect(filterSelects(w).length).toBe(1)
    })

    it('writes ?releaseType when a type tab is chosen', async () => {
        route.params = { folderId: '1', mode: 'releases' }
        route.query = {}
        const w = mountView()
        const typeBtn = w
            .findAllComponents(SelectButton)
            .find((sb) =>
                (sb.props('options') as Array<{ label: string }>).some((o) => o.label === 'Singles')
            )!
        typeBtn.vm.$emit('update:modelValue', 'Single')
        await w.vm.$nextTick()
        expect(replace).toHaveBeenCalledWith(
            expect.objectContaining({ query: expect.objectContaining({ releaseType: 'Single' }) })
        )
    })

    it('hides the release-type tabs while favorites is on', () => {
        route.params = { folderId: '1', mode: 'releases' }
        route.query = { favorites: '1' }
        const w = mountView()
        const labels = w.findAllComponents(SelectButton).flatMap((sb) =>
            (sb.props('options') as Array<{ label: string }>).map((o) => o.label)
        )
        expect(labels).not.toContain('Singles')
    })
})

// The root (no folderId) is the whole catalog, and opens on its Discover feed.
describe('LibraryView Discover at the root', () => {
    beforeEach(() => {
        route.params = {}
    })

    it('defaults to the whole catalog\'s Discover feed', () => {
        const w = mountView()
        expect(w.findComponent(DiscoveryFeedStub).exists()).toBe(true)
        expect(w.findComponent(DiscoveryFeedStub).props('folderId')).toBeUndefined()
        expect(w.findComponent(AlbumGridStub).exists()).toBe(false)
    })

    it('passes the layout through to the feed', () => {
        const ui = useUiStore()
        expect(mountView().findComponent(DiscoveryFeedStub).props('layout')).toBe('grid')
        ui.setLibraryViewMode('discover', 'list')
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

    it('leaves Discover for Releases in the releases mode', () => {
        route.params = { mode: 'releases' }
        const w = mountView()
        expect(w.findComponent(AlbumGridStub).exists()).toBe(true)
        expect(w.findComponent(DiscoveryFeedStub).exists()).toBe(false)
    })
})
