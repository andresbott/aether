import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { subsonicClient } from '@/lib/api/subsonic'

const okResponse = {
    'subsonic-response': {
        status: 'ok',
        version: '1.16.1',
        searchResult3: {
            song: [
                { id: 'tr-1', title: 'Song 1', artist: 'Artist 1' },
                { id: 'tr-2', title: 'Song 2', artist: 'Artist 2' }
            ]
        }
    }
}

describe('subsonic client search3', () => {
    beforeEach(() => {
        vi.stubGlobal(
            'fetch',
            vi.fn().mockResolvedValue({
                ok: true,
                json: () => Promise.resolve(okResponse)
            })
        )
        subsonicClient.setApiKey('test-key')
    })
    afterEach(() => {
        vi.unstubAllGlobals()
        subsonicClient.clearApiKey()
    })

    it('sends query, songCount, and songOffset params', async () => {
        await subsonicClient.search3('', 50, 0)
        const url = new URL(vi.mocked(fetch).mock.calls[0][0] as string)
        expect(url.searchParams.get('query')).toBe('')
        expect(url.searchParams.get('songCount')).toBe('50')
        expect(url.searchParams.get('songOffset')).toBe('0')
    })

    it('sends musicFolderId when folderId is provided', async () => {
        await subsonicClient.search3('', 50, 0, 123)
        const url = new URL(vi.mocked(fetch).mock.calls[0][0] as string)
        expect(url.searchParams.get('musicFolderId')).toBe('123')
    })

    it('omits musicFolderId when folderId is undefined', async () => {
        await subsonicClient.search3('', 50, 0, undefined)
        const url = new URL(vi.mocked(fetch).mock.calls[0][0] as string)
        expect(url.searchParams.has('musicFolderId')).toBe(false)
    })

    it('sends the correct endpoint', async () => {
        await subsonicClient.search3('', 50, 0)
        const url = new URL(vi.mocked(fetch).mock.calls[0][0] as string)
        expect(url.pathname).toBe('/rest/search3.view')
    })

    it('returns the searchResult3 from the response', async () => {
        const result = await subsonicClient.search3('', 50, 0)
        expect(result).toEqual(okResponse['subsonic-response'].searchResult3)
    })

    it('returns an empty object when not configured', async () => {
        subsonicClient.clearApiKey()
        const result = await subsonicClient.search3('', 50, 0)
        expect(result).toEqual({})
        expect(fetch).not.toHaveBeenCalled()
    })

    it('handles non-zero offsets correctly', async () => {
        await subsonicClient.search3('', 50, 100)
        const url = new URL(vi.mocked(fetch).mock.calls[0][0] as string)
        expect(url.searchParams.get('songOffset')).toBe('100')
    })

    it('accepts an empty query string', async () => {
        await subsonicClient.search3('', 50, 0)
        const url = new URL(vi.mocked(fetch).mock.calls[0][0] as string)
        expect(url.searchParams.get('query')).toBe('')
    })
})
