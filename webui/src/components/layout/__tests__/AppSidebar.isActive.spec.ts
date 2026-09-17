import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

// Guards the mode-aware `isActive` in AppSidebar: the four root browse modes
// (Discover/Releases/Artists/Songs) share `routeName: 'library'` with every
// per-folder entry, yet each must highlight independently — this is the
// coverage the plan promised but never landed.

const route: {
    name: string
    path: string
    params: Record<string, string>
    hash: string
} = { name: 'library', path: '/library', params: {}, hash: '' }

vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ push: vi.fn() })
}))

const foldersRef = ref<Array<{ id: number; name: string; icon?: string }>>([])
vi.mock('@/composables/useSubsonicQueries', () => ({
    useMusicFolders: () => ({ data: foldersRef })
}))
vi.mock('@/store/uiStore', () => ({
    useUiStore: () => ({ sidebarCollapsed: false, toggleSidebar: vi.fn() })
}))
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ queue: ref([]) }) }))
vi.mock('@/composables/useTheme', () => ({
    useTheme: () => ({
        hiddenUnlocked: ref(false),
        unlockHiddenThemes: vi.fn(),
        cycleHiddenTheme: vi.fn(() => ({ label: 'X' }))
    })
}))

import AppSidebar from '@/components/layout/AppSidebar.vue'

const mountSidebar = () =>
    mount(AppSidebar, { global: { directives: { tooltip: {} }, stubs: { UserMenu: true } } })

// Locate a nav entry by its rendered label (icons carry no text of their own),
// then read the `active` class straight off it — the same signal the user sees.
const navItem = (w: ReturnType<typeof mountSidebar>, label: string) =>
    w.findAll('.nav-item').find((n) => n.text() === label)!

const isActive = (w: ReturnType<typeof mountSidebar>, label: string) =>
    navItem(w, label).classes().includes('active')

beforeEach(() => {
    route.name = 'library'
    route.path = '/library'
    route.params = {}
    route.hash = ''
    foldersRef.value = []
})

describe('AppSidebar isActive', () => {
    it('activates Discover at the cross-collection root with no hash', () => {
        const w = mountSidebar()
        expect(isActive(w, 'Discover')).toBe(true)
        expect(isActive(w, 'Releases')).toBe(false)
        expect(isActive(w, 'Artists')).toBe(false)
        expect(isActive(w, 'Songs')).toBe(false)
    })

    it('activates Releases on #releases, and not Discover', () => {
        route.hash = '#releases'
        const w = mountSidebar()
        expect(isActive(w, 'Releases')).toBe(true)
        expect(isActive(w, 'Discover')).toBe(false)
    })

    it('activates Artists on #artists', () => {
        route.hash = '#artists'
        const w = mountSidebar()
        expect(isActive(w, 'Artists')).toBe(true)
        expect(isActive(w, 'Discover')).toBe(false)
        expect(isActive(w, 'Releases')).toBe(false)
        expect(isActive(w, 'Songs')).toBe(false)
    })

    it('activates Songs on #songs', () => {
        route.hash = '#songs'
        const w = mountSidebar()
        expect(isActive(w, 'Songs')).toBe(true)
        expect(isActive(w, 'Discover')).toBe(false)
        expect(isActive(w, 'Releases')).toBe(false)
        expect(isActive(w, 'Artists')).toBe(false)
    })

    it('activates the matching folder entry, and no mode entry, when a folderId is present', () => {
        foldersRef.value = [
            { id: 1, name: 'Main' },
            { id: 2, name: 'Archive' }
        ]
        route.params = { folderId: '2' }
        route.hash = ''
        const w = mountSidebar()
        expect(isActive(w, 'Archive')).toBe(true)
        expect(isActive(w, 'Main')).toBe(false)
        expect(isActive(w, 'Discover')).toBe(false)
        expect(isActive(w, 'Releases')).toBe(false)
        expect(isActive(w, 'Artists')).toBe(false)
        expect(isActive(w, 'Songs')).toBe(false)
    })
})
