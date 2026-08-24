import { describe, it, expect, vi, beforeEach } from 'vitest'

const listMock = vi.fn()
vi.mock('@/lib/api/Metadata', () => ({
    listReleaseCovers: (...args: unknown[]) => listMock(...args)
}))

import { useCoverArtSearch } from '@/composables/useCoverArtSearch'

beforeEach(() => {
    listMock.mockReset()
})

const candidate = {
    id: '1',
    thumbUrl: 'http://img/f-250.jpg',
    imageUrl: 'http://img/f.jpg',
    isFront: true
}

describe('useCoverArtSearch', () => {
    it('does not call the API when both ids are empty', async () => {
        const { search, candidates } = useCoverArtSearch()
        await search('', '')
        expect(listMock).not.toHaveBeenCalled()
        expect(candidates.value).toEqual([])
    })

    it('populates candidates on a successful lookup', async () => {
        listMock.mockResolvedValue([candidate])
        const { search, candidates, loading, error } = useCoverArtSearch()
        const p = search('rel-1', 'rg-1')
        expect(loading.value).toBe(true)
        await p
        expect(loading.value).toBe(false)
        expect(error.value).toBeNull()
        expect(candidates.value).toHaveLength(1)
        expect(listMock).toHaveBeenCalledWith('rel-1', 'rg-1')
    })

    it('passes undefined release-group when only a release id is given', async () => {
        listMock.mockResolvedValue([])
        const { search } = useCoverArtSearch()
        await search('rel-1')
        expect(listMock).toHaveBeenCalledWith('rel-1', undefined)
    })

    it('sets error and clears candidates on failure', async () => {
        listMock.mockRejectedValue(new Error('network down'))
        const { search, candidates, error } = useCoverArtSearch()
        await search('rel-1')
        expect(error.value).toBe('network down')
        expect(candidates.value).toEqual([])
    })

    // The backend sends a ready-to-show sentence for an upstream outage; the
    // composable must surface exactly that.
    it('surfaces the server sentence for an upstream failure', async () => {
        listMock.mockRejectedValue({
            response: {
                status: 502,
                data: {
                    type: 'https://aether.local/probs/upstream_error',
                    title: 'Upstream error',
                    status: 502,
                    detail: 'Cover Art Archive is temporarily unavailable. Try again in a few minutes.'
                }
            }
        })
        const { search, error, rateLimited } = useCoverArtSearch()
        await search('rel-1')
        expect(error.value).toBe(
            'Cover Art Archive is temporarily unavailable. Try again in a few minutes.'
        )
        expect(rateLimited.value).toBe(false)
    })

    // Regression for the raw-JSON bug: a double-wrapped body must still render
    // as a sentence, never as a JSON document.
    it('never exposes a raw JSON envelope as the error text', async () => {
        listMock.mockRejectedValue({
            response: {
                status: 502,
                data: {
                    type: 'https://aether.local/probs/upstream_error',
                    title: 'Upstream error',
                    status: 502,
                    detail:
                        '{"type":"https://aether.local/probs/upstream_error","title":"Upstream error","status":500,"detail":"cover art archive lookup failed: status 500"}'
                }
            }
        })
        const { search, error } = useCoverArtSearch()
        await search('rel-1')
        expect(error.value?.startsWith('{')).toBe(false)
        expect(error.value).toBe('cover art archive lookup failed: status 500')
    })

    it('flags a rate-limited lookup so the UI can invite a retry', async () => {
        listMock.mockRejectedValue({
            response: {
                status: 429,
                data: {
                    type: 'https://aether.local/probs/upstream_rate_limited',
                    title: 'Too Many Requests',
                    status: 429,
                    detail:
                        'Cover Art Archive is receiving too many requests right now. Wait a moment and try again.'
                }
            }
        })
        const { search, rateLimited, error } = useCoverArtSearch()
        await search('rel-1')
        expect(rateLimited.value).toBe(true)
        expect(error.value).toContain('too many requests')
    })

    it('clears a previous error when a later search succeeds', async () => {
        listMock.mockRejectedValueOnce({
            response: {
                status: 429,
                data: {
                    type: 'https://aether.local/probs/upstream_rate_limited',
                    title: 'Too Many Requests',
                    status: 429,
                    detail: 'slow down'
                }
            }
        })
        const { search, error, rateLimited } = useCoverArtSearch()
        await search('rel-1')
        expect(rateLimited.value).toBe(true)

        listMock.mockResolvedValueOnce([candidate])
        await search('rel-2')
        expect(error.value).toBeNull()
        expect(rateLimited.value).toBe(false)
    })
})
