import { describe, it, expect, vi, beforeEach } from 'vitest'

const get = vi.fn()

vi.mock('@/lib/api/client', () => ({
    apiClient: {
        get: (...a: unknown[]) => get(...a)
    }
}))

import * as Metadata from '@/lib/api/Metadata'

beforeEach(() => {
    get.mockReset()
})

describe('Metadata API — searchFolders', () => {
    it('GETs /metadata/folders with the query and returns folders + truncated', async () => {
        get.mockResolvedValue({
            data: {
                folders: [{ name: 'fire up', path: 'Alexia dixon/fire up', has_subfolders: false }],
                truncated: true
            }
        })
        const res = await Metadata.searchFolders(7, 'up')
        expect(get).toHaveBeenCalledWith('/metadata/folders', {
            params: { library_id: 7, q: 'up' }
        })
        expect(res.folders.map((f) => f.path)).toEqual(['Alexia dixon/fire up'])
        expect(res.truncated).toBe(true)
    })

    it('defaults truncated to false when the server omits it', async () => {
        get.mockResolvedValue({ data: { folders: [] } })
        const res = await Metadata.searchFolders(1, 'x')
        expect(res.truncated).toBe(false)
    })
})
