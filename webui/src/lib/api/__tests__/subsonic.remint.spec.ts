import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { subsonicClient } from '@/lib/api/subsonic'
import { remintApiKey, resetSubsonicSession, spaTokenId } from '@/lib/subsonicSession'
import { sessionExpired } from '@/lib/authState'
import * as TokensApi from '@/lib/api/Tokens'

vi.mock('@/lib/api/Tokens', () => ({
    mintSpaToken: vi.fn()
}))

const failed40 = {
    'subsonic-response': { status: 'failed', error: { code: 40, message: 'authentication required' } }
}
const okPing = { 'subsonic-response': { status: 'ok' } }

describe('transparent re-mint', () => {
    beforeEach(() => {
        resetSubsonicSession()
        sessionExpired.value = false
    })
    afterEach(() => {
        vi.unstubAllGlobals()
        vi.clearAllMocks()
    })

    it('re-mints once and retries on subsonic error 40', async () => {
        vi.mocked(TokensApi.mintSpaToken).mockResolvedValue({
            token: 'aether_new_key', tokenId: 'tid2', expiresAt: '2999-01-01T00:00:00Z'
        })
        subsonicClient.setApiKey('aether_old_key')
        const fetchMock = vi.fn()
            .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(failed40) })
            .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(okPing) })
        vi.stubGlobal('fetch', fetchMock)

        const ok = await subsonicClient.ping()
        expect(ok).toBe(true)
        expect(TokensApi.mintSpaToken).toHaveBeenCalledTimes(1)
        const retryUrl = new URL(fetchMock.mock.calls[1][0] as string)
        expect(retryUrl.searchParams.get('apiKey')).toBe('aether_new_key')
        expect(spaTokenId.value).toBe('tid2')
    })

    it('concurrent failures share one mint (single-flight)', async () => {
        vi.mocked(TokensApi.mintSpaToken).mockResolvedValue({
            token: 'aether_new_key', tokenId: 'tid2', expiresAt: '2999-01-01T00:00:00Z'
        })
        const [a, b] = await Promise.all([remintApiKey(), remintApiKey()])
        expect(a).toBe(true)
        expect(b).toBe(true)
        expect(TokensApi.mintSpaToken).toHaveBeenCalledTimes(1)
    })

    it('flips sessionExpired when the mint answers 401', async () => {
        vi.mocked(TokensApi.mintSpaToken).mockRejectedValue({ response: { status: 401 } })
        const ok = await remintApiKey()
        expect(ok).toBe(false)
        expect(sessionExpired.value).toBe(true)
    })
})
