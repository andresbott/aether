import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

// Replaces AppSidebar.discovery.spec.ts: the standalone /discover entry and view are
// gone, so what needs guarding is the Library group's new shape — one flat level, no
// section header, and Library itself as the door to the Discovery feed.

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

describe('AppSidebar Library entry', () => {
    it('names the cross-collection entry "Library", not "All Music"', () => {
        foldersRef.value = [
            { id: 1, name: 'Main' },
            { id: 2, name: 'Classical' }
        ]
        const labels = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .map((n) => n.text())
        expect(labels).toContain('Library')
        expect(labels).not.toContain('All Music')
    })

    it('carries the compass icon, since it opens on the Discovery feed', () => {
        const item = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .find((n) => n.text() === 'Library')!
        expect(item.find('i').classes()).toContain('pi-compass')
    })

    it('navigates to /library when clicked', async () => {
        const item = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .find((n) => n.text() === 'Library')!
        await item.trigger('click')
        expect(pushSpy).toHaveBeenCalledWith('/library')
    })

    it('offers no standalone Discover entry — the feed lives inside Library', async () => {
        const w = mountSidebar()
        expect(w.text()).not.toContain('Discover')
        expect(pushSpy).not.toHaveBeenCalledWith('/discover')
    })

    // Both section headers are gone; the nav is one flat list around a single
    // label-less separator.
    it('renders no section headers at all', () => {
        const w = mountSidebar()
        expect(w.findAll('.nav-section-label')).toHaveLength(0)
        expect(w.findAll('.nav-separator.has-label')).toHaveLength(0)
        expect(w.findAll('.sidebar-nav .nav-separator')).toHaveLength(1)
    })

    it('keeps Radio as a flat entry now that the Streaming header is gone', () => {
        const labels = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .map((n) => n.text())
        expect(labels).toContain('Radio')
        expect(mountSidebar().text()).not.toContain('Streaming')
    })

    // Each library is a peer destination, not a child of a browse mode, so no
    // per-folder entry may carry the old indent class.
    it('renders per-folder entries at the same level as every other entry', () => {
        foldersRef.value = [
            { id: 1, name: 'Main' },
            { id: 2, name: 'Classical' }
        ]
        const w = mountSidebar()
        expect(w.findAll('.nav-item.sub-item')).toHaveLength(0)
        const folder = w.findAll('.sidebar-nav .nav-item').find((n) => n.text() === 'Classical')!
        expect(folder.classes()).not.toContain('sub-item')
    })

    it('keeps the per-folder entries and their icons', () => {
        foldersRef.value = [
            { id: 1, name: 'Main' },
            { id: 2, name: 'Classical', icon: 'star' }
        ]
        const w = mountSidebar()
        const labels = w.findAll('.sidebar-nav .nav-item').map((n) => n.text())
        expect(labels).toEqual(
            expect.arrayContaining(['Library', 'Main', 'Classical', 'Playlists', 'Genres'])
        )
        const classical = w.findAll('.sidebar-nav .nav-item').find((n) => n.text() === 'Classical')!
        expect(classical.find('i').classes()).toContain('pi-star')
    })

    // A lone library needs no entry of its own — the Library entry already reaches it.
    it('adds no per-folder entry when there is one library or none', () => {
        foldersRef.value = [{ id: 1, name: 'Main' }]
        const labels = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .map((n) => n.text())
        expect(labels.filter((l) => l === 'Library')).toHaveLength(1)
        expect(labels).not.toContain('Main')
    })

    it('sits directly below Now Playing, above Search', () => {
        const labels = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .map((n) => n.text())
        expect(labels.slice(0, 3)).toEqual(['Now Playing', 'Library', 'Search'])
    })

    // The per-folder entries did NOT follow the root up: they lead the group below
    // the separator, with Search still above it.
    it('leaves the per-folder entries below Search', () => {
        foldersRef.value = [
            { id: 1, name: 'Main' },
            { id: 2, name: 'Classical' }
        ]
        const labels = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .map((n) => n.text())
        expect(labels).toEqual([
            'Now Playing',
            'Library',
            'Search',
            'Main',
            'Classical',
            'Playlists',
            'Genres',
            'Radio'
        ])
    })

    it('orders Library before Playlists and Genres', () => {
        const labels = mountSidebar()
            .findAll('.sidebar-nav .nav-item')
            .map((n) => n.text())
        expect(labels.indexOf('Library')).toBeLessThan(labels.indexOf('Playlists'))
        expect(labels.indexOf('Playlists')).toBeLessThan(labels.indexOf('Genres'))
    })
})
