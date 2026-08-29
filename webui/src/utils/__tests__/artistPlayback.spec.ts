import { describe, it, expect } from 'vitest'
import { sortAlbumsNewestFirst, pickRandomAlbum } from '@/utils/artistPlayback'
import type { Album } from '@/types/subsonic'

describe('artistPlayback', () => {
    describe('sortAlbumsNewestFirst', () => {
        it('sorts albums by year descending', () => {
            const albums: Album[] = [
                { id: '1', name: 'First', year: 2020 },
                { id: '2', name: 'Third', year: 2022 },
                { id: '3', name: 'Second', year: 2021 }
            ]
            const sorted = sortAlbumsNewestFirst(albums)
            expect(sorted.map((a) => a.id)).toEqual(['2', '3', '1'])
        })

        it('treats missing year as 0 and sorts those last', () => {
            const albums: Album[] = [
                { id: '1', name: 'Has year', year: 2020 },
                { id: '2', name: 'No year' },
                { id: '3', name: 'Another year', year: 2021 }
            ]
            const sorted = sortAlbumsNewestFirst(albums)
            expect(sorted.map((a) => a.id)).toEqual(['3', '1', '2'])
        })

        it('uses stable name-based tiebreak for albums with the same year', () => {
            const albums: Album[] = [
                { id: '1', name: 'Zebra', year: 2020 },
                { id: '2', name: 'Alpha', year: 2020 },
                { id: '3', name: 'Beta', year: 2020 }
            ]
            const sorted = sortAlbumsNewestFirst(albums)
            expect(sorted.map((a) => a.name)).toEqual(['Alpha', 'Beta', 'Zebra'])
        })

        it('does not mutate the input array', () => {
            const albums: Album[] = [
                { id: '1', name: 'First', year: 2020 },
                { id: '2', name: 'Second', year: 2021 }
            ]
            const original = [...albums]
            sortAlbumsNewestFirst(albums)
            expect(albums).toEqual(original)
        })

        it('returns empty array for empty input', () => {
            expect(sortAlbumsNewestFirst([])).toEqual([])
        })
    })

    describe('pickRandomAlbum', () => {
        it('returns a member of the input array', () => {
            const albums: Album[] = [
                { id: '1', name: 'First' },
                { id: '2', name: 'Second' },
                { id: '3', name: 'Third' }
            ]
            const picked = pickRandomAlbum(albums)
            expect(albums).toContainEqual(picked)
        })

        it('returns the only album when given a single-element array', () => {
            const albums: Album[] = [{ id: '1', name: 'Only' }]
            expect(pickRandomAlbum(albums)).toEqual(albums[0])
        })

        it('returns null for an empty array', () => {
            expect(pickRandomAlbum([])).toBeNull()
        })
    })
})
