import { describe, it, expect } from 'vitest'
import type { Album } from '@/types/subsonic'
import { groupAlbumsByReleaseType } from '@/lib/releaseGrouping'

const album = (id: string, releaseTypes?: string[], year?: number): Album => ({
    id,
    name: id,
    releaseTypes,
    year
})

const labels = (albums: Album[]) => groupAlbumsByReleaseType(albums).map((g) => g.label)

describe('groupAlbumsByReleaseType', () => {
    it('classifies an album by its primary type, ignoring secondary types and case', () => {
        const groups = groupAlbumsByReleaseType([
            album('a', ['album', 'Compilation']),
            album('e', ['EP']),
            // four singles so the group is kept rather than folded
            album('s1', ['Single']),
            album('s2', ['Single']),
            album('s3', ['Single']),
            album('s4', ['Single'])
        ])
        expect(groups.map((g) => g.type)).toEqual(['Album', 'EP', 'Single'])
    })

    it('treats an untyped or secondary-only release as an Album', () => {
        expect(labels([album('u1'), album('u2', []), album('u3', ['Compilation'])])).toEqual([
            'Albums'
        ])
    })

    it('folds a non-Album/EP type with fewer than four releases into Other', () => {
        const groups = groupAlbumsByReleaseType([
            album('a', ['Album']),
            album('s1', ['Single']),
            album('s2', ['Single']),
            album('s3', ['Single'])
        ])
        expect(groups.map((g) => g.label)).toEqual(['Albums', 'Other'])
        const other = groups.find((g) => g.type === 'Other')!
        expect(other.albums.map((al) => al.id)).toEqual(['s1', 's2', 's3'])
    })

    it('keeps a non-Album/EP type with four or more as its own group', () => {
        const groups = groupAlbumsByReleaseType([
            album('a', ['Album']),
            album('s1', ['Single']),
            album('s2', ['Single']),
            album('s3', ['Single']),
            album('s4', ['Single'])
        ])
        expect(groups.map((g) => g.label)).toEqual(['Albums', 'Singles'])
    })

    it('always keeps Album and EP as their own groups, even below four', () => {
        expect(labels([album('a', ['Album']), album('e', ['EP'])])).toEqual(['Albums', 'EPs'])
    })

    it('orders groups Albums, EPs, Singles, Broadcast, Other', () => {
        const four = (type: string, prefix: string) =>
            Array.from({ length: 4 }, (_, i) => album(`${prefix}${i}`, [type]))
        const groups = groupAlbumsByReleaseType([
            ...four('Broadcast', 'b'),
            album('o', ['Other']),
            ...four('Single', 's'),
            album('e', ['EP']),
            album('a', ['Album'])
        ])
        expect(groups.map((g) => g.label)).toEqual([
            'Albums',
            'EPs',
            'Singles',
            'Broadcast',
            'Other'
        ])
    })

    it('sorts albums within a group by year descending', () => {
        const groups = groupAlbumsByReleaseType([
            album('old', ['Album'], 2000),
            album('new', ['Album'], 2020),
            album('mid', ['Album'], 2010)
        ])
        expect(groups[0].albums.map((al) => al.id)).toEqual(['new', 'mid', 'old'])
    })

    it('returns a single group for a single-type discography', () => {
        expect(labels([album('a1', ['Album']), album('a2', ['Album'])])).toEqual(['Albums'])
    })

    it('returns no groups for an empty discography', () => {
        expect(groupAlbumsByReleaseType([])).toEqual([])
    })
})
