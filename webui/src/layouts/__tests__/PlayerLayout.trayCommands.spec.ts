import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { computed, reactive, ref } from 'vue'
import PlayerLayout from '../PlayerLayout.vue'
import type { TrayCommand } from '@/lib/host'

// The desktop app's tray menu lives in Rust and sends Play / Pause and Next as
// events; PlayerLayout turns them into the SPA player's own calls for as long as
// it is mounted. This spec owns that wiring: which player call each command
// makes, that the subscription follows the layout's life (including an unmount
// that beats it), and that a browser tab never subscribes. What the event carries
// is lib/host's own spec (events.spec.ts); the player is a fake, so only which of
// its calls ran matters here.
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({
        shell: computed(() => 'desktop'),
        tier: computed(() => 'desktop'),
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

const player = vi.hoisted(() => ({ togglePlayPause: vi.fn(), playNext: vi.fn() }))
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => player
}))

// A desktop app unless a case says otherwise; onTrayCommand is the host layer's
// subscription, replaced by one that hands the test the callback it was given.
const host = vi.hoisted(() => ({
    desktopApp: true,
    onTrayCommand: vi.fn<(cb: (cmd: TrayCommand) => void) => Promise<() => void>>()
}))
vi.mock('@/lib/host', async () => {
    const { ref } = await import('vue')
    return {
        isDesktopApp: () => host.desktopApp,
        onTrayCommand: host.onTrayCommand,
        serverUrl: ref<string | null>('http://nas.local:8075')
    }
})

const unlisten = vi.fn()
// The callback the layout subscribed with: calling it is the tray sending a command.
let send: (cmd: TrayCommand) => void

const mountLayout = () =>
    mount(PlayerLayout, {
        global: {
            stubs: {
                AppSidebar: true,
                QueueSidebar: true,
                DesktopShell: true,
                MobileShell: true,
                ConfirmDialog: true,
                Toast: true
            }
        }
    })

beforeEach(() => {
    host.desktopApp = true
    unlisten.mockReset()
    player.togglePlayPause.mockReset()
    player.playNext.mockReset()
    host.onTrayCommand.mockReset()
    host.onTrayCommand.mockImplementation(async (cb) => {
        send = cb
        return unlisten
    })
})

afterEach(() => {
    vi.restoreAllMocks()
})

describe('PlayerLayout tray commands in the desktop app', () => {
    it('toggles play / pause on the play_pause command', async () => {
        mountLayout()
        await flushPromises()

        send('play_pause')

        expect(player.togglePlayPause).toHaveBeenCalledOnce()
        expect(player.playNext).not.toHaveBeenCalled()
    })

    it('skips to the next track on the next command', async () => {
        mountLayout()
        await flushPromises()

        send('next')

        expect(player.playNext).toHaveBeenCalledOnce()
        expect(player.togglePlayPause).not.toHaveBeenCalled()
    })

    it('subscribes once, when mounted', async () => {
        mountLayout()
        await flushPromises()

        expect(host.onTrayCommand).toHaveBeenCalledOnce()
        expect(unlisten).not.toHaveBeenCalled()
    })

    it('unsubscribes when the layout unmounts', async () => {
        const layout = mountLayout()
        await flushPromises()

        layout.unmount()

        expect(unlisten).toHaveBeenCalledOnce()
    })

    // Subscribing is async. A layout that goes away first must not leave the
    // listener behind: it would keep driving the player after its window is gone.
    it('unsubscribes once a subscription that an unmount beat settles', async () => {
        let settle!: (unlisten: () => void) => void
        host.onTrayCommand.mockImplementation(
            () =>
                new Promise((resolve) => {
                    settle = resolve
                })
        )
        const layout = mountLayout()

        layout.unmount()
        expect(unlisten).not.toHaveBeenCalled()
        settle(unlisten)
        await flushPromises()

        expect(unlisten).toHaveBeenCalledOnce()
    })

    // Only the tray menu is lost; the player layout must still come up and go.
    it('logs and carries on when the app refuses the subscription', async () => {
        const refusal = new Error('event.listen not allowed')
        host.onTrayCommand.mockRejectedValue(refusal)
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})

        const layout = mountLayout()
        await flushPromises()

        expect(warn).toHaveBeenCalledExactlyOnceWith('Tray commands unavailable', refusal)
        expect(() => layout.unmount()).not.toThrow()
    })
})

describe('PlayerLayout tray commands in a browser tab', () => {
    // There is no tray to hear from: no subscription is even asked for.
    it('never subscribes', async () => {
        host.desktopApp = false

        const layout = mountLayout()
        await flushPromises()
        layout.unmount()

        expect(host.onTrayCommand).not.toHaveBeenCalled()
        expect(unlisten).not.toHaveBeenCalled()
    })
})
