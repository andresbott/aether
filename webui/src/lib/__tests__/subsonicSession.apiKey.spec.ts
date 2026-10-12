import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'

vi.mock('@/lib/api/Tokens', () => ({ mintSpaToken: vi.fn() }))

import { mintSpaToken } from '@/lib/api/Tokens'
import { sessionKind } from '@/lib/host'
import { sessionExpired } from '@/lib/authState'
import { subsonicClient } from '@/lib/api/subsonic'
import {
    MAX_MINT_ATTEMPTS,
    handleRestAuthFailure,
    mintRetriesExhausted,
    remintApiKey,
    resetSubsonicSession
} from '@/lib/subsonicSession'

// What the server answers a key it no longer accepts (error 44; a 40 means the
// same for a key that was sent).
const failed44 = {
    'subsonic-response': { status: 'failed', error: { code: 44, message: 'Invalid API key' } }
}

// The real host, in browser mode: its sessionKind is set by hand here, as the
// app's initHost would set it from the saved settings.
describe('the /rest credential in a key session', () => {
    beforeEach(() => {
        resetSubsonicSession()
        sessionExpired.value = false
        sessionKind.value = 'apiKey'
        vi.mocked(mintSpaToken).mockReset()
    })

    afterEach(() => {
        sessionKind.value = 'cookie'
        vi.unstubAllGlobals()
    })

    it('treats a refused key as the end of the session and mints nothing', async () => {
        await expect(handleRestAuthFailure()).resolves.toBe('session-gone')

        expect(sessionExpired.value).toBe(true)
        expect(mintSpaToken).not.toHaveBeenCalled()
    })

    it('fails the /rest call that met the refused key, without retrying it', async () => {
        subsonicClient.setApiKey('aether_revoked')
        const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(failed44) })
        vi.stubGlobal('fetch', fetchMock)

        await expect(subsonicClient.getUser()).rejects.toThrow('Invalid API key')

        expect(fetchMock).toHaveBeenCalledTimes(1)
        expect(sessionExpired.value).toBe(true)
        expect(mintSpaToken).not.toHaveBeenCalled()
    })

    // Review focus 5: a media element error carries no reason (the player calls
    // remintApiKey on it), so in a key session it is no verdict on the key — the
    // connectivity banner, not a sign-out on a network blip. No budget is spent.
    it('gives a stream recovery no verdict: failed, nothing flagged, no budget spent', async () => {
        for (let i = 0; i < MAX_MINT_ATTEMPTS + 1; i++) {
            await expect(remintApiKey()).resolves.toBe('failed')
        }

        expect(sessionExpired.value).toBe(false)
        expect(mintRetriesExhausted()).toBe(false)
        expect(mintSpaToken).not.toHaveBeenCalled()
    })

    it('still mints in a cookie session', async () => {
        sessionKind.value = 'cookie'
        vi.mocked(mintSpaToken).mockResolvedValue({
            token: 'aether_new',
            tokenId: 'tid',
            expiresAt: '2999-01-01T00:00:00Z'
        })

        await expect(handleRestAuthFailure()).resolves.toBe('ok')

        expect(mintSpaToken).toHaveBeenCalledTimes(1)
        expect(sessionExpired.value).toBe(false)
    })
})
