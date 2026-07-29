import { describe, it, expect, beforeEach } from 'vitest'
import {
    MAX_ALBUM_ENTRIES,
    MAX_TRACK_ENTRIES,
    mergeTrackResults,
    useIdentifyCache
} from '@/composables/useIdentifyCache'
import type { IdentifyAlbumResponse, IdentifyTrackResult } from '@/types/metadata'

const cache = useIdentifyCache()

const mkResult = (path: string, over: Partial<IdentifyTrackResult> = {}): IdentifyTrackResult => ({
    path,
    candidates: [
        {
            recording_mbid: `rec-${path}`,
            title: `Title ${path}`,
            artists: [],
            score: 0.9,
            releases: []
        }
    ],
    ...over
})

const mkAlbumResponse = (album: string): IdentifyAlbumResponse => ({
    options: [
        {
            release_mbid: `rel-${album}`,
            release_group_mbid: `rg-${album}`,
            album,
            year: 1991,
            artists: [],
            track_count: 2,
            disc_count: 1,
            enriched: true,
            matched_count: 2,
            mean_score: 0.9,
            assignments: [],
            tracks: []
        }
    ],
    errors: []
})

beforeEach(() => {
    cache.clear()
})

describe('useIdentifyCache track results', () => {
    it('reports every path as missing when nothing is cached', () => {
        const { cached, missing } = cache.getTrackResults(1, ['a.mp3', 'b.mp3'])
        expect(cached).toEqual([])
        expect(missing).toEqual(['a.mp3', 'b.mp3'])
    })

    it('serves a stored result without a missing path', () => {
        cache.putTrackResults(1, [mkResult('a.mp3')])
        const { cached, missing } = cache.getTrackResults(1, ['a.mp3'])
        expect(missing).toEqual([])
        expect(cached).toHaveLength(1)
        expect(cached[0].candidates[0].recording_mbid).toBe('rec-a.mp3')
    })

    it('serves the cached subset and reports only the uncached paths as missing', () => {
        cache.putTrackResults(1, [mkResult('a.mp3')])
        const { cached, missing } = cache.getTrackResults(1, ['a.mp3', 'b.mp3'])
        expect(cached.map((r) => r.path)).toEqual(['a.mp3'])
        expect(missing).toEqual(['b.mp3'])
    })

    it('does not store a result that failed, so the path can be retried', () => {
        cache.putTrackResults(1, [mkResult('a.mp3', { candidates: [], error: 'fpcalc failed' })])
        expect(cache.getTrackResults(1, ['a.mp3']).missing).toEqual(['a.mp3'])
    })

    it('stores a result that matched nothing, so an empty answer is not looked up twice', () => {
        cache.putTrackResults(1, [mkResult('a.mp3', { candidates: [] })])
        const { cached, missing } = cache.getTrackResults(1, ['a.mp3'])
        expect(missing).toEqual([])
        expect(cached[0].candidates).toEqual([])
    })

    it('keys results by library, so the same path in another library is a miss', () => {
        cache.putTrackResults(1, [mkResult('a.mp3')])
        expect(cache.getTrackResults(2, ['a.mp3']).missing).toEqual(['a.mp3'])
    })

    it('drops the least recently used result once the cap is reached', () => {
        for (let i = 0; i < MAX_TRACK_ENTRIES; i++) {
            cache.putTrackResults(1, [mkResult(`t-${i}.mp3`)])
        }
        // Reading the oldest makes it recently used, so the next one is evicted.
        expect(cache.getTrackResults(1, ['t-0.mp3']).missing).toEqual([])
        cache.putTrackResults(1, [mkResult('overflow.mp3')])
        expect(cache.getTrackResults(1, ['t-0.mp3']).missing).toEqual([])
        expect(cache.getTrackResults(1, ['t-1.mp3']).missing).toEqual(['t-1.mp3'])
        expect(cache.getTrackResults(1, ['overflow.mp3']).missing).toEqual([])
    })
})

describe('useIdentifyCache album responses', () => {
    it('has nothing for a set it has not seen', () => {
        expect(cache.getAlbumResponse(1, ['a.mp3', 'b.mp3'])).toBeUndefined()
    })

    it('serves a stored response for the same set of files', () => {
        cache.putAlbumResponse(1, ['a.mp3', 'b.mp3'], mkAlbumResponse('Album A'))
        expect(cache.getAlbumResponse(1, ['a.mp3', 'b.mp3'])?.options[0].album).toBe('Album A')
    })

    it('ignores the order the paths are given in', () => {
        cache.putAlbumResponse(1, ['a.mp3', 'b.mp3'], mkAlbumResponse('Album A'))
        expect(cache.getAlbumResponse(1, ['b.mp3', 'a.mp3'])?.options[0].album).toBe('Album A')
    })

    it('misses for a different set of files', () => {
        cache.putAlbumResponse(1, ['a.mp3', 'b.mp3'], mkAlbumResponse('Album A'))
        expect(cache.getAlbumResponse(1, ['a.mp3', 'b.mp3', 'c.mp3'])).toBeUndefined()
    })

    it('keys responses by library', () => {
        cache.putAlbumResponse(1, ['a.mp3'], mkAlbumResponse('Album A'))
        expect(cache.getAlbumResponse(2, ['a.mp3'])).toBeUndefined()
    })

    it('drops the least recently used response once the cap is reached', () => {
        for (let i = 0; i < MAX_ALBUM_ENTRIES; i++) {
            cache.putAlbumResponse(1, [`set-${i}.mp3`], mkAlbumResponse(`Album ${i}`))
        }
        // Touch the oldest so it is no longer the least recently used.
        expect(cache.getAlbumResponse(1, ['set-0.mp3'])).toBeDefined()
        cache.putAlbumResponse(1, ['overflow.mp3'], mkAlbumResponse('Overflow'))
        expect(cache.getAlbumResponse(1, ['set-0.mp3'])).toBeDefined()
        expect(cache.getAlbumResponse(1, ['set-1.mp3'])).toBeUndefined()
        expect(cache.getAlbumResponse(1, ['overflow.mp3'])).toBeDefined()
    })
})

describe('useIdentifyCache clear', () => {
    it('empties both track results and album responses', () => {
        cache.putTrackResults(1, [mkResult('a.mp3')])
        cache.putAlbumResponse(1, ['a.mp3'], mkAlbumResponse('Album A'))
        cache.clear()
        expect(cache.getTrackResults(1, ['a.mp3']).missing).toEqual(['a.mp3'])
        expect(cache.getAlbumResponse(1, ['a.mp3'])).toBeUndefined()
    })
})

describe('mergeTrackResults', () => {
    it('orders the merged results by the requested paths', () => {
        const merged = mergeTrackResults(
            ['a.mp3', 'b.mp3', 'c.mp3'],
            [mkResult('c.mp3'), mkResult('a.mp3')],
            [mkResult('b.mp3')]
        )
        expect(merged.map((r) => r.path)).toEqual(['a.mp3', 'b.mp3', 'c.mp3'])
    })

    it('omits a requested path no result covers', () => {
        const merged = mergeTrackResults(['a.mp3', 'b.mp3'], [mkResult('a.mp3')], [])
        expect(merged.map((r) => r.path)).toEqual(['a.mp3'])
    })

    it('prefers a fresh result over a cached one for the same path', () => {
        const stale = mkResult('a.mp3')
        const fresh = mkResult('a.mp3', { candidates: [] })
        expect(mergeTrackResults(['a.mp3'], [stale], [fresh])[0]).toBe(fresh)
    })
})
