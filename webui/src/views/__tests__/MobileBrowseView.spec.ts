import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { MenuItem } from 'primevue/menuitem'
import BrandMark from '@/components/common/BrandMark.vue'
import { BROWSE_SHELF_SIZE } from '@/lib/browseShelf'

const push = vi.fn()
const replace = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ push, replace })
}))

const shell = ref<'desktop' | 'mobile'>('mobile')
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({ shell })
}))

const discoveryItems = ref<Array<Record<string, unknown>>>([])
const discoveryLoading = ref(false)
const discoveryError = ref(false)
vi.mock('@/composables/useDiscovery', () => ({
    useDiscoveryFeed: () => ({
        items: discoveryItems,
        isLoading: discoveryLoading,
        isError: discoveryError
    })
}))

const folders = ref<Array<{ id: number; name: string; icon?: string }>>([])
const playlists = ref<Array<{ id: string; name: string }>>([])
const playlistsError = ref(false)
const genres = ref<Array<{ value: string }>>([])
const stations = ref<Array<{ id: string; name: string }>>([])
vi.mock('@/composables/useSubsonicQueries', () => ({
    useMusicFolders: () => ({ data: folders }),
    usePlaylists: () => ({ data: playlists, isLoading: ref(false), isError: playlistsError }),
    useGenres: () => ({ data: genres, isLoading: ref(false), isError: ref(false) }),
    useRadioStations: () => ({ data: stations, isLoading: ref(false), isError: ref(false) })
}))

const logoutMutate = vi.fn()
const isAdmin = ref(false)
const authRequired = ref(true)
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({ isAdmin, authRequired, logout: { mutate: logoutMutate } })
}))

// Stubs so the spec asserts what the view PASSES to the shelves (its own
// responsibility) rather than their markup — BrowseShelf has its own spec.
const ScaffoldStub = {
    name: 'ContentScaffold',
    // navRoot typed, not just named: the view passes it as a valueless attribute,
    // which only casts to `true` for a declared Boolean prop.
    props: { title: String, navRoot: Boolean },
    template: '<div><slot name="title-actions" /><slot name="actions" /><slot /></div>'
}
const ShelfStub = {
    name: 'BrowseShelf',
    props: ['title', 'to', 'items', 'icon', 'loading', 'error', 'errorText', 'emptyText'],
    template: '<div class="shelf-stub">{{ title }}</div>'
}
const AlbumShelfStub = {
    name: 'BrowseAlbumShelf',
    props: ['folderId', 'title', 'icon'],
    template: '<div class="album-shelf-stub">{{ title }}</div>'
}
const MenuStub = {
    name: 'Menu',
    props: ['model'],
    methods: { toggle: () => {} },
    template: '<div class="menu-stub" />'
}

import MobileBrowseView from '@/views/MobileBrowseView.vue'

const mountView = () =>
    mount(MobileBrowseView, {
        global: {
            plugins: [PrimeVue],
            stubs: {
                ContentScaffold: ScaffoldStub,
                BrowseShelf: ShelfStub,
                BrowseAlbumShelf: AlbumShelfStub,
                Menu: MenuStub,
                Button: {
                    inheritAttrs: false,
                    template: '<button :class="$attrs.class" @click="$emit(\'click\')" />'
                }
            }
        }
    })

const sectionTitles = (w: ReturnType<typeof mountView>) =>
    w.findAll('.shelf-stub, .album-shelf-stub').map((el) => el.text())

const shelfByTitle = (w: ReturnType<typeof mountView>, title: string) =>
    w.findAllComponents(ShelfStub).find((s) => s.props('title') === title)!

// Playlists, Genres and Radio only render when they hold something, so any test
// about the page's shape has to put something in them first.
const fillSections = (): void => {
    playlists.value = [{ id: 'pl-1', name: 'Roadtrip' }]
    genres.value = [{ value: 'Jazz' }]
    stations.value = [{ id: 'st-1', name: 'Radio One' }]
}

