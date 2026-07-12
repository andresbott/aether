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
})
