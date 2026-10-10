import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { TrayCommand } from '@/lib/host'
import { useTrayBridge, resetTrayBridgeForTests } from '@/composables/useTrayBridge'

// The desktop app's tray menu lives in Rust and sends Play / Pause and Next as
// events; the bridge turns them into the SPA player's own calls for the life of
// the app. This spec owns that wiring: which player call each command makes, that
// it installs once and only in the app, that it outlives the component that
// installed it (layouts come and go while the music plays on), and that a refused
// subscription costs only the tray. What the event carries is lib/host's own spec
// (events.spec.ts); the player is a fake, so only which of its calls ran matters.
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
vi.mock('@/lib/host', () => ({
    isDesktopApp: () => host.desktopApp,
    onTrayCommand: host.onTrayCommand
}))

// The callback the bridge subscribed with: calling it is the tray sending a command.
let send: (cmd: TrayCommand) => void

// A component that asks for the bridge, the way App does, and can be unmounted.
const installer = () =>
    mount({
        setup() {
            useTrayBridge()
            return () => null
        }
    })

beforeEach(() => {
    resetTrayBridgeForTests()
    host.desktopApp = true
    player.togglePlayPause.mockReset()
    player.playNext.mockReset()
    host.onTrayCommand.mockReset()
    host.onTrayCommand.mockImplementation(async (cb) => {
        send = cb
        return () => {}
    })
})

afterEach(() => {
    vi.restoreAllMocks()
})

describe('useTrayBridge in the desktop app', () => {
    it('toggles play / pause on the play_pause command', async () => {
        useTrayBridge()
        await flushPromises()

        send('play_pause')

        expect(player.togglePlayPause).toHaveBeenCalledOnce()
        expect(player.playNext).not.toHaveBeenCalled()
    })

    it('skips to the next track on the next command', async () => {
        useTrayBridge()
        await flushPromises()

        send('next')

        expect(player.playNext).toHaveBeenCalledOnce()
        expect(player.togglePlayPause).not.toHaveBeenCalled()
    })

    // One listener whoever asks and however often: a second would run each command
    // twice, and a toggle done twice is no toggle.
    it('subscribes once, however many times it is asked', async () => {
        useTrayBridge()
        useTrayBridge()
        installer()
        await flushPromises()

        expect(host.onTrayCommand).toHaveBeenCalledOnce()
        send('play_pause')
        expect(player.togglePlayPause).toHaveBeenCalledOnce()
    })

    // Why it is not PlayerLayout's: the settings routes unmount that layout while
    // the player, a module-level singleton, plays on. The tray must not go with
    // whichever component asked for it.
    it('keeps serving after the component that installed it unmounts', async () => {
        const app = installer()
        await flushPromises()

        app.unmount()
        send('play_pause')
        send('next')

        expect(player.togglePlayPause).toHaveBeenCalledOnce()
        expect(player.playNext).toHaveBeenCalledOnce()
    })

    // Only the tray menu is lost; the app must still come up.
    it('logs and carries on when the app refuses the subscription', async () => {
        const refusal = new Error('event.listen not allowed')
        host.onTrayCommand.mockRejectedValue(refusal)
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})

        expect(() => useTrayBridge()).not.toThrow()
        await flushPromises()

        expect(warn).toHaveBeenCalledExactlyOnceWith('Tray commands unavailable', refusal)
    })
})

describe('useTrayBridge in a browser tab', () => {
    // There is no tray to hear from: no subscription is even asked for.
    it('subscribes to nothing', async () => {
        host.desktopApp = false

        useTrayBridge()
        useTrayBridge()
        await flushPromises()

        expect(host.onTrayCommand).not.toHaveBeenCalled()
    })
})
