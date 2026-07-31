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

    it('never rejects when the request fails', async () => {
        vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new Error('offline'))))
        await expect(subsonicClient.scrobble('tr-1')).resolves.toBeUndefined()
    })
})
