import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { subsonicClient } from '@/lib/api/subsonic'

beforeEach(() => {
    subsonicClient.initWithDefaults()
})

afterEach(() => {
    vi.unstubAllGlobals()
})

const okFetch = () =>
    vi.fn().mockResolvedValue({
        ok: true,
        json: async () => ({ 'subsonic-response': { status: 'ok' } })
    })

describe('subsonicClient.savePlayQueue', () => {
    // The index-based variant is used deliberately: a queue may hold the same
    // track twice, and only an index says which copy is playing.
    it('sends every id plus currentIndex and position to savePlayQueueByIndex', async () => {
        const fetchMock = okFetch()
        vi.stubGlobal('fetch', fetchMock)

        await subsonicClient.savePlayQueue(['tr-1', 'tr-2', 'tr-1'], 2, 42000)

        expect(fetchMock).toHaveBeenCalledTimes(1)
        const url = new URL(fetchMock.mock.calls[0][0] as string)
        expect(url.pathname).toContain('savePlayQueueByIndex.view')
        expect(url.searchParams.getAll('id')).toEqual(['tr-1', 'tr-2', 'tr-1'])
        expect(url.searchParams.get('currentIndex')).toBe('2')
        expect(url.searchParams.get('position')).toBe('42000')
    })

    // An empty queue is the spec's clear call: no id, and currentIndex must NOT
    // be sent or the server rejects it.
    it('clears the saved queue by sending no ids and no currentIndex', async () => {
        const fetchMock = okFetch()
        vi.stubGlobal('fetch', fetchMock)

        await subsonicClient.savePlayQueue([], -1, 0)

        const url = new URL(fetchMock.mock.calls[0][0] as string)
        expect(url.searchParams.getAll('id')).toEqual([])
        expect(url.searchParams.has('currentIndex')).toBe(false)
    })

    // Queue persistence is a background convenience; a failed save must never
    // surface as an unhandled rejection in the player.
    it('never rejects when the request fails', async () => {
        vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new Error('offline'))))
        await expect(subsonicClient.savePlayQueue(['tr-1'], 0, 0)).resolves.toBeUndefined()
    })
})

describe('subsonicClient.getPlayQueue', () => {
    it('returns the queue entries, current index and position', async () => {
        vi.stubGlobal(
            'fetch',
            vi.fn().mockResolvedValue({
                ok: true,
                json: async () => ({
                    'subsonic-response': {
                        status: 'ok',
                        playQueueByIndex: {
                            currentIndex: 1,
                            position: 90000,
                            changedBy: 'other-device',
                            entry: [
                                { id: 'tr-1', title: 'One' },
                                { id: 'tr-2', title: 'Two' }
                            ]
                        }
                    }
                })
            })
        )

        const queue = await subsonicClient.getPlayQueue()
        expect(queue).not.toBeNull()
        expect(queue?.entry.map((s) => s.id)).toEqual(['tr-1', 'tr-2'])
        expect(queue?.currentIndex).toBe(1)
        expect(queue?.position).toBe(90000)
        expect(queue?.changedBy).toBe('other-device')
    })

    // A fresh account has no saved queue; that is not an error and must not be
    // reported as an empty-but-present queue.
    it('returns null when the server has no saved queue', async () => {
        vi.stubGlobal(
            'fetch',
            vi.fn().mockResolvedValue({
                ok: true,
                json: async () => ({ 'subsonic-response': { status: 'ok' } })
            })
        )
        await expect(subsonicClient.getPlayQueue()).resolves.toBeNull()
    })

    it('returns null instead of throwing when the request fails', async () => {
        vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new Error('offline'))))
        await expect(subsonicClient.getPlayQueue()).resolves.toBeNull()
    })
})
