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

describe('subsonicClient.updatePlaylist', () => {
    beforeEach(() => subsonicClient.initWithDefaults())
    afterEach(() => vi.unstubAllGlobals())

    it('sends the public flag when provided', async () => {
        const fetchMock = mockFetchOnce({})
        await subsonicClient.updatePlaylist('pl-7', { public: true })
        const params = new URL(fetchMock.mock.calls[0][0] as string).searchParams
        expect(params.get('playlistId')).toBe('pl-7')
        expect(params.get('public')).toBe('true')
    })

    it('omits the public flag when it is not provided', async () => {
        const fetchMock = mockFetchOnce({})
        await subsonicClient.updatePlaylist('pl-7', { name: 'Renamed' })
        const params = new URL(fetchMock.mock.calls[0][0] as string).searchParams
        expect(params.has('public')).toBe(false)
    })
})

describe('subsonicClient generated cover support', () => {
    beforeEach(() => subsonicClient.initWithDefaults())
    afterEach(() => vi.unstubAllGlobals())

    it('getGeneratedCoverCandidates unwraps the candidate array', async () => {
        const fetchMock = mockFetchOnce({
            generatedCoverCandidates: {
                candidate: [
                    { style: 'bauhaus', variation: 1 },
                    { style: 'rings', variation: 2 }
                ]
            }
        })
        const result = await subsonicClient.getGeneratedCoverCandidates('al-1', 9)
        expect(result).toEqual([
            { style: 'bauhaus', variation: 1 },
            { style: 'rings', variation: 2 }
        ])
        const url = new URL(fetchMock.mock.calls[0][0] as string)
        expect(url.pathname).toContain('/rest/getGeneratedCoverCandidates.view')
        expect(url.searchParams.get('id')).toBe('al-1')
        expect(url.searchParams.get('count')).toBe('9')
    })

    it('getGeneratedCoverCandidates returns empty array when response omits candidates', async () => {
        mockFetchOnce({})
        const result = await subsonicClient.getGeneratedCoverCandidates('al-1')
        expect(result).toEqual([])
    })

    it('getGeneratedCoverPreviewUrl builds a preview URL with all params', () => {
        subsonicClient.initWithDefaults()
        const url = new URL(subsonicClient.getGeneratedCoverPreviewUrl({
            id: 'al-1',
            style: 'bauhaus',
            variation: 3,
            size: 300
        }))
        expect(url.pathname).toContain('/rest/getGeneratedCoverPreview.view')
        expect(url.searchParams.get('id')).toBe('al-1')
        expect(url.searchParams.get('style')).toBe('bauhaus')
        expect(url.searchParams.get('variation')).toBe('3')
        expect(url.searchParams.get('size')).toBe('300')
    })

    it('getGeneratedCoverPreviewUrl omits id when not provided', () => {
        subsonicClient.initWithDefaults()
        const url = new URL(subsonicClient.getGeneratedCoverPreviewUrl({
            style: 'poster',
            variation: 1
        }))
        expect(url.searchParams.has('id')).toBe(false)
        expect(url.searchParams.get('style')).toBe('poster')
        expect(url.searchParams.get('variation')).toBe('1')
    })

    it('updateGenreCover posts generateStyle and generateVariation when generate is provided', async () => {
        const fetchMock = mockFetchOnce({})
        await subsonicClient.updateGenreCover('ge-1', undefined, false, {
            style: 'bauhaus',
            variation: 5
        })
        const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit]
        expect(url).toContain('/rest/updateGenre.view')
        const body = init.body as FormData
        expect(body.get('id')).toBe('ge-1')
        expect(body.get('generateStyle')).toBe('bauhaus')
        expect(body.get('generateVariation')).toBe('5')
    })

    it('updateAlbumCover posts generateStyle and generateVariation when generate is provided', async () => {
        const fetchMock = mockFetchOnce({})
        await subsonicClient.updateAlbumCover('al-1', undefined, false, {
            style: 'rings',
            variation: 2
        })
        const body = (fetchMock.mock.calls[0][1] as RequestInit).body as FormData
        expect(body.get('generateStyle')).toBe('rings')
        expect(body.get('generateVariation')).toBe('2')
    })

    it('updateArtistCover posts generateStyle and generateVariation when generate is provided', async () => {
        const fetchMock = mockFetchOnce({})
        await subsonicClient.updateArtistCover('ar-1', undefined, false, {
            style: 'waves',
            variation: 3
        })
        const body = (fetchMock.mock.calls[0][1] as RequestInit).body as FormData
        expect(body.get('generateStyle')).toBe('waves')
        expect(body.get('generateVariation')).toBe('3')
    })

    it('updatePlaylistCover posts generateStyle and generateVariation when generate is provided', async () => {
        const fetchMock = mockFetchOnce({})
        await subsonicClient.updatePlaylistCover('pl-1', undefined, false, {
            style: 'poster',
            variation: 1
        })
        const body = (fetchMock.mock.calls[0][1] as RequestInit).body as FormData
        expect(body.get('generateStyle')).toBe('poster')
        expect(body.get('generateVariation')).toBe('1')
    })

    it('updateInternetRadioStation posts generateStyle and generateVariation when generate is provided', async () => {
        const fetchMock = mockFetchOnce({})
        await subsonicClient.updateInternetRadioStation('rs-1', 'Radio', 'http://stream', undefined, undefined, false, {
            style: 'remix',
            variation: 4
        })
        const body = (fetchMock.mock.calls[0][1] as RequestInit).body as FormData
        expect(body.get('generateStyle')).toBe('remix')
        expect(body.get('generateVariation')).toBe('4')
    })
})
