import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, nextTick } from 'vue'
import PrimeVue from 'primevue/config'
import MobileMoreDrawer from '../MobileMoreDrawer.vue'

const push = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ push }),
    useRoute: () => ({ name: 'home', params: {} })
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

const mountDrawer = async () => {
    const wrapper = mount(MobileMoreDrawer, {
        props: { visible: false },
        global: { plugins: [PrimeVue] },
        attachTo: document.body
    })
    await wrapper.setProps({ visible: true })
    await nextTick()
    return wrapper
}

beforeEach(() => {
    push.mockClear()
    logoutMutate.mockClear()
    isAdmin = ref(false)
    authRequired = ref(true)
    folders = ref([])
})

const itemLabels = () =>
    Array.from(document.body.querySelectorAll('.drawer-item')).map((el) =>
        (el.textContent ?? '').trim()
    )

describe('MobileMoreDrawer', () => {
    it('lists the secondary destinations', async () => {
        await mountDrawer()
        // Account block order mirrors UserMenu: User settings → Admin → About.
        expect(itemLabels()).toEqual(['Genres', 'Radio', 'User settings', 'About', 'Log out'])
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

    it('lists per-folder library entries when there are two or more', async () => {
        folders = ref([
            { id: 1, name: 'Music' },
            { id: 2, name: 'Audiobooks' }
        ])
        await mountDrawer()
        expect(itemLabels().slice(0, 2)).toEqual(['Music', 'Audiobooks'])
    })

    // @primeuix's styled mode caps every bottom drawer at 10rem, which showed ~2 of
    // these rows. The opt-in class in _main.scss lifts that to auto/80dvh; it has to
    // land on the PANEL (.p-drawer), not the mask, or the override never matches.
    // The rule itself is pinned in assets/scss/__tests__/bottom-sheet.spec.ts.
    it('marks its panel as an auto-height bottom sheet', async () => {
        await mountDrawer()
        const panel = document.body.querySelector('.p-drawer')
        expect(panel).toBeTruthy()
        expect(panel!.classList.contains('app-bottom-sheet')).toBe(true)
        expect(panel!.closest('.p-drawer-bottom')).toBeTruthy()
    })

    it('navigating closes the drawer and pushes the route', async () => {
        const wrapper = await mountDrawer()
        const genres = Array.from(document.body.querySelectorAll('.drawer-item')).find(
            (el) => el.textContent?.includes('Genres')
        ) as HTMLElement
        genres.click()
        expect(push).toHaveBeenCalledWith('/genres')
        expect(wrapper.emitted('update:visible')?.at(-1)).toEqual([false])
    })
})
