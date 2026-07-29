import { describe, it, expect, vi, beforeEach } from 'vitest'

const genresMock = vi.fn()
vi.mock('@/lib/api/Artists', () => ({
    getReleaseGroupGenres: (...args: unknown[]) => genresMock(...args)
}))

import {
    MAX_GENRE_ENTRIES,
    useReleaseGroupGenres
} from '@/composables/useReleaseGroupGenres'

const genres = useReleaseGroupGenres()

beforeEach(() => {
    genresMock.mockReset()
    genres.clear()
})

describe('useReleaseGroupGenres lookup', () => {
    it('fetches the genres of a release group', async () => {
        genresMock.mockResolvedValue(['Grunge', 'Alternative Rock'])
        expect(await genres.lookup('rg-1')).toEqual(['Grunge', 'Alternative Rock'])
        expect(genresMock).toHaveBeenCalledWith('rg-1')
    })

    it('makes no request for an empty mbid', async () => {
        expect(await genres.lookup('')).toEqual([])
        expect(genresMock).not.toHaveBeenCalled()
    })

    it('serves a second lookup of the same group from cache', async () => {
        // MusicBrainz is throttled to one request per second, and reopening a
        // dialog re-asks for the same release group every time.
        genresMock.mockResolvedValue(['Grunge'])
        await genres.lookup('rg-1')
        await genres.lookup('rg-1')
        expect(genresMock).toHaveBeenCalledTimes(1)
    })

    it('caches an empty answer — "this group has no genres" is an answer', async () => {
        genresMock.mockResolvedValue([])
        expect(await genres.lookup('rg-1')).toEqual([])
        expect(await genres.lookup('rg-1')).toEqual([])
        expect(genresMock).toHaveBeenCalledTimes(1)
    })

    it('shares one in-flight request between concurrent lookups', async () => {
        // Both identify dialogs can ask for the same group while the first
        // request is still open — a 12-song selection usually resolves to one.
        genresMock.mockResolvedValue(['Grunge'])
        const [a, b] = await Promise.all([genres.lookup('rg-1'), genres.lookup('rg-1')])
        expect(a).toEqual(['Grunge'])
        expect(b).toEqual(['Grunge'])
        expect(genresMock).toHaveBeenCalledTimes(1)
    })

    it('resolves to no genres when the lookup fails', async () => {
        // Genres are a nice-to-have: a failed lookup stages nothing, it does not
        // sink the identify apply the user is in the middle of.
        genresMock.mockRejectedValue(new Error('rate limited'))
        expect(await genres.lookup('rg-1')).toEqual([])
    })

    it('does not cache a failure, so a retry can still succeed', async () => {
        genresMock.mockRejectedValueOnce(new Error('rate limited'))
        genresMock.mockResolvedValueOnce(['Grunge'])
        expect(await genres.lookup('rg-1')).toEqual([])
        expect(await genres.lookup('rg-1')).toEqual(['Grunge'])
    })

    it('does not leak a failed request to a later caller', async () => {
        // The in-flight entry must be dropped on rejection too, or every
        // subsequent lookup would await the same dead promise.
        genresMock.mockRejectedValue(new Error('rate limited'))
        await genres.lookup('rg-1')
        genresMock.mockResolvedValue(['Grunge'])
        expect(await genres.lookup('rg-1')).toEqual(['Grunge'])
    })

    it('keys the cache by release group', async () => {
        genresMock.mockResolvedValueOnce(['Grunge'])
        genresMock.mockResolvedValueOnce(['Jazz'])
        expect(await genres.lookup('rg-1')).toEqual(['Grunge'])
        expect(await genres.lookup('rg-2')).toEqual(['Jazz'])
    })

    it('hands out a copy, so a caller cannot mutate the cached entry', async () => {
        genresMock.mockResolvedValue(['Grunge'])
        const first = await genres.lookup('rg-1')
        first.push('Injected')
        expect(await genres.lookup('rg-1')).toEqual(['Grunge'])
    })
})

describe('useReleaseGroupGenres cached', () => {
    it('has nothing for a group it has not looked up', () => {
        expect(genres.cached('rg-1')).toBeUndefined()
    })

    it('reads a stored answer without making a request', async () => {
        genresMock.mockResolvedValue(['Grunge'])
        await genres.lookup('rg-1')
        genresMock.mockReset()
        expect(genres.cached('rg-1')).toEqual(['Grunge'])
        expect(genresMock).not.toHaveBeenCalled()
    })

    it('drops the least recently used entry once the cap is reached', async () => {
        genresMock.mockResolvedValue(['Grunge'])
        for (let i = 0; i < MAX_GENRE_ENTRIES; i++) {
            await genres.lookup(`rg-${i}`)
        }
        // Touch the oldest so it is no longer the least recently used.
        expect(genres.cached('rg-0')).toBeDefined()
        await genres.lookup('rg-overflow')
        expect(genres.cached('rg-0')).toBeDefined()
        expect(genres.cached('rg-1')).toBeUndefined()
        expect(genres.cached('rg-overflow')).toBeDefined()
    })
})

describe('useReleaseGroupGenres clear', () => {
    it('empties the cache so the next lookup refetches', async () => {
        genresMock.mockResolvedValue(['Grunge'])
        await genres.lookup('rg-1')
        genres.clear()
        expect(genres.cached('rg-1')).toBeUndefined()
        await genres.lookup('rg-1')
        expect(genresMock).toHaveBeenCalledTimes(2)
    })
})
