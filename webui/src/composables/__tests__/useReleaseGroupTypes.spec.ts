import { describe, it, expect, vi, beforeEach } from 'vitest'

const typesMock = vi.fn()
vi.mock('@/lib/api/Artists', () => ({
    getReleaseGroupTypes: (...args: unknown[]) => typesMock(...args)
}))

import { useReleaseGroupTypes } from '@/composables/useReleaseGroupTypes'

const types = useReleaseGroupTypes()

beforeEach(() => {
    typesMock.mockReset()
    types.clear()
})

describe('useReleaseGroupTypes', () => {
    it('fetches the types of a release group', async () => {
        typesMock.mockResolvedValue(['Album', 'Compilation'])
        expect(await types.lookup('rg-1')).toEqual(['Album', 'Compilation'])
        expect(typesMock).toHaveBeenCalledWith('rg-1')
    })

    it('serves a second lookup of the same group from cache', async () => {
        typesMock.mockResolvedValue(['Album'])
        await types.lookup('rg-1')
        await types.lookup('rg-1')
        expect(typesMock).toHaveBeenCalledTimes(1)
    })

    it('resolves to an empty list when the lookup fails', async () => {
        typesMock.mockRejectedValue(new Error('rate limited'))
        expect(await types.lookup('rg-1')).toEqual([])
    })
})
