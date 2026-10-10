import { describe, it, expect, vi, beforeEach } from 'vitest'
import { onTrayCommand } from '@/lib/host/events'
import * as host from '@/lib/host'

// The Tauri side, faked. `isTauri` decides browser vs app; `listen` stands in for
// the event bridge to Rust and keeps the handler it is given, so a test can
// deliver an event the way Rust would.
const tauri = vi.hoisted(() => ({
    isTauri: vi.fn(() => false),
    invoke: vi.fn(),
    listen: vi.fn()
}))

vi.mock('@tauri-apps/api/core', () => ({ isTauri: tauri.isTauri, invoke: tauri.invoke }))
vi.mock('@tauri-apps/api/event', () => ({ listen: tauri.listen }))

type Delivered = { event: string; id: number; payload: unknown }

const unlisten = vi.fn()
let deliver: (event: Delivered) => void

// What Rust sends: the event name, a listener id, and the payload.
const sent = (payload: unknown): Delivered => ({ event: 'tray://command', id: 1, payload })

beforeEach(() => {
    unlisten.mockReset()
    tauri.listen.mockReset()
    tauri.listen.mockImplementation(async (_name: string, handler: typeof deliver) => {
        deliver = handler
        return unlisten
    })
})

describe('onTrayCommand in the browser', () => {
    beforeEach(() => {
        tauri.isTauri.mockReturnValue(false)
    })

    // No Rust side, so no tray: subscribing is harmless and Tauri is never asked.
    it('resolves to a callable no-op and listens to nothing', async () => {
        const cb = vi.fn()

        const unsubscribe = await onTrayCommand(cb)

        expect(() => unsubscribe()).not.toThrow()
        expect(tauri.listen).not.toHaveBeenCalled()
        expect(cb).not.toHaveBeenCalled()
    })
})

describe('onTrayCommand in the desktop app', () => {
    beforeEach(() => {
        tauri.isTauri.mockReturnValue(true)
    })

    it('listens for the tray command event', async () => {
        await onTrayCommand(vi.fn())

        expect(tauri.listen).toHaveBeenCalledOnce()
        expect(tauri.listen).toHaveBeenCalledWith('tray://command', expect.any(Function))
    })

    it.each(['play_pause', 'next'] as const)(
        'hands the %s command to the callback',
        async (cmd) => {
            const cb = vi.fn()
            await onTrayCommand(cb)

            deliver(sent(cmd))

            expect(cb).toHaveBeenCalledOnce()
            expect(cb).toHaveBeenCalledWith(cmd)
        }
    )

    // Whatever else arrives on the event is not for the player: a command the SPA
    // does not know must never reach the callback.
    it.each([
        ['an unknown command', 'previous'],
        ['a command in another case', 'PLAY_PAUSE'],
        ['an empty string', ''],
        ['a number', 7],
        ['null', null],
        ['no payload', undefined],
        ['an object', { command: 'next' }]
    ])('ignores %s', async (_, payload) => {
        const cb = vi.fn()
        await onTrayCommand(cb)

        deliver(sent(payload))

        expect(cb).not.toHaveBeenCalled()
    })

    it('resolves to the function that stops listening', async () => {
        const unsubscribe = await onTrayCommand(vi.fn())
        expect(unlisten).not.toHaveBeenCalled()

        unsubscribe()

        expect(unlisten).toHaveBeenCalledOnce()
    })

    // The caller decides what a missing tray bridge means (the layout logs it and
    // carries on), so a refusal reaches it as the rejection it is.
    it('rejects with the error when the app refuses the subscription', async () => {
        const refusal = new Error('event.listen not allowed')
        tauri.listen.mockRejectedValue(refusal)

        await expect(onTrayCommand(vi.fn())).rejects.toBe(refusal)
    })
})

describe('the host layer', () => {
    // Callers import it from '@/lib/host', not from the file exercised above.
    it('re-exports onTrayCommand', () => {
        expect(host.onTrayCommand).toBe(onTrayCommand)
    })
})
