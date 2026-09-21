import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

// Guards the redesigned Library block: Discover/Releases/Artists sit under a
// titled "Library" section header, followed by Playlists and any per-folder entries,
// with Now Playing/Search above it and Genres/Radio below.

const pushSpy = vi.fn()
vi.mock('vue-router', () => ({
    useRoute: () => ({ name: 'home', path: '/', params: {} }),
    useRouter: () => ({ push: pushSpy })
}))

const foldersRef = ref<Array<{ id: number; name: string; icon?: string }>>([])
vi.mock('@/composables/useSubsonicQueries', () => ({
    useMusicFolders: () => ({ data: foldersRef })
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

const mountSidebar = () =>
    mount(AppSidebar, { global: { directives: { tooltip: {} }, stubs: { UserMenu: true } } })

beforeEach(() => {
    setActivePinia(createPinia())
    pushSpy.mockReset()
    foldersRef.value = []
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

    it('lists the browse modes, then Playlists, in the Library block', () => {
        const labels = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .map((n) => n.text())
        expect(labels).toEqual([
            'Now Playing',
            'Search',
            'Discover',
            'Releases',
            'Artists',
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

    it('appends per-library entries after Playlists, before Genres', () => {
        foldersRef.value = [
            { id: 1, name: 'Main' },
            { id: 2, name: 'Classical' }
        ]
        const labels = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .map((n) => n.text())
        expect(labels).toEqual([
            'Now Playing',
            'Search',
            'Discover',
            'Releases',
            'Artists',
            'Playlists',
            'Main',
            'Classical',
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
            'Releases',
            'Artists',
            'Playlists',
            'Genres',
            'Radio'
        ])
    })
})
