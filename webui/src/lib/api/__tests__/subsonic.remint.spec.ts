import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { subsonicClient } from '@/lib/api/subsonic'
import {
    MAX_MINT_ATTEMPTS,
    mintRetriesExhausted,
    remintApiKey,
    resetMintAttempts,
    resetSubsonicSession,
    spaTokenId
} from '@/lib/subsonicSession'
import { sessionExpired } from '@/lib/authState'
import { serverUrl } from '@/lib/host'
import * as TokensApi from '@/lib/api/Tokens'

vi.mock('@/lib/api/Tokens', () => ({
    mintSpaToken: vi.fn()
}))

const failed40 = {
    'subsonic-response': { status: 'failed', error: { code: 40, message: 'authentication required' } }
}
const failed44 = {
    'subsonic-response': { status: 'failed', error: { code: 44, message: 'Invalid API key' } }
}
const okPing = { 'subsonic-response': { status: 'ok' } }

describe('transparent re-mint', () => {
    beforeEach(() => {
        resetSubsonicSession()
        sessionExpired.value = false
    })
    afterEach(() => {
        serverUrl.value = window.location.origin
        vi.unstubAllGlobals()
        vi.clearAllMocks()
    })

    // A /rest answer speaks for the session that sent its request. A logout, an
    // expiry or a server switch may overtake it: the 44 that server A answers to
    // a request sent while the user was leaving it can arrive once a session on
    // server B is running. Acted on, it would mint on B and retry the request —
    // and the retry rebuilds A's URL, so it would carry B's fresh token to A.
    it('acts on no auth failure that a session reset overtook', async () => {
        vi.mocked(TokensApi.mintSpaToken).mockResolvedValue({
            token: 'aether_b_fresh',
            tokenId: 'tid-b',
            expiresAt: '2999-01-01T00:00:00Z'
        })
        serverUrl.value = 'http://a.lan:8075'
        subsonicClient.setApiKey('aether_a_key')
        let answerFromA!: (response: unknown) => void
        const fetchMock = vi
            .fn()
            .mockReturnValueOnce(new Promise((resolve) => (answerFromA = resolve)))
            .mockResolvedValue({ ok: true, json: () => Promise.resolve(okPing) })
        vi.stubGlobal('fetch', fetchMock)

        const request = subsonicClient.ping()
        // The user leaves A and is signed in to B before A answers…
        resetSubsonicSession()
        serverUrl.value = 'http://b.lan:8075'
        subsonicClient.setApiKey('aether_b_key')
        // …which it does with a 44: the logout revoked A's token.
        answerFromA({ ok: true, json: () => Promise.resolve(failed44) })

        const pinged = await request
        const keysSentToA = fetchMock.mock.calls
            .map(([url]) => new URL(url as string))
            .filter((url) => url.host === 'a.lan:8075')
            .map((url) => url.searchParams.get('apiKey'))
        expect(keysSentToA).toEqual(['aether_a_key'])
        expect(TokensApi.mintSpaToken).not.toHaveBeenCalled()
        expect(sessionExpired.value).toBe(false)
        // the overtaken request itself fails: it belongs to a session that is over
        expect(pinged).toBe(false)
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

    // A failure belongs to its session just as a result does: a mint that a
    // logout, an expiry or a server switch overtook flags nothing when it is
    // refused, and is no failure of the session running now.
    describe('a mint the session reset overtook', () => {
        let failMint!: (reason: unknown) => void

        beforeEach(() => {
            resetMintAttempts()
            vi.mocked(TokensApi.mintSpaToken).mockImplementation(
                () =>
                    new Promise((_resolve, reject) => {
                        failMint = reject
                    })
            )
        })

        it('flags no expiry when it answers 401', async () => {
            const mint = remintApiKey()
            resetSubsonicSession()
            failMint({ response: { status: 401 } })

            await expect(mint).resolves.toBe('failed')
            expect(sessionExpired.value).toBe(false)
        })

        it('spends none of the retry budget when it fails', async () => {
            for (let i = 0; i < MAX_MINT_ATTEMPTS; i++) {
                const mint = remintApiKey()
                resetSubsonicSession()
                failMint({ response: { status: 500 } })
                await expect(mint).resolves.toBe('failed')
            }

            expect(mintRetriesExhausted()).toBe(false)
        })
    })

    // The next session mints for itself. Joining the mint still out would hand
    // it that mint's discarded outcome, which its caller reads as a failure.
    it('mints afresh after a reset rather than joining the mint still out', async () => {
        const minted = (id: string) => ({
            token: `aether_${id}`,
            tokenId: id,
            expiresAt: '2999-01-01T00:00:00Z'
        })
        let finishOvertaken!: (value: ReturnType<typeof minted>) => void
        let finishFresh!: (value: ReturnType<typeof minted>) => void
        vi.mocked(TokensApi.mintSpaToken)
            .mockReturnValueOnce(new Promise((resolve) => (finishOvertaken = resolve)))
            .mockReturnValueOnce(new Promise((resolve) => (finishFresh = resolve)))
        const overtaken = remintApiKey()
        resetSubsonicSession()

        const fresh = remintApiKey()
        finishOvertaken(minted('tid-overtaken'))
        await expect(overtaken).resolves.toBe('failed')
        // Settling, the overtaken mint leaves the fresh one in place to join.
        expect(remintApiKey()).toBe(fresh)
        finishFresh(minted('tid-fresh'))

        await expect(fresh).resolves.toBe('ok')
        expect(TokensApi.mintSpaToken).toHaveBeenCalledTimes(2)
        expect(spaTokenId.value).toBe('tid-fresh')
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
