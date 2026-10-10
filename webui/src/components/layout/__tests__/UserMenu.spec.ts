import { describe, it, expect, vi, beforeEach } from 'vitest'
import { nextTick, ref, type Ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { ConfirmationOptions } from 'primevue/confirmationoptions'
import type { MeUser } from '@/types/users'

const push = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ push })
}))

const authRequired = ref(false)
const currentUser: Ref<MeUser | null> = ref(null)
const isAdmin = ref(true)
const logoutMutate = vi.fn()
const switchServer = vi.fn()
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({
        authRequired,
        currentUser,
        isAdmin,
        logout: { mutate: logoutMutate },
        switchServer
    })
}))

// A browser tab unless a case says otherwise.
const host = vi.hoisted(() => ({ desktopApp: false }))
vi.mock('@/lib/host', async () => {
    const { ref } = await import('vue')
    return {
        isDesktopApp: () => host.desktopApp,
        serverUrl: ref<string | null>(null)
    }
})

// Change server asks through PrimeVue's confirmation service; the dialog showing
// the question is PlayerLayout's, so here only the question itself is observed.
const confirmRequire = vi.fn<(options: ConfirmationOptions) => void>()
vi.mock('primevue/useconfirm', () => ({ useConfirm: () => ({ require: confirmRequire }) }))

import UserMenu from '@/components/layout/UserMenu.vue'
import { CHANGE_SERVER_GROUP } from '@/composables/useChangeServer'
import { serverUrl } from '@/lib/host'

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
    host.desktopApp = false
    serverUrl.value = null
    push.mockClear()
    logoutMutate.mockClear()
    switchServer.mockClear()
    confirmRequire.mockClear()
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

    // The admin areas answer 403 to non-admins; the entries would be dead doors
    // for them.
    it('hides the Admin and Metadata editor entries from non-admins', async () => {
        isAdmin.value = false
        const menu = await openMenu(mountMenu())
        expect(menuItem(menu, 'Admin')).toBeUndefined()
        expect(menuItem(menu, 'Metadata editor')).toBeUndefined()
        expect(menuItem(menu, 'User settings')).toBeDefined()
        expect(menuItem(menu, 'About')).toBeDefined()
    })

    // The admin entries are grouped in order: Admin, then Metadata editor,
    // then About.
    it('navigates to /metadata-editor from the Metadata editor entry, below Admin', async () => {
        const menu = await openMenu(mountMenu())
        const labels = Array.from(menu.querySelectorAll('.menu-item')).map(
            (b) => b.textContent!.trim()
        )
        expect(labels.indexOf('Metadata editor')).toBe(labels.indexOf('Admin') + 1)
        expect(labels.indexOf('About')).toBe(labels.indexOf('Metadata editor') + 1)

        menuItem(menu, 'Metadata editor')!.click()
        expect(push).toHaveBeenCalledWith('/metadata-editor')
    })

    it('navigates to /about from the About entry', async () => {
        const menu = await openMenu(mountMenu())
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

// The popup as the user reads it: entry labels in order, '---' where the separator is.
const outline = (menu: HTMLElement) =>
    Array.from(menu.children).map((el) =>
        el.classList.contains('menu-sep') ? '---' : el.textContent!.trim()
    )

describe('UserMenu in a browser tab', () => {
    // The page's own origin is the server there, so there is none to change: even
    // with an address known, the popup lists what it always did.
    beforeEach(() => {
        serverUrl.value = 'http://localhost:8075'
    })

    it('lists no server entry', async () => {
        const menu = await openMenu(mountMenu())
        expect(outline(menu)).toEqual(['User settings', 'Admin', 'Metadata editor', 'About'])
    })

    it('still separates Log out, and only Log out, when sessions exist', async () => {
        authRequired.value = true
        const menu = await openMenu(mountMenu())
        expect(outline(menu)).toEqual([
            'User settings',
            'Admin',
            'Metadata editor',
            'About',
            '---',
            'Log out'
        ])
    })
})

// What the question says, how it looks and how it is answered is useChangeServer's
// own spec; the menu has to put the entry in place and raise the question.
describe('UserMenu Change server in the desktop app', () => {
    beforeEach(() => {
        host.desktopApp = true
        serverUrl.value = 'http://nas.local:8075'
    })

    it('is the only entry below the separator when there is no login to leave', async () => {
        const menu = await openMenu(mountMenu())
        expect(outline(menu)).toEqual([
            'User settings',
            'Admin',
            'Metadata editor',
            'About',
            '---',
            'Change server'
        ])
    })

    it('sits between the separator and Log out when sessions exist', async () => {
        authRequired.value = true
        currentUser.value = { login: 'alice', role: 'admin' }
        const menu = await openMenu(mountMenu())
        expect(outline(menu)).toEqual([
            'User settings',
            'Admin',
            'Metadata editor',
            'About',
            '---',
            'Change server',
            'Log out'
        ])
    })

    it('asks before leaving, and closes the popup behind the question', async () => {
        const menu = await openMenu(mountMenu())

        menuItem(menu, 'Change server')!.click()
        await nextTick()

        expect(confirmRequire).toHaveBeenCalledOnce()
        expect(confirmRequire).toHaveBeenCalledWith(
            expect.objectContaining({
                group: CHANGE_SERVER_GROUP,
                header: 'Change server',
                message: 'Disconnect from nas.local:8075? You will be signed out.',
                acceptLabel: 'Change server',
                rejectLabel: 'Cancel'
            })
        )
        // Nothing happens before the answer.
        expect(switchServer).not.toHaveBeenCalled()
        expect(document.body.querySelector('.account-menu')).toBeNull()
    })

    it('switches server once the question is accepted, without logging out twice', async () => {
        authRequired.value = true
        currentUser.value = { login: 'alice', role: 'admin' }
        const menu = await openMenu(mountMenu())
        menuItem(menu, 'Change server')!.click()

        confirmRequire.mock.calls[0][0].accept!()

        expect(switchServer).toHaveBeenCalledOnce()
        expect(switchServer).toHaveBeenCalledWith()
        // switchServer signs out by itself; the Log out mutation would repeat it.
        expect(logoutMutate).not.toHaveBeenCalled()
    })
})
