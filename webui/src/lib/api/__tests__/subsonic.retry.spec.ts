import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { subsonicClient } from '@/lib/api/subsonic'
import { resetSubsonicSession } from '@/lib/subsonicSession'
import { retryDelays } from '@/lib/retry'
import * as TokensApi from '@/lib/api/Tokens'

vi.mock('@/lib/api/Tokens', () => ({ mintSpaToken: vi.fn() }))

const ATTEMPTS = 1 + retryDelays().length

const json = (body: unknown, status = 200) => ({
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(body)
})
const okPing = { 'subsonic-response': { status: 'ok' } }
const failed44 = {
    'subsonic-response': { status: 'failed', error: { code: 44, message: 'Invalid API key' } }
}

describe('subsonic client transport retries', () => {
    beforeEach(() => {
        resetSubsonicSession()
        subsonicClient.setApiKey('aether_key')
    })
    afterEach(() => {
        vi.unstubAllGlobals()
        vi.clearAllMocks()
    })

    it('retries a request the network dropped', async () => {
        const fetchMock = vi
            .fn()
            .mockRejectedValueOnce(new TypeError('Failed to fetch'))
            .mockResolvedValueOnce(json(okPing))
        vi.stubGlobal('fetch', fetchMock)

        await expect(subsonicClient.ping()).resolves.toBe(true)
        expect(fetchMock).toHaveBeenCalledTimes(2)
    })

    it('retries a gateway that could not reach the server', async () => {
        const fetchMock = vi
            .fn()
            .mockResolvedValueOnce(json({}, 503))
            .mockResolvedValueOnce(json(okPing))
        vi.stubGlobal('fetch', fetchMock)

        await expect(subsonicClient.ping()).resolves.toBe(true)
        expect(fetchMock).toHaveBeenCalledTimes(2)
    })

    it('does not retry an answer the server gave', async () => {
        const fetchMock = vi.fn().mockResolvedValue(json({}, 500))
        vi.stubGlobal('fetch', fetchMock)

        await expect(subsonicClient.ping()).resolves.toBe(false)
        expect(fetchMock).toHaveBeenCalledTimes(1)
    })

    // A dropped request may still have reached the server: counting the play
    // twice is worse than missing it.
    it('never repeats a call that adds something', async () => {
        const fetchMock = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'))
        vi.stubGlobal('fetch', fetchMock)

        vi.spyOn(console, 'warn').mockImplementation(() => {})
        await subsonicClient.scrobble('s1')
        expect(fetchMock).toHaveBeenCalledTimes(1)
    })
})

describe('subsonicClient.checkSession', () => {
    beforeEach(() => {
        resetSubsonicSession()
        subsonicClient.setApiKey('aether_key')
    })
    afterEach(() => {
        vi.unstubAllGlobals()
        vi.clearAllMocks()
    })

    it('answers ok when the ping does', async () => {
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue(json(okPing)))
        await expect(subsonicClient.checkSession()).resolves.toBe('ok')
    })

    it('answers unreachable when no attempt gets an answer', async () => {
        const fetchMock = vi.fn().mockRejectedValue(new TypeError('Failed to fetch'))
        vi.stubGlobal('fetch', fetchMock)
        await expect(subsonicClient.checkSession()).resolves.toBe('unreachable')
        expect(fetchMock).toHaveBeenCalledTimes(ATTEMPTS)
    })

    // The point of checking with a ping: an expired key is replaced on the way.
    it('re-mints an expired key and answers ok', async () => {
        vi.mocked(TokensApi.mintSpaToken).mockResolvedValue({
            token: 'aether_fresh',
            tokenId: 'tid',
            expiresAt: '2999-01-01T00:00:00Z'
        })
        const fetchMock = vi
            .fn()
            .mockResolvedValueOnce(json(failed44))
            .mockResolvedValueOnce(json(okPing))
        vi.stubGlobal('fetch', fetchMock)

        await expect(subsonicClient.checkSession()).resolves.toBe('ok')
        expect(TokensApi.mintSpaToken).toHaveBeenCalledTimes(1)
        expect(subsonicClient.getStreamUrl('s1')).toContain('apiKey=aether_fresh')
    })

    it('answers refused when the server rejects the session', async () => {
        vi.mocked(TokensApi.mintSpaToken).mockRejectedValue({ response: { status: 401 } })
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue(json(failed44)))
        await expect(subsonicClient.checkSession()).resolves.toBe('refused')
    })

    it('answers refused without a configured client', async () => {
        resetSubsonicSession()
        await expect(subsonicClient.checkSession()).resolves.toBe('refused')
    })
})
