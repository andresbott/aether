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

describe('Metadata API — candidate image info', () => {
    it('getArtistImageCandidateInfo GETs the artist probe with mbid + url', async () => {
        const meta = { width: 1000, height: 1000, format: 'jpeg', bytes: 250880 }
        get.mockResolvedValue({ data: meta })
        const res = await Metadata.getArtistImageCandidateInfo('mbid-1', 'https://p.example/a.jpg')
        expect(get).toHaveBeenCalledWith('/metadata/artist-image/candidate-info', {
            params: { mbid: 'mbid-1', url: 'https://p.example/a.jpg' }
        })
        expect(res).toEqual(meta)
    })

    it('getPictureCandidateInfo GETs the cover probe with the url', async () => {
        const meta = { width: 500, height: 500, format: 'png', bytes: 1536 }
        get.mockResolvedValue({ data: meta })
        const res = await Metadata.getPictureCandidateInfo('https://caa.example/f.jpg')
        expect(get).toHaveBeenCalledWith('/metadata/pictures/candidate-info', {
            params: { url: 'https://caa.example/f.jpg' }
        })
        expect(res).toEqual(meta)
    })
})
