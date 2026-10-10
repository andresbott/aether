import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h, nextTick, ref, type Ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import ConfirmDialog from 'primevue/confirmdialog'
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

import UserMenu from '@/components/layout/UserMenu.vue'
import { serverUrl } from '@/lib/host'

const mountMenu = (props: { collapsed?: boolean } = {}) =>
    mount(UserMenu, {
        props,
        global: { plugins: [PrimeVue, ConfirmationService], directives: { tooltip: {} } },
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

    it('lists no server entry, and mounts no dialog for one', async () => {
        const w = mountMenu()
        const menu = await openMenu(w)
        expect(outline(menu)).toEqual(['User settings', 'Admin', 'Metadata editor', 'About'])
        expect(w.findComponent(ConfirmDialog).exists()).toBe(false)
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

describe('UserMenu Change server in the desktop app', () => {
    beforeEach(() => {
        host.desktopApp = true
        serverUrl.value = 'http://nas.local:8075'
    })

    // Opens the popup, picks the entry and lets the question render. The spy wraps
    // the real service: the options are readable and the real dialog still answers.
    const askToChangeServer = async () => {
        const w = mountMenu()
        const confirmRequire = vi.spyOn(w.vm.$confirm, 'require')
        const menu = await openMenu(w)
        menuItem(menu, 'Change server')!.click()
        await nextTick()
        return { w, confirmRequire }
    }
    const dialog = () => document.body.querySelector<HTMLElement>('.p-confirmdialog')
    const answer = (button: 'accept' | 'reject') =>
        dialog()!.querySelector<HTMLButtonElement>(`.p-confirmdialog-${button}-button`)!.click()

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

    it('asks before leaving, naming the server and warning about the sign-out', async () => {
        const { confirmRequire } = await askToChangeServer()

        expect(confirmRequire).toHaveBeenCalledOnce()
        expect(confirmRequire).toHaveBeenCalledWith(
            expect.objectContaining({
                header: 'Change server',
                message: 'Disconnect from nas.local:8075? You will be signed out.',
                acceptLabel: 'Change server',
                rejectLabel: 'Cancel'
            })
        )
        // And it is on screen as the user will read it.
        const shown = dialog()!
        expect(shown.querySelector('.p-dialog-title')!.textContent).toBe('Change server')
        expect(shown.querySelector('.p-confirmdialog-message')!.textContent).toBe(
            'Disconnect from nas.local:8075? You will be signed out.'
        )
        expect(shown.querySelector('.p-confirmdialog-accept-button')!.textContent!.trim()).toBe(
            'Change server'
        )
        expect(shown.querySelector('.p-confirmdialog-reject-button')!.textContent!.trim()).toBe(
            'Cancel'
        )
        // Nothing happens before the answer.
        expect(switchServer).not.toHaveBeenCalled()
    })

    it('closes the popup behind the question', async () => {
        await askToChangeServer()
        expect(document.body.querySelector('.account-menu')).toBeNull()
    })

    it('switches server once the question is accepted, without logging out twice', async () => {
        authRequired.value = true
        currentUser.value = { login: 'alice', role: 'admin' }
        await askToChangeServer()

        answer('accept')
        await nextTick()

        expect(switchServer).toHaveBeenCalledOnce()
        expect(switchServer).toHaveBeenCalledWith()
        // switchServer signs out by itself; the Log out mutation would repeat it.
        expect(logoutMutate).not.toHaveBeenCalled()
        expect(dialog()).toBeNull()
    })

    it('leaves the session alone when the question is declined', async () => {
        await askToChangeServer()

        answer('reject')
        await nextTick()

        expect(switchServer).not.toHaveBeenCalled()
        expect(dialog()).toBeNull()
    })

    // {host} is host[:port]: what the user knows the server by, without scheme or path.
    it.each([
        ['http://nas.local:8075', 'nas.local:8075'],
        ['https://music.example.com/aether', 'music.example.com'],
        // A hand-edited settings file can hold anything; the question shows it as saved.
        ['not a url', 'not a url']
    ])('names the server %s as %s', async (saved, shown) => {
        serverUrl.value = saved
        const { confirmRequire } = await askToChangeServer()
        expect(confirmRequire).toHaveBeenCalledWith(
            expect.objectContaining({
                message: `Disconnect from ${shown}? You will be signed out.`
            })
        )
    })

    // The views mount ungrouped ConfirmDialogs next to this always-present sidebar,
    // and PrimeVue shows every ungrouped dialog for every ungrouped question.
    it('puts the question to its own dialog, not to the ungrouped ones the views mount', async () => {
        const withViewDialog = defineComponent({
            render: () => h('div', [h(UserMenu), h(ConfirmDialog)])
        })
        const w = mount(withViewDialog, {
            global: { plugins: [PrimeVue, ConfirmationService], directives: { tooltip: {} } },
            attachTo: document.body
        })
        await w.find('.user-btn').trigger('click')
        const menu = document.body.querySelector('.account-menu') as HTMLElement
        menuItem(menu, 'Change server')!.click()
        await nextTick()

        expect(document.body.querySelectorAll('.p-confirmdialog')).toHaveLength(1)
        expect(dialog()!.textContent).toContain('Disconnect from nas.local:8075?')
    })

    // The entry is only rendered in the app; the handler does not rely on that alone.
    it('does nothing if it is no longer the desktop app', async () => {
        const w = mountMenu()
        const confirmRequire = vi.spyOn(w.vm.$confirm, 'require')
        const entry = menuItem(await openMenu(w), 'Change server')!

        host.desktopApp = false
        entry.click()
        await nextTick()

        expect(confirmRequire).not.toHaveBeenCalled()
        expect(dialog()).toBeNull()
        expect(switchServer).not.toHaveBeenCalled()
    })

    it('does nothing when no server is chosen', async () => {
        const w = mountMenu()
        const confirmRequire = vi.spyOn(w.vm.$confirm, 'require')
        const entry = menuItem(await openMenu(w), 'Change server')!

        serverUrl.value = null
        entry.click()
        await nextTick()

        expect(confirmRequire).not.toHaveBeenCalled()
        expect(dialog()).toBeNull()
        expect(switchServer).not.toHaveBeenCalled()
    })
})
