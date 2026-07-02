import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { subsonicClient } from '@/lib/api/subsonic'

function mockFetchOnce(payload: unknown) {
    const fetchMock = vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ 'subsonic-response': { status: 'ok', ...(payload as object) } })
    })
    vi.stubGlobal('fetch', fetchMock)
    return fetchMock
}

describe('subsonicClient.getAlbumIndex', () => {
    beforeEach(() => subsonicClient.initWithDefaults())
    afterEach(() => vi.unstubAllGlobals())

    it('requests getAlbumList2Index and unwraps the index', async () => {
        const fetchMock = mockFetchOnce({
            albumList2Index: { total: 2, index: [{ name: 'A', offset: 0, count: 2 }] }
        })
        const result = await subsonicClient.getAlbumIndex(1)
        expect(result).toEqual({ total: 2, index: [{ name: 'A', offset: 0, count: 2 }] })
        const url = (fetchMock.mock.calls[0][0] as string)
        expect(url).toContain('/rest/getAlbumList2Index.view')
        expect(url).toContain('musicFolderId=1')
    })

    it('returns an empty index when the response omits it', async () => {
        mockFetchOnce({})
        const result = await subsonicClient.getAlbumIndex()
        expect(result).toEqual({ total: 0, index: [] })
    })
})
