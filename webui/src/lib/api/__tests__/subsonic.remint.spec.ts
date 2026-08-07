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
        expect(a).toBe('ok')
        expect(b).toBe('ok')
        expect(TokensApi.mintSpaToken).toHaveBeenCalledTimes(1)
    })

    it('returns session-gone when the mint answers 401', async () => {
        vi.mocked(TokensApi.mintSpaToken).mockRejectedValue({ response: { status: 401 } })
        const result = await remintApiKey()
        expect(result).toBe('session-gone')
        expect(sessionExpired.value).toBe(true)
    })

    it('returns failed for non-401 errors (409, 500, network)', async () => {
        vi.mocked(TokensApi.mintSpaToken).mockRejectedValue({ response: { status: 409 } })
        const result = await remintApiKey()
        expect(result).toBe('failed')
        expect(sessionExpired.value).toBe(false)
    })

    it('discards a mint result when logout happens mid-flight', async () => {
        let resolveDelayedMint: (value: any) => void
        vi.mocked(TokensApi.mintSpaToken).mockReturnValue(
            new Promise((resolve) => { resolveDelayedMint = resolve })
        )
        subsonicClient.setApiKey('aether_old_key')
        const mintPromise = remintApiKey()
        // Logout while the mint is in flight
        resetSubsonicSession()
        resolveDelayedMint!({ token: 'aether_new_key', tokenId: 'tid2', expiresAt: '2999-01-01T00:00:00Z' })
        const result = await mintPromise
        expect(result).toBe('failed')
        expect(subsonicClient.hasApiKey()).toBe(false)
        expect(spaTokenId.value).toBeNull()
    })

    it('re-mints and retries through submitMultipart (FormData path)', async () => {
        vi.mocked(TokensApi.mintSpaToken).mockResolvedValue({
            token: 'aether_new_key', tokenId: 'tid3', expiresAt: '2999-01-01T00:00:00Z'
        })
        subsonicClient.setApiKey('aether_old_key')
        const fetchMock = vi.fn()
            .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(failed40) })
            .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(okPing) })
        vi.stubGlobal('fetch', fetchMock)

        await expect(subsonicClient.updateGenreCover('g1')).resolves.toBeUndefined()
        expect(TokensApi.mintSpaToken).toHaveBeenCalledTimes(1)
        const retryUrl = new URL(fetchMock.mock.calls[1][0] as string)
        expect(retryUrl.searchParams.get('apiKey')).toBe('aether_new_key')
    })

    it('re-mints and retries through hand-rolled fetch (createPlaylist path)', async () => {
        vi.mocked(TokensApi.mintSpaToken).mockResolvedValue({
            token: 'aether_new_key', tokenId: 'tid4', expiresAt: '2999-01-01T00:00:00Z'
        })
        subsonicClient.setApiKey('aether_old_key')
        const okPlaylist = {
            'subsonic-response': { status: 'ok', playlist: { id: 'pl1', name: 'Test' } }
        }
        const fetchMock = vi.fn()
            .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(failed40) })
            .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(okPlaylist) })
        vi.stubGlobal('fetch', fetchMock)

        const result = await subsonicClient.createPlaylist('Test')
        expect(result?.id).toBe('pl1')
        expect(TokensApi.mintSpaToken).toHaveBeenCalledTimes(1)
        const retryUrl = new URL(fetchMock.mock.calls[1][0] as string)
        expect(retryUrl.searchParams.get('apiKey')).toBe('aether_new_key')
    })

    it('re-mints and retries through savePlayQueue', async () => {
        vi.mocked(TokensApi.mintSpaToken).mockResolvedValue({
            token: 'aether_new_key', tokenId: 'tid5', expiresAt: '2999-01-01T00:00:00Z'
        })
        subsonicClient.setApiKey('aether_old_key')
        const fetchMock = vi.fn()
            .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(failed40) })
            .mockResolvedValueOnce({ ok: true, json: () => Promise.resolve(okPing) })
        vi.stubGlobal('fetch', fetchMock)

        await expect(subsonicClient.savePlayQueue(['s1'], 0, 5000)).resolves.toBeUndefined()
        expect(TokensApi.mintSpaToken).toHaveBeenCalledTimes(1)
        const retryUrl = new URL(fetchMock.mock.calls[1][0] as string)
        expect(retryUrl.searchParams.get('apiKey')).toBe('aether_new_key')
    })
})
