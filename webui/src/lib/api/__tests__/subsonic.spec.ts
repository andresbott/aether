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

describe('subsonicClient.getArtistIndex', () => {
    beforeEach(() => subsonicClient.initWithDefaults())
    afterEach(() => vi.unstubAllGlobals())

    const grouped = {
        artists: {
            index: [
                { name: 'A', artist: [{ id: 'ar1', name: 'ABBA' }, { id: 'ar2', name: 'Air' }] },
                { name: 'B', artist: [{ id: 'ar3', name: 'Beck' }] },
                { name: 'E', artist: [] } // empty group is skipped
            ]
        }
    }

    it('parses groups into total, letters (cumulative offsets) and flattened items', async () => {
        mockFetchOnce(grouped)
        const res = await subsonicClient.getArtistIndex(1)
        expect(res.total).toBe(3)
        expect(res.letters).toEqual([
            { name: 'A', offset: 0, count: 2 },
            { name: 'B', offset: 2, count: 1 }
        ])
        expect(res.items.map((a) => a.id)).toEqual(['ar1', 'ar2', 'ar3'])
    })

    it('getArtists returns the flattened list (delegates to getArtistIndex)', async () => {
        mockFetchOnce(grouped)
        const artists = await subsonicClient.getArtists()
        expect(artists.map((a) => a.id)).toEqual(['ar1', 'ar2', 'ar3'])
    })
})

describe('subsonicClient.replacePlaylistTracks', () => {
    beforeEach(() => subsonicClient.initWithDefaults())
    afterEach(() => vi.unstubAllGlobals())

    it('posts the full ordered song set to createPlaylist with the playlistId', async () => {
        const fetchMock = mockFetchOnce({})
        await subsonicClient.replacePlaylistTracks('pl-7', ['s1', 's2', 's3'])
        const url = fetchMock.mock.calls[0][0] as string
        expect(url).toContain('/rest/createPlaylist.view')
        expect(url).toContain('playlistId=pl-7')
        const params = new URL(url).searchParams
        expect(params.getAll('songId')).toEqual(['s1', 's2', 's3'])
    })
})
