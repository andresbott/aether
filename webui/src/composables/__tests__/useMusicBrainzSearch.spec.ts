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

    // The backend answers upstream outages with a ready-to-show sentence; a
    // double-wrapped body must never reach the user as raw JSON.
    it('surfaces the server sentence and never raw JSON', async () => {
        searchMock.mockRejectedValue({
            response: {
                status: 502,
                data: {
                    error: '{"error":"MusicBrainz is temporarily unavailable. Try again in a few minutes.","code":"upstream_error"}',
                    code: 502
                }
            }
        })
        const { search, error } = useMusicBrainzSearch()
        await search('nirvana')
        expect(error.value?.startsWith('{')).toBe(false)
        expect(error.value).toBe('MusicBrainz is temporarily unavailable. Try again in a few minutes.')
    })

    it('flags a rate-limited search', async () => {
        searchMock.mockRejectedValue({
            response: {
                status: 429,
                data: { error: 'MusicBrainz is receiving too many requests right now.', code: 'upstream_rate_limited' }
            }
        })
        const { search, rateLimited } = useMusicBrainzSearch()
        await search('nirvana')
        expect(rateLimited.value).toBe(true)
    })
})
