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

describe('subsonicClient genres', () => {
    beforeEach(() => subsonicClient.initWithDefaults())
    afterEach(() => vi.unstubAllGlobals())

    it('getGenres unwraps genres.genre including coverArt', async () => {
        const fetchMock = mockFetchOnce({
            genres: {
                genre: [{ value: 'Rock', songCount: 5, albumCount: 2, coverArt: 'ge-1' }]
            }
        })
        const genres = await subsonicClient.getGenres()
        expect(genres).toEqual([
            { value: 'Rock', songCount: 5, albumCount: 2, coverArt: 'ge-1' }
        ])
        expect(fetchMock.mock.calls[0][0] as string).toContain('/rest/getGenres.view')
    })

    it('getSongsByGenre passes genre, count and offset', async () => {
        const fetchMock = mockFetchOnce({
            songsByGenre: { song: [{ id: 'tr-1', title: 'Song' }] }
        })
        const songs = await subsonicClient.getSongsByGenre('Rock', 100, 200)
        expect(songs).toEqual([{ id: 'tr-1', title: 'Song' }])
        const params = new URL(fetchMock.mock.calls[0][0] as string).searchParams
        expect(params.get('genre')).toBe('Rock')
        expect(params.get('count')).toBe('100')
        expect(params.get('offset')).toBe('200')
    })

    it('updateGenreCover posts multipart with id and coverClear', async () => {
        const fetchMock = mockFetchOnce({})
        await subsonicClient.updateGenreCover('ge-3', undefined, true)
        const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
        expect(url).toContain('/rest/updateGenre.view')
        expect(init.method).toBe('POST')
        const body = init.body as FormData
        expect(body.get('id')).toBe('ge-3')
        expect(body.get('coverClear')).toBe('true')
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
