import { describe, it, expect } from 'vitest'
import { deriveLetterIndex, firstLetter } from '@/utils/letterIndex'

describe('firstLetter', () => {
    it('uppercases the first latin letter', () => {
        expect(firstLetter('abbey road')).toBe('A')
        expect(firstLetter('Kid A')).toBe('K')
    })

    it('folds diacritics like the server does on name_norm', () => {
        expect(firstLetter('Björk')).toBe('B')
        expect(firstLetter('Église')).toBe('E')
    })

    it('buckets non-alphabetic starts under #, skipping leading symbols', () => {
        expect(firstLetter('1999')).toBe('#')
        expect(firstLetter('!!!')).toBe('#')
        // A leading symbol does not hide a following letter — matches Go's firstLetter.
        expect(firstLetter('(Untitled)')).toBe('U')
    })

    it('returns # for an empty name', () => {
        expect(firstLetter('')).toBe('#')
    })
})

describe('deriveLetterIndex', () => {
    it('returns offsets and counts over an already-sorted list', () => {
        expect(deriveLetterIndex(['Abbey Road', 'Amnesiac', 'Bends', 'Kid A'])).toEqual([
            { name: 'A', offset: 0, count: 2 },
            { name: 'B', offset: 2, count: 1 },
            { name: 'K', offset: 3, count: 1 }
        ])
    })

    it('groups numeric and symbol names under one # bucket', () => {
        expect(deriveLetterIndex(['1999', '2112', 'Aja'])).toEqual([
            { name: '#', offset: 0, count: 2 },
            { name: 'A', offset: 2, count: 1 }
        ])
    })

    it('is empty for an empty list', () => {
        expect(deriveLetterIndex([])).toEqual([])
    })

    it('merges a letter that reappears into its first bucket, keeping that offset', () => {
        // Not expected from a sorted server response, but the rail must still get
        // one entry per letter pointing at its first item.
        expect(deriveLetterIndex(['Aja', 'Bends', 'Amnesiac'])).toEqual([
            { name: 'A', offset: 0, count: 2 },
            { name: 'B', offset: 1, count: 1 }
        ])
    })
})
