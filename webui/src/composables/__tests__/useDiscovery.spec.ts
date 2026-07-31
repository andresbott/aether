import { describe, it, expect } from 'vitest'
import {
    flattenDiscoveryPages,
    nextDiscoveryOffset,
    DISCOVERY_PAGE_SIZE
} from '@/composables/useDiscovery'
import type { DiscoveryPage } from '@/types/subsonic'

const album = (id: string, rank: number, reason = 'genreMatch') =>
    ({ id, name: `Album ${id}`, rank, reason }) as DiscoveryPage['album'][number]

const playlist = (id: string, rank: number, reason = 'rediscover') =>
    ({
        id,
        name: `PL ${id}`,
        rank,
        reason,
        songCount: 1,
        duration: 1,
        created: '2026-01-01T00:00:00Z'
    }) as DiscoveryPage['playlist'][number]

describe('DISCOVERY_PAGE_SIZE', () => {
    it('is 48', () => {
        expect(DISCOVERY_PAGE_SIZE).toBe(48)
    })
})

describe('flattenDiscoveryPages', () => {
    it('returns an empty array for no pages', () => {
        expect(flattenDiscoveryPages([])).toEqual([])
    })

    it('interleaves albums and playlists by rank', () => {
        const page: DiscoveryPage = {
            album: [album('al-1', 0), album('al-2', 2)],
            playlist: [playlist('pl-1', 1), playlist('pl-2', 3)]
        }
        const got = flattenDiscoveryPages([page])
        expect(got.map((e) => e.rank)).toEqual([0, 1, 2, 3])
        expect(got.map((e) => e.type)).toEqual(['album', 'playlist', 'album', 'playlist'])
    })

    it('preserves each entity under its own key', () => {
        const page: DiscoveryPage = {
            album: [album('al-9', 0)],
            playlist: [playlist('pl-9', 1)]
        }
        const [first, second] = flattenDiscoveryPages([page])
        expect(first.type === 'album' && first.album.id).toBe('al-9')
        expect(second.type === 'playlist' && second.playlist.id).toBe('pl-9')
    })

    it('carries the reason onto the entry', () => {
        const page: DiscoveryPage = {
            album: [album('al-1', 0, 'favorite')],
            playlist: []
        }
        expect(flattenDiscoveryPages([page])[0].reason).toBe('favorite')
    })

    it('concatenates multiple pages in rank order', () => {
        const page1: DiscoveryPage = { album: [album('al-1', 0)], playlist: [playlist('pl-1', 1)] }
        const page2: DiscoveryPage = { album: [album('al-2', 2)], playlist: [playlist('pl-2', 3)] }
        expect(flattenDiscoveryPages([page1, page2]).map((e) => e.rank)).toEqual([0, 1, 2, 3])
    })

    // The server may return pages out of order under concurrent fetches; rank is
    // authoritative, so the sort must not rely on arrival order.
    it('sorts by rank even when pages arrive out of order', () => {
        const later: DiscoveryPage = { album: [album('al-2', 5)], playlist: [] }
        const earlier: DiscoveryPage = { album: [album('al-1', 1)], playlist: [] }
        expect(flattenDiscoveryPages([later, earlier]).map((e) => e.rank)).toEqual([1, 5])
    })

    it('handles a page with only albums', () => {
        const page: DiscoveryPage = { album: [album('al-1', 0), album('al-2', 1)], playlist: [] }
        expect(flattenDiscoveryPages([page])).toHaveLength(2)
    })

    it('handles a page with only playlists', () => {
        const page: DiscoveryPage = { album: [], playlist: [playlist('pl-1', 0)] }
        expect(flattenDiscoveryPages([page])).toHaveLength(1)
    })

    // Starring an album mid-scroll raises its score and can move it into a range an
    // earlier page already served. The server cannot prevent that statelessly, so
    // the flatten drops the later copy.
    it('dedupes an id that appears on two pages, keeping the lowest rank', () => {
        const page1: DiscoveryPage = { album: [album('al-1', 0), album('al-2', 1)], playlist: [] }
        const page2: DiscoveryPage = { album: [album('al-1', 7), album('al-3', 8)], playlist: [] }
        const got = flattenDiscoveryPages([page1, page2])
        expect(got.map((e) => (e.type === 'album' ? e.album.id : ''))).toEqual([
            'al-1',
            'al-2',
            'al-3'
        ])
        expect(got[0].rank).toBe(0)
    })

    it('dedupes albums and playlists independently', () => {
        const page: DiscoveryPage = {
            album: [album('al-1', 0), album('al-1', 2)],
            playlist: [playlist('pl-1', 1), playlist('pl-1', 3)]
        }
        expect(flattenDiscoveryPages([page])).toHaveLength(2)
    })
})

describe('nextDiscoveryOffset', () => {
    // A page holding exactly DISCOVERY_PAGE_SIZE items — the "keep going" case.
    const full = (): DiscoveryPage => ({
        album: Array.from({ length: DISCOVERY_PAGE_SIZE }, (_, i) => album(`al-${i}`, i)),
        playlist: []
    })

    it('advances by one page while pages come back full', () => {
        expect(nextDiscoveryOffset(full(), [full()])).toBe(DISCOVERY_PAGE_SIZE)
        expect(nextDiscoveryOffset(full(), [full(), full()])).toBe(2 * DISCOVERY_PAGE_SIZE)
    })

    // The terminal signal. Without it the feed pages forever on a small library.
    it('stops on a short page', () => {
        const short: DiscoveryPage = { album: [album('al-1', 0)], playlist: [] }
        expect(nextDiscoveryOffset(short, [short])).toBeUndefined()
    })

    it('stops on an empty page', () => {
        expect(nextDiscoveryOffset({ album: [], playlist: [] }, [])).toBeUndefined()
    })

    // Fullness is measured ACROSS BOTH ARRAYS, not per array: the server splits one
    // ranking into two per-type arrays, so either alone is meaningless.
    it('counts albums and playlists together when judging fullness', () => {
        const split: DiscoveryPage = {
            album: Array.from({ length: DISCOVERY_PAGE_SIZE - 1 }, (_, i) => album(`al-${i}`, i)),
            playlist: [playlist('pl-1', DISCOVERY_PAGE_SIZE - 1)]
        }
        expect(nextDiscoveryOffset(split, [split])).toBe(DISCOVERY_PAGE_SIZE)
    })

    it('stops when the two arrays together fall one short', () => {
        const nearly: DiscoveryPage = {
            album: Array.from({ length: DISCOVERY_PAGE_SIZE - 2 }, (_, i) => album(`al-${i}`, i)),
            playlist: [playlist('pl-1', DISCOVERY_PAGE_SIZE - 2)]
        }
        expect(nextDiscoveryOffset(nearly, [nearly])).toBeUndefined()
    })
})