const menuLabels = (w: ReturnType<typeof mountView>) =>
    (w.findComponent(MenuStub).props('model') as MenuItem[]).map((item) =>
        item.separator ? '—' : item.label
    )

beforeEach(() => {
    push.mockReset()
    replace.mockReset()
    logoutMutate.mockReset()
    shell.value = 'mobile'
    discoveryItems.value = []
    discoveryLoading.value = false
    discoveryError.value = false
    folders.value = []
    playlists.value = []
    playlistsError.value = false
    genres.value = []
    stations.value = []
    isAdmin.value = false
    authRequired.value = true
})

describe('MobileBrowseView sections', () => {
    it('renders the sections in the sidebar’s order, libraries after Library', () => {
        fillSections()
        folders.value = [
            { id: 1, name: 'Music' },
            { id: 2, name: 'Audiobooks' }
        ]
        expect(sectionTitles(mountView())).toEqual([
            'Library',
            'Music',
            'Audiobooks',
            'Playlists',
            'Genres',
            'Radio'
        ])
    })

    // Same rule as the sidebar: with one library the Library shelf already
    // covers everything in it.
    it('omits the per-library shelves when there is only one library', () => {
        fillSections()
        folders.value = [{ id: 1, name: 'Music' }]
        expect(sectionTitles(mountView())).toEqual(['Library', 'Playlists', 'Genres', 'Radio'])
    })

    it('points every section at the full view it samples', () => {
        fillSections()
        folders.value = [
            { id: 1, name: 'Music' },
            { id: 2, name: 'Audiobooks' }
        ]
        const w = mountView()
        expect(shelfByTitle(w, 'Library').props('to')).toEqual({ name: 'library' })
        expect(shelfByTitle(w, 'Playlists').props('to')).toEqual({ name: 'playlists' })
        expect(shelfByTitle(w, 'Genres').props('to')).toEqual({ name: 'genres' })
        expect(shelfByTitle(w, 'Radio').props('to')).toEqual({ name: 'radio' })
        expect(w.findComponent(AlbumShelfStub).props('folderId')).toBe(1)
    })

    it('samples the discovery feed for the Library shelf, capped at the shelf size', () => {
        discoveryItems.value = Array.from({ length: BROWSE_SHELF_SIZE + 4 }, (_, i) => ({
            type: 'album',
            rank: i,
            album: { id: `al-${i}`, name: `Album ${i}` }
        }))
        const items = shelfByTitle(mountView(), 'Library').props('items') as Array<{ key: string }>
        expect(items).toHaveLength(BROWSE_SHELF_SIZE)
        expect(items[0].key).toBe('al-0')
    })

    it('caps and keys the playlist, genre and radio shelves', () => {
        playlists.value = Array.from({ length: BROWSE_SHELF_SIZE + 2 }, (_, i) => ({
            id: `pl-${i}`,
            name: `List ${i}`
        }))
        // Genres are keyed by `value`: they carry no id at all.
        genres.value = [{ value: 'Jazz' }, { value: 'Rock' }]
        stations.value = [{ id: 'st-1', name: 'Radio One' }]
        const w = mountView()
        expect(shelfByTitle(w, 'Playlists').props('items')).toHaveLength(BROWSE_SHELF_SIZE)
        expect(
            (shelfByTitle(w, 'Genres').props('items') as Array<{ key: string }>).map((i) => i.key)
        ).toEqual(['Jazz', 'Rock'])
        expect(
            (shelfByTitle(w, 'Radio').props('items') as Array<{ key: string }>)[0].key
        ).toBe('st-1')
    })

    it('passes each section its own loading and error state', () => {
        fillSections()
        discoveryLoading.value = true
        playlistsError.value = true
        const w = mountView()
        expect(shelfByTitle(w, 'Library').props('loading')).toBe(true)
        expect(shelfByTitle(w, 'Playlists').props('error')).toBe(true)
        expect(shelfByTitle(w, 'Genres').props('error')).toBe(false)
    })
})

