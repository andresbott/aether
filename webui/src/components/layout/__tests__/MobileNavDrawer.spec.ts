import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, reactive, nextTick } from 'vue'
import PrimeVue from 'primevue/config'
import MobileNavDrawer from '../MobileNavDrawer.vue'
import { useMobileNav, resetMobileNavForTests } from '@/composables/useMobileNav'

const push = vi.fn()
const route = reactive({
    name: 'home' as string,
    params: {} as Record<string, unknown>,
    path: '/',
    fullPath: '/',
    hash: ''
})
vi.mock('vue-router', () => ({
    useRouter: () => ({ push }),
    useRoute: () => route
}))

const logoutMutate = vi.fn()
let isAdmin = ref(false)
let authRequired = ref(true)
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({
        isAdmin,
        authRequired,
        currentUser: ref({ login: 'andres' }),
        logout: { mutate: logoutMutate }
    })
}))

let folders = ref<Array<{ id: number; name: string; icon?: string }>>([])
vi.mock('@/composables/useSubsonicQueries', () => ({
    useMusicFolders: () => ({ data: folders })
}))

const queue = ref<Array<{ id: string }>>([{ id: '1' }])
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ queue })
}))

const mountDrawer = async () => {
    const wrapper = mount(MobileNavDrawer, {
        global: { plugins: [PrimeVue] },
        attachTo: document.body
    })
    useMobileNav().open()
    await nextTick()
    return wrapper
}

beforeEach(() => {
    resetMobileNavForTests()
    push.mockClear()
    logoutMutate.mockClear()
    isAdmin = ref(false)
    authRequired = ref(true)
    folders = ref([])
    queue.value = [{ id: '1' }]
    route.name = 'home'
    route.params = {}
    route.path = '/'
    route.fullPath = '/'
    route.hash = ''
})

const itemLabels = () =>
    Array.from(document.body.querySelectorAll('.drawer-item')).map((el) =>
        (el.textContent ?? '').trim()
    )

describe('MobileNavDrawer', () => {
    it('lists every destination in the desktop sidebar order', async () => {
        await mountDrawer()
        // Primary trio → browse extras → account block (mirrors UserMenu:
        // User settings → Admin → About) → Log out.
        expect(itemLabels()).toEqual([
            'Now Playing',
            'Queue',
            'Library',
            'Search',
            'Playlists',
            'Genres',
            'Radio',
            'User settings',
            'About',
            'Log out'
        ])
    })

    // With an empty queue `/` replaces itself with the library (HomeView), so
    // a Now Playing or Queue entry would silently take the user somewhere else.
    it('hides Now Playing and Queue while the queue is empty', async () => {
        queue.value = []
        await mountDrawer()
        expect(itemLabels()).not.toContain('Now Playing')
        expect(itemLabels()).not.toContain('Queue')
        expect(itemLabels()[0]).toBe('Library')
    })

    // The queue panel's address (see MobilePlayView): same route, the hash
    // picks the panel. Desktop keeps the queue in its sidebar, so the entry
    // exists only here in the drawer.
    it('the Queue entry navigates to the home route addressed to the queue panel', async () => {
        await mountDrawer()
        const entry = Array.from(document.body.querySelectorAll('.drawer-item')).find(
            (el) => el.textContent?.trim() === 'Queue'
        ) as HTMLElement
        entry.click()
        expect(push).toHaveBeenCalledWith('/#queue')
        expect(useMobileNav().isOpen.value).toBe(false)
    })

    // Now Playing and Queue share route name 'home'; the hash — kept in sync
    // with the swiped-to panel by MobilePlayView — decides which lights up.
    it('the hash decides whether Now Playing or Queue is marked current', async () => {
        await mountDrawer()
        let current = Array.from(document.body.querySelectorAll('[aria-current="page"]'))
        expect(current).toHaveLength(1)
        expect(current[0].textContent?.trim()).toBe('Now Playing')

        // Navigating to the queue panel closes the drawer (route watcher);
        // the highlight matters the next time it opens.
        route.hash = '#queue'
        route.fullPath = '/#queue'
        await nextTick()
        useMobileNav().open()
        await nextTick()
        current = Array.from(document.body.querySelectorAll('[aria-current="page"]'))
        expect(current).toHaveLength(1)
        expect(current[0].textContent?.trim()).toBe('Queue')
    })

    it('shows Admin only for admins', async () => {
        isAdmin = ref(true)
        await mountDrawer()
        expect(itemLabels()).toContain('Admin')
    })

    it('hides Log out when there is no session (auth method none)', async () => {
        authRequired = ref(false)
        await mountDrawer()
        expect(itemLabels()).not.toContain('Log out')
    })

    it('lists per-folder library entries between the primaries and the extras', async () => {
        folders = ref([
            { id: 1, name: 'Music' },
            { id: 2, name: 'Audiobooks' }
        ])
        await mountDrawer()
        expect(itemLabels().slice(4, 6)).toEqual(['Music', 'Audiobooks'])
    })

    it('marks the current destination for AT, and only that one', async () => {
        route.name = 'search'
        route.path = '/search'
        await mountDrawer()
        const current = Array.from(document.body.querySelectorAll('[aria-current="page"]'))
        expect(current).toHaveLength(1)
        expect(current[0].textContent?.trim()).toBe('Search')
    })

    // The library root and the per-folder entries share routeName 'library';
    // the folderId param is what decides which lights up.
    it('lights the folder entry, not the library root, inside a folder', async () => {
        folders = ref([
            { id: 1, name: 'Music' },
            { id: 2, name: 'Audiobooks' }
        ])
        route.name = 'library'
        route.path = '/library/2'
        route.params = { folderId: '2' }
        await mountDrawer()
        const current = Array.from(document.body.querySelectorAll('[aria-current="page"]'))
        expect(current).toHaveLength(1)
        expect(current[0].textContent?.trim()).toBe('Audiobooks')
    })

    it('renders as a width-capped left drawer', async () => {
        await mountDrawer()
        const panel = document.body.querySelector('.p-drawer')
        expect(panel).toBeTruthy()
        // The width/safe-area rule in _main.scss keys on this pair — it has to
        // land on the PANEL, not the mask, or the override never matches.
        expect(panel!.classList.contains('mobile-nav-drawer')).toBe(true)
        expect(panel!.closest('.p-drawer-left')).toBeTruthy()
    })

    it('navigating closes the drawer and pushes the route', async () => {
        await mountDrawer()
        const genres = Array.from(document.body.querySelectorAll('.drawer-item')).find((el) =>
            el.textContent?.includes('Genres')
        ) as HTMLElement
        genres.click()
        expect(push).toHaveBeenCalledWith('/genres')
        expect(useMobileNav().isOpen.value).toBe(false)
    })

    // The drawer never unmounts on navigation (it is shell chrome), so without
    // this a system-back press navigated the app UNDERNEATH an open drawer.
    it('closes when the route changes underneath it', async () => {
        await mountDrawer()
        expect(useMobileNav().isOpen.value).toBe(true)
        route.fullPath = '/genres'
        await nextTick()
        expect(useMobileNav().isOpen.value).toBe(false)
    })

    it('logging out closes the drawer and fires the mutation', async () => {
        await mountDrawer()
        const logout = Array.from(document.body.querySelectorAll('.drawer-item')).find((el) =>
            el.textContent?.includes('Log out')
        ) as HTMLElement
        logout.click()
        expect(logoutMutate).toHaveBeenCalled()
        expect(useMobileNav().isOpen.value).toBe(false)
    })
})
