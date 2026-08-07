import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { subsonicClient } from '@/lib/api/subsonic'

const okResponse = () =>
    Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ 'subsonic-response': { status: 'ok' } })
    } as Response)

beforeEach(() => {
    subsonicClient.initWithDefaults()
})

afterEach(() => {
    vi.unstubAllGlobals()
})

describe('subsonicClient.scrobble', () => {
    it('posts the id with submission=true', async () => {
        const fetchMock = vi.fn().mockResolvedValue({
            ok: true,
            json: async () => ({ 'subsonic-response': { status: 'ok' } })
        })
        vi.stubGlobal('fetch', fetchMock)

        await subsonicClient.scrobble('pl-7')

        expect(fetchMock).toHaveBeenCalledTimes(1)
        const url = (fetchMock.mock.calls[0][0] as string)
        expect(url).toContain('scrobble.view')
        expect(url).toContain('id=pl-7')
        expect(url).toContain('submission=true')
    })

    it('never rejects when the request fails, and warns instead', async () => {
        vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new Error('offline'))))
        // The warn is the intended behaviour, so assert it rather than let it
        // print: an expected failure must not look like a broken test run.
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
        await expect(subsonicClient.scrobble('tr-1')).resolves.toBeUndefined()
        expect(warn).toHaveBeenCalledWith('scrobble failed', expect.any(Error))
        warn.mockRestore()
    })
})
