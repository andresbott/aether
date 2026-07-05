import { describe, it, expect, vi, beforeEach } from 'vitest'

const searchMock = vi.fn()
vi.mock('@/lib/api/Artists', () => ({
    searchMusicBrainzArtists: (...args: unknown[]) => searchMock(...args)
}))

import { useMusicBrainzSearch } from '@/composables/useMusicBrainzSearch'

beforeEach(() => {
    searchMock.mockReset()
})

describe('useMusicBrainzSearch', () => {
    it('does not call the API for a query shorter than 2 characters', async () => {
        const { search, results } = useMusicBrainzSearch()
        await search('a')
        expect(searchMock).not.toHaveBeenCalled()
        expect(results.value).toEqual([])
    })

    it('populates results on a successful search', async () => {
        searchMock.mockResolvedValue([{ mbid: 'abc', name: 'Nirvana', type: 'Group', disambiguation: '', lifeSpanBegin: '', lifeSpanEnd: '', score: 100 }])
        const { search, results, loading, error } = useMusicBrainzSearch()
        const p = search('Nirvana')
        expect(loading.value).toBe(true)
        await p
        expect(loading.value).toBe(false)
        expect(error.value).toBeNull()
        expect(results.value).toHaveLength(1)
        expect(results.value[0].name).toBe('Nirvana')
    })

    it('sets error and clears results on a failed search', async () => {
        searchMock.mockRejectedValue(new Error('network down'))
        const { search, results, error } = useMusicBrainzSearch()
        await search('Nirvana')
        expect(error.value).toBe('network down')
        expect(results.value).toEqual([])
    })
})
