import { describe, it, expect } from 'vitest'
import { nextSongOffset, SONG_LIST_PAGE_SIZE } from '@/composables/useSongList'
import type { SearchResult3 } from '@/types/subsonic'

const makePage = (songCount: number): SearchResult3 => ({
    song: Array.from({ length: songCount }, (_, i) => ({
        id: `tr-${i}`,
        title: `Song ${i}`
    })) as SearchResult3['song']
})

describe('SONG_LIST_PAGE_SIZE', () => {
    it('is 50', () => {
        expect(SONG_LIST_PAGE_SIZE).toBe(50)
    })
})

describe('nextSongOffset', () => {
    // A page holding exactly SONG_LIST_PAGE_SIZE songs — the "keep going" case.
    const full = (): SearchResult3 => makePage(SONG_LIST_PAGE_SIZE)

    it('advances by one page while pages come back full', () => {
        expect(nextSongOffset(full(), [full()])).toBe(SONG_LIST_PAGE_SIZE)
        expect(nextSongOffset(full(), [full(), full()])).toBe(2 * SONG_LIST_PAGE_SIZE)
    })

    // The terminal signal. Without it the list pages forever on a small library.
    it('stops on a short page', () => {
        const short: SearchResult3 = makePage(1)
        expect(nextSongOffset(short, [short])).toBeUndefined()
    })

    it('stops on an empty page', () => {
        expect(nextSongOffset({ song: [] }, [])).toBeUndefined()
    })

    it('stops when a page is missing the song array entirely', () => {
        const empty: SearchResult3 = {}
        expect(nextSongOffset(empty, [empty])).toBeUndefined()
    })

    it('stops when the page falls one short', () => {
        const nearly: SearchResult3 = makePage(SONG_LIST_PAGE_SIZE - 1)
        expect(nextSongOffset(nearly, [nearly])).toBeUndefined()
    })

    it('advances on exactly a full page at any depth', () => {
        expect(nextSongOffset(full(), Array(5).fill(full()))).toBe(5 * SONG_LIST_PAGE_SIZE)
    })
})
