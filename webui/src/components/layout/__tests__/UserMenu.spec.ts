import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref, type Ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { MeUser } from '@/types/users'

const push = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ push })
}))

const authRequired = ref(false)
const currentUser: Ref<MeUser | null> = ref(null)
const isAdmin = ref(true)
const logoutMutate = vi.fn()
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({
        authRequired,
        currentUser,
        isAdmin,
        logout: { mutate: logoutMutate }
    })
}))

import UserMenu from '@/components/layout/UserMenu.vue'

const mountMenu = (props: { collapsed?: boolean } = {}) =>
    mount(UserMenu, {
        props,
        global: { plugins: [PrimeVue], directives: { tooltip: {} } },
        attachTo: document.body
    })

const openMenu = async (w: ReturnType<typeof mountMenu>) => {
    await w.find('.user-btn').trigger('click')
    // Popover teleports its panel to body, outside the wrapper.
    return document.body.querySelector('.account-menu') as HTMLElement
}

const menuItem = (menu: HTMLElement, label: string) =>
    Array.from(menu.querySelectorAll<HTMLButtonElement>('.menu-item')).find((b) =>
        b.textContent!.includes(label)
    )

beforeEach(() => {
    authRequired.value = false
    currentUser.value = null
    isAdmin.value = true
    push.mockClear()
    logoutMutate.mockClear()
    document.body.innerHTML = ''
})

describe('UserMenu identity chip', () => {
    it('shows the login of the signed-in user', () => {
        currentUser.value = { login: 'alice', role: 'admin' }
        expect(mountMenu().find('.user-btn').text()).toBe('alice')
    })

    it('falls back to Guest when anonymous', () => {
        expect(mountMenu().find('.user-btn').text()).toBe('Guest')
    })

    it('hides the name but keeps the avatar when collapsed', () => {
        currentUser.value = { login: 'alice', role: 'admin' }
        const w = mountMenu({ collapsed: true })
        expect(w.find('.user-name').exists()).toBe(false)
        expect(w.find('.user-avatar').exists()).toBe(true)
    })
})

describe('UserMenu account popup', () => {
    // The gear button is gone: the chip is the only control, and Settings is
    // an entry inside the popup.
    it('has no separate gear button', () => {
        expect(mountMenu().find('.gear-btn').exists()).toBe(false)
    })

    it('navigates to /settings from the Admin entry', async () => {
        const menu = await openMenu(mountMenu())
        menuItem(menu, 'Admin')!.click()
        expect(push).toHaveBeenCalledWith('/settings')
    })

    // The /settings area answers 403 to non-admins; the entry would be a
    // dead door for them.
    it('hides the Admin entry from non-admins', async () => {
        isAdmin.value = false
        const menu = await openMenu(mountMenu())
        expect(menuItem(menu, 'Admin')).toBeUndefined()
        expect(menuItem(menu, 'User settings')).toBeDefined()
        expect(menuItem(menu, 'About')).toBeDefined()
    })

    it('navigates to /about from the About entry, below Admin', async () => {
        const menu = await openMenu(mountMenu())
        const labels = Array.from(menu.querySelectorAll('.menu-item')).map(
            (b) => b.textContent!.trim()
        )
        expect(labels.indexOf('About')).toBe(labels.indexOf('Admin') + 1)

        menuItem(menu, 'About')!.click()
        expect(push).toHaveBeenCalledWith('/about')
    })

    it('opens on the identity chip and navigates to the user settings', async () => {
        const w = mountMenu()
        const menu = await openMenu(w)
        expect(menu).not.toBeNull()

        menuItem(menu, 'User settings')!.click()
        expect(push).toHaveBeenCalledWith('/user-settings')
    })

    // The theme picker lives in the User settings view, not here.
    it('offers no theme picker', async () => {
        const menu = await openMenu(mountMenu())
        expect(menu.querySelector('.p-selectbutton')).toBeNull()
    })

    it('has no Log out entry when the server needs no login', async () => {
        const menu = await openMenu(mountMenu())
        expect(menuItem(menu, 'Log out')).toBeUndefined()
    })

    it('logs out through useAuth when sessions exist', async () => {
        authRequired.value = true
        currentUser.value = { login: 'alice', role: 'admin' }
        const menu = await openMenu(mountMenu())

        menuItem(menu, 'Log out')!.click()
        expect(logoutMutate).toHaveBeenCalledOnce()
    })
})
