import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h, nextTick, type VNode } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import ConfirmDialog from 'primevue/confirmdialog'

const switchServer = vi.fn()
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({ switchServer })
}))

// A browser tab unless a case says otherwise.
const host = vi.hoisted(() => ({ desktopApp: false }))
vi.mock('@/lib/host', async () => {
    const { ref } = await import('vue')
    return {
        isDesktopApp: () => host.desktopApp,
        serverHost: (await import('@/lib/host/serverUrl')).serverHost,
        serverUrl: ref<string | null>(null)
    }
})

import { CHANGE_SERVER_GROUP, useChangeServer } from '@/composables/useChangeServer'
import { serverUrl } from '@/lib/host'

// An account menu's Change server entry, as UserMenu and the phone's ⋮ menu use it.
const Entry = defineComponent({
    setup() {
        const { confirmChangeServer } = useChangeServer()
        return () => h('button', { class: 'entry', onClick: () => confirmChangeServer() })
    }
})

// The entry next to the real dialog PlayerLayout mounts for the group, plus
// whatever else the page has on screen.
const mountEntry = (others: VNode[] = []) =>
    mount(
        defineComponent({
            render: () =>
                h('div', [h(Entry), h(ConfirmDialog, { group: CHANGE_SERVER_GROUP }), ...others])
        }),
        { global: { plugins: [PrimeVue, ConfirmationService] }, attachTo: document.body }
    )

// Presses the entry. The spy wraps the real service: the options stay readable
// and the real dialog still answers.
const ask = async (w: ReturnType<typeof mountEntry>) => {
    const confirmRequire = vi.spyOn(w.vm.$confirm, 'require')
    await w.find('.entry').trigger('click')
    return confirmRequire
}

const dialogs = () => document.body.querySelectorAll<HTMLElement>('.p-confirmdialog')
const dialog = () => document.body.querySelector<HTMLElement>('.p-confirmdialog')
const answer = (button: 'accept' | 'reject') =>
    dialog()!.querySelector<HTMLButtonElement>(`.p-confirmdialog-${button}-button`)!.click()

beforeEach(() => {
    host.desktopApp = true
    serverUrl.value = 'http://nas.local:8075'
    switchServer.mockReset()
    document.body.innerHTML = ''
})

// What leaving costs, said as it is: the queue on this device goes, and the
// sign-out reaches the server only when it answers. One that cannot be reached
// keeps its session — a promise of "you will be signed out" would not hold.
const QUESTION =
    'Disconnect from nas.local:8075? The queue on this device is cleared, and you are signed out if the server can be reached.'

describe('useChangeServer', () => {
    it('asks before leaving, naming the server and what leaving costs', async () => {
        const confirmRequire = await ask(mountEntry())

        expect(confirmRequire).toHaveBeenCalledOnce()
        expect(confirmRequire).toHaveBeenCalledWith(
            expect.objectContaining({
                group: CHANGE_SERVER_GROUP,
                header: 'Change server',
                message: QUESTION,
                acceptLabel: 'Change server',
                rejectLabel: 'Cancel'
            })
        )
        // And it is on screen as the user will read it, styled like the app's
        // other confirmations (warning icon, danger accept button).
        const shown = dialog()!
        expect(shown.querySelector('.p-dialog-title')!.textContent).toBe('Change server')
        expect(shown.querySelector('.p-confirmdialog-message')!.textContent).toBe(QUESTION)
        expect(shown.querySelector('.p-confirmdialog-icon')!.classList.contains('ms-warning')).toBe(
            true
        )
        const accept = shown.querySelector('.p-confirmdialog-accept-button')!
        expect(accept.textContent!.trim()).toBe('Change server')
        expect(accept.classList.contains('p-button-danger')).toBe(true)
        expect(shown.querySelector('.p-confirmdialog-reject-button')!.textContent!.trim()).toBe(
            'Cancel'
        )
        // Nothing happens before the answer.
        expect(switchServer).not.toHaveBeenCalled()
    })

    it('switches server once the question is accepted', async () => {
        await ask(mountEntry())

        answer('accept')
        await nextTick()

        expect(switchServer).toHaveBeenCalledOnce()
        expect(switchServer).toHaveBeenCalledWith()
        expect(dialog()).toBeNull()
    })

    it('leaves everything as it was when the question is declined', async () => {
        await ask(mountEntry())

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

        const confirmRequire = await ask(mountEntry())

        expect(confirmRequire).toHaveBeenCalledWith(
            expect.objectContaining({
                message: expect.stringMatching(new RegExp(`^Disconnect from ${shown}\\? `))
            })
        )
    })

    // The views mount ungrouped ConfirmDialogs of their own, and PrimeVue shows
    // every ungrouped dialog for every ungrouped question.
    it('puts the question to the dialog for its group, not to the ungrouped ones the views mount', async () => {
        const viewDialog = h(ConfirmDialog, { pt: { root: { 'data-view-dialog': '' } } })

        await ask(mountEntry([viewDialog]))

        expect(dialogs()).toHaveLength(1)
        expect(dialog()!.textContent).toContain('Disconnect from nas.local:8075?')
        expect(document.body.querySelector('[data-view-dialog]')).toBeNull()
    })

    // switchServer must be unreachable outside the app: its sign-out and purge
    // are the app's, and in the browser the page's own origin is the server.
    it('does nothing outside the desktop app', async () => {
        host.desktopApp = false

        const confirmRequire = await ask(mountEntry())

        expect(confirmRequire).not.toHaveBeenCalled()
        expect(dialog()).toBeNull()
        expect(switchServer).not.toHaveBeenCalled()
    })

    it('does nothing when no server is chosen', async () => {
        serverUrl.value = null

        const confirmRequire = await ask(mountEntry())

        expect(confirmRequire).not.toHaveBeenCalled()
        expect(dialog()).toBeNull()
        expect(switchServer).not.toHaveBeenCalled()
    })
})
