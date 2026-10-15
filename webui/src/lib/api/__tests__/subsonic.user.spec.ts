import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { subsonicClient } from '@/lib/api/subsonic'
import { resetSubsonicSession } from '@/lib/subsonicSession'

// Aether's getUser: the caller's own record, whatever `username` says.
const okUser = {
    'subsonic-response': {
        status: 'ok',
        version: '1.16.1',
        type: 'Aether',
        serverVersion: '0.4.0',
        user: { username: 'alice', adminRole: true, settingsRole: true, streamRole: true }
    }
}
const okPing = {
    'subsonic-response': { status: 'ok', version: '1.16.1', type: 'Aether', serverVersion: '0.4.0' }
}
const bareOk = { 'subsonic-response': { status: 'ok', version: '1.16.1' } }

function answer(body: unknown) {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(body) })
    vi.stubGlobal('fetch', fetchMock)
    return fetchMock
}

beforeEach(() => {
    resetSubsonicSession()
    subsonicClient.setApiKey('aether_k3y')
})

afterEach(() => {
    vi.unstubAllGlobals()
})

describe('subsonicClient.getUser', () => {
    it('asks getUser with the key and no username, and answers the record', async () => {
        const fetchMock = answer(okUser)

        await expect(subsonicClient.getUser()).resolves.toEqual(okUser['subsonic-response'].user)

        const url = new URL(fetchMock.mock.calls[0][0] as string)
        expect(url.pathname).toBe('/rest/getUser.view')
        expect(url.searchParams.get('apiKey')).toBe('aether_k3y')
        expect(url.searchParams.has('username')).toBe(false)
    })

    // The key session's bootstrap: a server that drops packets must not hold the
    // window blank for minutes, as getMe is bounded for the cookie flow.
    it('bounds the request with a timeout signal', async () => {
        const fetchMock = answer(okUser)

        await subsonicClient.getUser()

        const init = fetchMock.mock.calls[0][1] as RequestInit | undefined
        expect(init?.signal).toBeInstanceOf(AbortSignal)
    })
})

describe('subsonicClient.serverVersion', () => {
    it('reads the version off the ping envelope', async () => {
        answer(okPing)
        await expect(subsonicClient.serverVersion()).resolves.toBe('0.4.0')
    })

    it('answers null when the envelope carries none', async () => {
        answer(bareOk)
        await expect(subsonicClient.serverVersion()).resolves.toBeNull()
    })
})
