import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// The desktop player's Rust client calls the server with the SPA's /rest key:
// every mint hands it over, every reset takes it back. The host's IPC wrappers
// are stood in; the rest of the host (the browser's) stays real.
const host = vi.hoisted(() => ({
    setSession: vi.fn<(apiKey: string) => Promise<void>>(),
    clearSession: vi.fn<() => Promise<void>>()
}))
vi.mock('@/lib/host', async (importOriginal) => ({
    ...(await importOriginal<typeof import('@/lib/host')>()),
    setSession: host.setSession,
    clearSession: host.clearSession
}))
vi.mock('@/lib/api/Tokens', () => ({ mintSpaToken: vi.fn() }))

import { mintSpaToken } from '@/lib/api/Tokens'
import { subsonicClient } from '@/lib/api/subsonic'
import { remintApiKey, resetSubsonicSession } from '@/lib/subsonicSession'

const minted = { token: 'aether_minted', tokenId: 'tid1', expiresAt: '2999-01-01T00:00:00Z' }

beforeEach(() => {
    resetSubsonicSession()
    host.setSession.mockReset().mockResolvedValue(undefined)
    host.clearSession.mockReset().mockResolvedValue(undefined)
    vi.mocked(mintSpaToken).mockReset()
})

afterEach(() => {
    vi.restoreAllMocks()
})

describe('session hand-off to the desktop player', () => {
    it('hands every freshly minted key to the desktop player', async () => {
        vi.mocked(mintSpaToken).mockResolvedValue(minted)

        await expect(remintApiKey()).resolves.toBe('ok')

        expect(host.setSession).toHaveBeenCalledTimes(1)
        expect(host.setSession).toHaveBeenCalledWith('aether_minted')
    })

    // The Rust side takes a bare key for whatever server is set: a key minted
    // before a logout or a server switch must never reach it.
    it('never hands over a key whose session ended while it was minted', async () => {
        let finishMint!: (value: typeof minted) => void
        vi.mocked(mintSpaToken).mockReturnValue(
            new Promise((resolve) => {
                finishMint = resolve
            })
        )
        const mint = remintApiKey()

        resetSubsonicSession()
        finishMint(minted)

        await expect(mint).resolves.toBe('failed')
        expect(host.setSession).not.toHaveBeenCalled()
        expect(host.clearSession).toHaveBeenCalledTimes(1)
    })

    it('takes the key back from the desktop player when the session is reset', () => {
        resetSubsonicSession()

        expect(host.clearSession).toHaveBeenCalledTimes(1)
    })

    // Only the Rust client goes without a credential; the SPA keeps its key and
    // logs a short line — never the error, whose text could carry the key.
    it('keeps the SPA working when the desktop player refuses the key', async () => {
        vi.mocked(mintSpaToken).mockResolvedValue(minted)
        host.setSession.mockRejectedValue({ code: 'NotConnected', message: 'not connected' })
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})

        await expect(remintApiKey()).resolves.toBe('ok')
        await vi.waitFor(() => expect(warn).toHaveBeenCalled())

        expect(subsonicClient.hasApiKey()).toBe(true)
        expect(warn).toHaveBeenCalledWith('Could not hand the session to the desktop player')
    })

    it('logs a short line when the desktop player cannot drop its key', async () => {
        host.clearSession.mockRejectedValue({ code: 'Internal', message: 'poisoned lock' })
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})

        resetSubsonicSession()
        await vi.waitFor(() => expect(warn).toHaveBeenCalled())

        expect(warn).toHaveBeenCalledWith('Could not clear the desktop player session')
    })
})
