import { describe, it, expect, vi, beforeEach } from 'vitest'

const searchMock = vi.fn()
vi.mock('@/lib/api/Artists', () => ({
    searchMusicBrainzReleases: (...args: unknown[]) => searchMock(...args)
}))

import { useMusicBrainzReleaseSearch } from '@/composables/useMusicBrainzReleaseSearch'

beforeEach(() => {
    searchMock.mockReset()
})

const candidate = {
    releaseMbid: 'rel-1',
    releaseGroupMbid: 'rg-1',
    title: 'OK Computer',
    artist: 'Radiohead',
    date: '1997',
    country: 'GB',
    trackCount: 12,
    disambiguation: '',
    score: 100
}

describe('useMusicBrainzReleaseSearch', () => {
    it('does not call the API for a query shorter than 2 characters', async () => {
        const { search, results } = useMusicBrainzReleaseSearch()
        await search('a')
        expect(searchMock).not.toHaveBeenCalled()
        expect(results.value).toEqual([])
    })

    it('populates results on a successful search', async () => {
        searchMock.mockResolvedValue([candidate])
        const { search, results, loading, error } = useMusicBrainzReleaseSearch()
        const p = search('OK Computer')
        expect(loading.value).toBe(true)
        await p
        expect(loading.value).toBe(false)
        expect(error.value).toBeNull()
        expect(results.value).toHaveLength(1)
        expect(results.value[0].title).toBe('OK Computer')
    })

    it('sets error and clears results on a failed search', async () => {
        searchMock.mockRejectedValue(new Error('network down'))
        const { search, results, error } = useMusicBrainzReleaseSearch()
        await search('OK Computer')
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
                    type: 'https://aether.local/probs/upstream_error',
                    title: 'Upstream error',
                    status: 502,
                    detail:
                        '{"type":"https://aether.local/probs/upstream_error","title":"Upstream error","status":500,"detail":"MusicBrainz is temporarily unavailable. Try again in a few minutes."}'
                }
            }
        })
        const { search, error } = useMusicBrainzReleaseSearch()
        await search('nirvana')
        expect(error.value?.startsWith('{')).toBe(false)
        expect(error.value).toBe('MusicBrainz is temporarily unavailable. Try again in a few minutes.')
    })

    it('flags a rate-limited search', async () => {
        searchMock.mockRejectedValue({
            response: {
                status: 429,
                data: {
                    type: 'https://aether.local/probs/upstream_rate_limited',
                    title: 'Too Many Requests',
                    status: 429,
                    detail: 'MusicBrainz is receiving too many requests right now.'
                }
            }
        })
        const { search, rateLimited } = useMusicBrainzReleaseSearch()
        await search('nirvana')
        expect(rateLimited.value).toBe(true)
    })
})
