import { describe, it, expect } from 'vitest'
import {
    DISCOVERY_SECTIONS,
    findSection,
    sortPlaylistsForSection
} from '@/composables/useDiscovery'
import type { Playlist } from '@/types/subsonic'

const pl = (over: Partial<Playlist> & { id: string; name: string }): Playlist => ({
    songCount: 1,
    duration: 60,
    created: '2026-01-01T00:00:00Z',
    ...over
})

describe('DISCOVERY_SECTIONS', () => {
    it('lists the five sections in display order', () => {
        expect(DISCOVERY_SECTIONS.map((s) => s.key)).toEqual([
            'recently-added',
            'favorites',
            'most-played',
            'recently-played',
            'random'
        ])
    })

    it('maps each section to an album list type', () => {
        const types = Object.fromEntries(DISCOVERY_SECTIONS.map((s) => [s.key, s.albumListType]))
        expect(types).toEqual({
            'recently-added': 'newest',
            favorites: 'starred',
            'most-played': 'frequent',
            'recently-played': 'recent',
            random: 'random'
        })
    })

    it('resolves a known key and rejects an unknown one', () => {
        expect(findSection('favorites')?.title).toBe('Favorites')
        expect(findSection('nope')).toBeUndefined()
    })
})

describe('sortPlaylistsForSection', () => {
    it('sorts recently-added by created, newest first', () => {
        const input = [
            pl({ id: 'a', name: 'A', created: '2026-01-01T00:00:00Z' }),
            pl({ id: 'b', name: 'B', created: '2026-06-01T00:00:00Z' })
        ]
        expect(sortPlaylistsForSection(input, 'recently-added').map((p) => p.id)).toEqual(['b', 'a'])
    })

    it('keeps only starred playlists for favorites, newest star first', () => {
        const input = [
            pl({ id: 'a', name: 'A' }),
            pl({ id: 'b', name: 'B', starred: '2026-02-01T00:00:00Z' }),
            pl({ id: 'c', name: 'C', starred: '2026-05-01T00:00:00Z' })
        ]
        expect(sortPlaylistsForSection(input, 'favorites').map((p) => p.id)).toEqual(['c', 'b'])
    })

    it('sorts most-played by playCount and drops never-played', () => {
        const input = [
            pl({ id: 'a', name: 'A', playCount: 2 }),
            pl({ id: 'b', name: 'B' }),
            pl({ id: 'c', name: 'C', playCount: 9 })
        ]
        expect(sortPlaylistsForSection(input, 'most-played').map((p) => p.id)).toEqual(['c', 'a'])
    })

    it('sorts recently-played by played and drops never-played', () => {
        const input = [
            pl({ id: 'a', name: 'A', played: '2026-03-01T00:00:00Z' }),
            pl({ id: 'b', name: 'B' }),
            pl({ id: 'c', name: 'C', played: '2026-07-01T00:00:00Z' })
        ]
        expect(sortPlaylistsForSection(input, 'recently-played').map((p) => p.id)).toEqual(['c', 'a'])
    })

    it('returns every playlist for random without mutating the input', () => {
        const input = [pl({ id: 'a', name: 'A' }), pl({ id: 'b', name: 'B' })]
        const snapshot = input.map((p) => p.id)
        const out = sortPlaylistsForSection(input, 'random')
        expect(out).toHaveLength(2)
        expect(input.map((p) => p.id)).toEqual(snapshot)
    })

    it('returns an empty array for an unknown section', () => {
        expect(sortPlaylistsForSection([pl({ id: 'a', name: 'A' })], 'nope')).toEqual([])
    })
})
