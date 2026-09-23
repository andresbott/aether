import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

// Guards the Library block: Discover/Artists/Releases sit under a titled
// "Library" section header, followed by any per-folder entries, with Now
// Playing/Search above it and Playlists/Genres/Radio past the spacer below.

const pushSpy = vi.fn()
const route: { name: string; path: string; params: Record<string, string> } = {
    name: 'home',
    path: '/',
    params: {}
}
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ push: pushSpy })
}))

const foldersRef = ref<MusicFolder[]>([])
const catalogRef = ref<CatalogView | undefined>(undefined)
vi.mock('@/composables/useSubsonicQueries', () => ({
    useMusicFolders: () => ({ data: foldersRef }),
    useCatalogView: () => ({ data: catalogRef })
}))
vi.mock('@/composables/useTheme', () => ({
    useTheme: () => ({
        hiddenUnlocked: ref(false),
        unlockHiddenThemes: vi.fn(),
        cycleHiddenTheme: vi.fn(() => ({ label: 'X' }))
    })
}))
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

import AppSidebar from '@/components/layout/AppSidebar.vue'
import { useUiStore } from '@/store/uiStore'
import type { CatalogView, MusicFolder } from '@/types/subsonic'

const mountSidebar = () =>
    mount(AppSidebar, { global: { directives: { tooltip: {} }, stubs: { UserMenu: true } } })

// The nav in DOM order, each spacer as '---' and the section header by its text,
// so an entry crossing a spacer fails too, not just a reorder within a group.
const navLayout = () =>
    mountSidebar()
        .findAll('.sidebar-nav > *')
        .map((n) => (n.classes('nav-separator') ? '---' : n.text()))

beforeEach(() => {
    setActivePinia(createPinia())
    pushSpy.mockReset()
    foldersRef.value = []
    catalogRef.value = undefined
    Object.assign(route, { name: 'home', path: '/', params: {} })
})

describe('AppSidebar Library block', () => {
    it('puts Now Playing and Search above the Library block', () => {
        const labels = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .map((n) => n.text())
        expect(labels.slice(0, 2)).toEqual(['Now Playing', 'Search'])
    })

    it('renders a "Library" section header', () => {
        const headers = mountSidebar()
            .findAll('.nav-section-label')
            .map((n) => n.text())
        expect(headers).toContain('Library')
    })

    it('lists the browse modes in the Library block, then Playlists past the spacer', () => {
        expect(navLayout()).toEqual([
            'Now Playing',
            'Search',
            '---',
            'Library',
            'Discover',
            'Artists',
            'Releases',
            '---',
            'Playlists',
            'Genres',
            'Radio'
        ])
    })

    it('gives Discover the compass icon and routes it to /library', async () => {
        const item = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .find((n) => n.text() === 'Discover')!
        expect(item.find('i').classes()).toContain('pi-compass')
        await item.trigger('click')
        expect(pushSpy).toHaveBeenCalledWith('/library')
    })

    it('routes Releases to the /library/releases path', async () => {
        const item = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .find((n) => n.text() === 'Releases')!
        await item.trigger('click')
        expect(pushSpy).toHaveBeenCalledWith('/library/releases')
    })

    it('routes Artists to the /library/artists path', async () => {
        const item = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .find((n) => n.text() === 'Artists')!
        await item.trigger('click')
        expect(pushSpy).toHaveBeenCalledWith('/library/artists')
    })

    it('appends per-library entries to the Library block, before the spacer', () => {
        foldersRef.value = [
            { id: 1, name: 'Main' },
            { id: 2, name: 'Classical' }
        ]
        expect(navLayout()).toEqual([
            'Now Playing',
            'Search',
            '---',
            'Library',
            'Discover',
            'Artists',
            'Releases',
            'Main',
            'Classical',
            '---',
            'Playlists',
            'Genres',
            'Radio'
        ])
    })

    // A library is a saved filter, so even the only one is a narrower view than
    // the catalog the Discover/Releases/Artists entries browse: it has to be
    // reachable, or following the README produces nothing visible here.
    it('lists the only library, with its own route', async () => {
        foldersRef.value = [{ id: 1, name: 'Main' }]
        const item = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .find((n) => n.text() === 'Main')
        expect(item).toBeTruthy()
        await item!.trigger('click')
        expect(pushSpy).toHaveBeenCalledWith('/library/1')
    })

    it('adds no per-library entry when there is no library', () => {
        foldersRef.value = []
        const labels = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .map((n) => n.text())
        expect(labels).toEqual([
            'Now Playing',
            'Search',
            'Discover',
            'Artists',
            'Releases',
            'Playlists',
            'Genres',
            'Radio'
        ])
    })
})