// A heading and a "See all" over empty space is chrome pointing at nothing, so
// these three sections are left out rather than shown empty.
describe('MobileBrowseView empty sections', () => {
    it('leaves out Playlists, Genres and Radio when they hold nothing', () => {
        expect(sectionTitles(mountView())).toEqual(['Library'])
    })

    it('leaves out only the sections that are empty', () => {
        genres.value = [{ value: 'Jazz' }]
        expect(sectionTitles(mountView())).toEqual(['Library', 'Genres'])
    })

    // Hiding this one would report a network failure as "you have no playlists".
    it('keeps an item-less section that failed to load, for its error line', () => {
        playlistsError.value = true
        const w = mountView()
        expect(sectionTitles(w)).toEqual(['Library', 'Playlists'])
        expect(shelfByTitle(w, 'Playlists').props('error')).toBe(true)
    })

    // The Library shelf is the page's primary destination: it says the library is
    // empty rather than leaving the page blank.
    it('keeps the Library shelf even with nothing in it', () => {
        expect(sectionTitles(mountView())).toContain('Library')
    })
})

describe('MobileBrowseView header', () => {
    it('tells the scaffold it is the nav root, so it grows no hamburger to itself', () => {
        expect(mountView().findComponent(ScaffoldStub).props('navRoot')).toBe(true)
    })

    // The phone's home surface carries the app's identity, like the desktop
    // sidebar's brand — and it is the h1, so the scaffold's own title stays
    // empty rather than the page having two headings.
    it('heads the page with the Aether brand instead of a title', () => {
        const w = mountView()
        expect(w.findComponent(ScaffoldStub).props('title')).toBe('')
        const brand = w.find('h1.browse-brand')
        expect(brand.exists()).toBe(true)
        expect(brand.text()).toBe('Aether')
        expect(brand.findComponent(BrandMark).exists()).toBe(true)
    })

    it('the search action opens the search view', async () => {
        const w = mountView()
        await w.find('.browse-search-btn').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'search' })
    })

    // Same entries and order as UserMenu's popup — the desktop's account
    // surface — and this is the phone's only way out, since UserMenu lives in
    // the desktop sidebar.
    it('the ⋮ menu mirrors the desktop account menu', () => {
        expect(menuLabels(mountView())).toEqual(['User settings', 'About', '—', 'Log out'])
    })

    it('offers Admin only to admins', () => {
        isAdmin.value = true
        expect(menuLabels(mountView())).toEqual([
            'User settings',
            'Admin',
            'About',
            '—',
            'Log out'
        ])
    })

    it('drops Log out when there is no session (auth method none)', () => {
        authRequired.value = false
        expect(menuLabels(mountView())).toEqual(['User settings', 'About'])
    })

    it('the ⋮ entries navigate and log out', () => {
        isAdmin.value = true
        const items = mountView().findComponent(MenuStub).props('model') as MenuItem[]
        const run = (label: string) =>
            items.find((i) => i.label === label)!.command!({ originalEvent: new Event('click'), item: {} })
        run('User settings')
        expect(push).toHaveBeenCalledWith('/user-settings')
        run('Admin')
        expect(push).toHaveBeenCalledWith('/settings')
        run('About')
        expect(push).toHaveBeenCalledWith('/about')
        run('Log out')
        expect(logoutMutate).toHaveBeenCalled()
    })
})

// The desktop shell has the sidebar, so this page has no place there — the
// mirror of HomeView's mobile-side redirect.
describe('MobileBrowseView shell guard', () => {
    it('replaces the route with the library on the desktop shell', () => {
        shell.value = 'desktop'
        mountView()
        expect(replace).toHaveBeenCalledWith({ name: 'library' })
    })

    it('stays put on the mobile shell', () => {
        mountView()
        expect(replace).not.toHaveBeenCalled()
    })
})
