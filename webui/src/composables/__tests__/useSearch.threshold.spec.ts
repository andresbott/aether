import { describe, it, expect } from 'vitest'
import {
    MIN_SEARCH_LENGTH,
    queryKeys,
    searchTermIsLongEnough
} from '@/composables/useSubsonicQueries'

// The threshold guards `enabled` on the query, so it is what actually stops a
// request from reaching the server. SearchView only mirrors it for messaging.
describe('search term threshold', () => {
    it('rejects terms shorter than the minimum', () => {
        expect(searchTermIsLongEnough('')).toBe(false)
        expect(searchTermIsLongEnough('r')).toBe(false)
        expect(searchTermIsLongEnough('ro')).toBe(false)
    })

    it('accepts the minimum length and above', () => {
        expect(searchTermIsLongEnough('roc')).toBe(true)
        expect(searchTermIsLongEnough('rock')).toBe(true)
    })

    it('ignores surrounding whitespace, so blanks never pad a short term', () => {
        expect(searchTermIsLongEnough('   ')).toBe(false)
        expect(searchTermIsLongEnough(' ro ')).toBe(false)
        expect(searchTermIsLongEnough('  roc  ')).toBe(true)
    })

    it('counts a multi-byte term by characters, not bytes', () => {
        // "Éxt" is 3 characters but 4 bytes in UTF-8; a byte-based check would
        // have let a 2-character accented term through.
        expect(searchTermIsLongEnough('Éx')).toBe(false)
        expect(searchTermIsLongEnough('Éxt')).toBe(true)
    })

    it('exposes the threshold so the UI copy cannot drift from the guard', () => {
        expect(MIN_SEARCH_LENGTH).toBe(3)
    })
})

// Regression: keying on the term alone let a scoped ("songs only") response
// satisfy a later unscoped request from cache, so switching back to All showed
// only songs and silently hid every other type — with no request to reveal it.
describe('search cache key', () => {
    const all = { artistCount: 24, albumCount: 24, songCount: 50, genreCount: 24 }
    const songsOnly = { artistCount: 0, albumCount: 0, songCount: 100, genreCount: 0 }

    it('separates entries for the same term under different scopes', () => {
        expect(queryKeys.search('rock', all)).not.toEqual(queryKeys.search('rock', songsOnly))
    })

    it('reuses one entry for the same term and the same scope', () => {
        expect(queryKeys.search('rock', all)).toEqual(queryKeys.search('rock', { ...all }))
    })

    it('still separates entries by term within one scope', () => {
        expect(queryKeys.search('rock', all)).not.toEqual(queryKeys.search('jazz', all))
    })

    it('keeps searchAll a prefix of every entry, so invalidation still sweeps them', () => {
        const key = queryKeys.search('rock', songsOnly)
        expect(key.slice(0, queryKeys.searchAll.length)).toEqual([...queryKeys.searchAll])
    })
})