// A library set to split its views gets a section of its own below the Library
// block: a header with its name, then an entry per view it offers.
describe('AppSidebar split libraries', () => {
    const classical = (): MusicFolder => ({
        id: 2,
        name: 'Classical',
        views: ['artists', 'releases'],
        defaultView: 'releases',
        splitViews: true
    })

    const sectionItems = () =>
        mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .filter((n) => n.text() === 'Artists' || n.text() === 'Releases')
            .slice(2) // past the root Library block's own two

    it('gives a split library its own section, keeping single ones in the Library block', () => {
        foldersRef.value = [{ id: 1, name: 'Main' }, classical()]
        expect(navLayout()).toEqual([
            'Now Playing',
            'Search',
            '---',
            'Library',
            'Discover',
            'Artists',
            'Releases',
            'Main',
            '---',
            'Classical',
            'Artists',
            'Releases',
            '---',
            'Playlists',
            'Genres',
            'Radio'
        ])
    })

    it('routes the view it opens on to the bare path and the others to a segment', async () => {
        foldersRef.value = [classical()]
        const [artists, releases] = sectionItems()
        await artists.trigger('click')
        expect(pushSpy).toHaveBeenLastCalledWith('/library/2/artists')
        await releases.trigger('click')
        expect(pushSpy).toHaveBeenLastCalledWith('/library/2')
    })

    it('marks only the view shown active, the bare path being the default view', () => {
        foldersRef.value = [classical()]
        Object.assign(route, { name: 'library-folder', path: '/library/2', params: { folderId: '2' } })
        expect(sectionItems().map((n) => n.classes('active'))).toEqual([false, true])

        route.params = { folderId: '2', mode: 'artists' }
        expect(sectionItems().map((n) => n.classes('active'))).toEqual([true, false])
    })

    it('drops the section header when collapsed, keeping the spacer that sets it apart', () => {
        foldersRef.value = [classical()]
        useUiStore().sidebarCollapsed = true
        const layout = navLayout()
        expect(layout).not.toContain('Classical')
        expect(layout.filter((n) => n === '---')).toHaveLength(3)
    })
})

// The root set to one entry (catalog.splitViews false): a single "All music"
// entry replaces the three browse modes, active on any root view.
describe('AppSidebar root as one entry', () => {
    it('lists one root entry, then the single-entry libraries', () => {
        catalogRef.value = { splitViews: false }
        foldersRef.value = [{ id: 1, name: 'Main' }]
        expect(navLayout()).toEqual([
            'Now Playing',
            'Search',
            '---',
            'Library',
            'All music',
            'Main',
            '---',
            'Playlists',
            'Genres',
            'Radio'
        ])
    })

    it('routes to /library, keeps the shortcut badge, and is active on any root view', async () => {
        catalogRef.value = { splitViews: false }
        Object.assign(route, { name: 'library', path: '/library/artists', params: { mode: 'artists' } })
        const item = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .find((n) => n.text() === 'All music')!
        expect(item.attributes('data-shortcut')).toBe('library')
        expect(item.classes()).toContain('active')
        await item.trigger('click')
        expect(pushSpy).toHaveBeenCalledWith('/library')
    })

    it('is not active inside a library', () => {
        catalogRef.value = { splitViews: false }
        foldersRef.value = [{ id: 1, name: 'Main' }]
        Object.assign(route, { name: 'library-folder', path: '/library/1', params: { folderId: '1' } })
        const item = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .find((n) => n.text() === 'All music')!
        expect(item.classes()).not.toContain('active')
    })
})
