import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { computed, defineComponent, h, nextTick, reactive, ref } from 'vue'
import PrimeVue from 'primevue/config'
import ConfirmationService from 'primevue/confirmationservice'
import ConfirmDialog from 'primevue/confirmdialog'
import PlayerLayout from '../PlayerLayout.vue'
import { useChangeServer } from '@/composables/useChangeServer'

// The desktop app's Change server is asked from the account menu of whichever
// shell is up (UserMenu in the desktop sidebar, the ⋮ menu on the phone), and the
// question is shown by ONE dialog that PlayerLayout mounts. This spec owns that:
// it answers the entries of both shells, it outlives a shell swap, and the browser
// never gets it. What the question says is useChangeServer's own spec; chrome
// internals belong to the shells' specs, so both are stubbed here.
const shell = ref<'desktop' | 'mobile'>('desktop')
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({
        shell: computed(() => shell.value),
        tier: computed(() => (shell.value === 'desktop' ? 'desktop' : 'phone')),
        isTouch: ref(false)
    })
}))

vi.mock('@/composables/useQueueSync', () => ({
    useQueueSync: () => ({
        restore: vi.fn().mockResolvedValue(undefined),
        start: vi.fn(),
        stop: vi.fn()
    })
}))

vi.mock('@/composables/useScrollbarWidth', () => ({
    useScrollbarWidth: () => ref(0)
}))

vi.mock('@/composables/useNowPlayingSheet', () => ({
    useNowPlayingSheet: () => ({ detent: ref('collapsed'), open: ref(false), snapTo: () => {} })
}))

vi.mock('@/composables/useMediaSession', () => ({
    useMediaSession: () => {}
}))

const route = reactive({ name: 'library', meta: { flush: true } })
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ push: vi.fn() }),
    RouterView: { template: '<div class="router-outlet" />' },
    RouterLink: { template: '<a><slot /></a>' }
}))

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
        // In the app the layout also subscribes to the tray menu's commands, which
        // this spec has no interest in (PlayerLayout.trayCommands.spec.ts owns it).
        onTrayCommand: async () => () => {},
        serverUrl: ref<string | null>('http://nas.local:8075')
    }
})

// An account menu's Change server entry, standing where each shell has its own.
const Entry = defineComponent({
    setup() {
        const { confirmChangeServer } = useChangeServer()
        return () => h('button', { class: 'entry', onClick: () => confirmChangeServer() })
    }
})

const mountLayout = () =>
    mount(PlayerLayout, {
        global: {
            plugins: [PrimeVue, ConfirmationService],
            stubs: {
                AppSidebar: Entry,
                MobileShell: Entry,
                DesktopShell: { template: '<div />' },
                QueueSidebar: true,
                Toast: true
            }
        },
        attachTo: document.body
    })

const dialog = () => document.body.querySelector<HTMLElement>('.p-confirmdialog')
const answer = (button: 'accept' | 'reject') =>
    dialog()!.querySelector<HTMLButtonElement>(`.p-confirmdialog-${button}-button`)!.click()

beforeEach(() => {
    host.desktopApp = false
    shell.value = 'desktop'
    switchServer.mockReset()
    document.body.innerHTML = ''
})

describe('PlayerLayout Change server dialog in the desktop app', () => {
    beforeEach(() => {
        host.desktopApp = true
    })

    it.each(['desktop', 'mobile'] as const)(
        'answers the question the %s shell asks',
        async (which) => {
            shell.value = which
            const layout = mountLayout()

            await layout.find('.entry').trigger('click')

            expect(dialog()!.querySelector('.p-confirmdialog-message')!.textContent).toBe(
                'Disconnect from nas.local:8075? You will be signed out.'
            )
            expect(switchServer).not.toHaveBeenCalled()

            answer('accept')
            await nextTick()

            expect(switchServer).toHaveBeenCalledOnce()
            expect(dialog()).toBeNull()
        }
    )

    it('leaves the session alone when the question is declined', async () => {
        const layout = mountLayout()
        await layout.find('.entry').trigger('click')

        answer('reject')
        await nextTick()

        expect(switchServer).not.toHaveBeenCalled()
        expect(dialog()).toBeNull()
    })

    // Why the dialog is here and not in either menu: a rotation or a window resize
    // swaps the chrome but must not lose a question the user is in the middle of.
    it('keeps an open question across a shell swap', async () => {
        shell.value = 'desktop'
        const layout = mountLayout()
        await layout.find('.entry').trigger('click')
        const shown = dialog()

        shell.value = 'mobile'
        await nextTick()

        expect(dialog()).toBe(shown)
        answer('accept')
        await nextTick()
        expect(switchServer).toHaveBeenCalledOnce()
    })
})

describe('PlayerLayout Change server dialog in a browser tab', () => {
    // The browser has no server to change: nothing of the dialog is mounted.
    it('mounts none', () => {
        expect(mountLayout().findComponent(ConfirmDialog).exists()).toBe(false)
    })
})
