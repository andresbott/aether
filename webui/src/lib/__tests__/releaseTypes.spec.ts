import { describe, it, expect } from 'vitest'
import { formatReleaseTypes, splitReleaseTypes, mergeReleaseTypes } from '@/lib/releaseTypes'

describe('splitReleaseTypes', () => {
    it('classifies the first primary value and the rest as secondary, canonicalizing case', () => {
        expect(splitReleaseTypes(['album', 'compilation'])).toEqual({
            primary: 'Album',
            secondary: ['Compilation']
        })
    })

    it('keeps an unrecognized value as a secondary so it round-trips', () => {
        expect(splitReleaseTypes(['Album', 'Weird'])).toEqual({
            primary: 'Album',
            secondary: ['Weird']
        })
    })

    it('returns no primary and no secondary for an empty list', () => {
        expect(splitReleaseTypes([])).toEqual({ primary: '', secondary: [] })
    })

    it('treats a second primary-vocabulary value as a secondary', () => {
        expect(splitReleaseTypes(['Album', 'Single'])).toEqual({
            primary: 'Album',
            secondary: ['Single']
        })
    })
})

describe('mergeReleaseTypes', () => {
    it('flattens primary-first and drops blanks', () => {
        expect(mergeReleaseTypes('Album', ['Compilation'])).toEqual(['Album', 'Compilation'])
        expect(mergeReleaseTypes('', ['Live'])).toEqual(['Live'])
        expect(mergeReleaseTypes('EP', [])).toEqual(['EP'])
    })
})

describe('formatReleaseTypes', () => {
    it('joins the types primary-first in vocabulary casing', () => {
        expect(formatReleaseTypes(['soundtrack', 'album', 'live'])).toBe('Album · Soundtrack · Live')
        expect(formatReleaseTypes(['EP'])).toBe('EP')
    })

    it('shows secondary-only and unrecognized types as they are', () => {
        expect(formatReleaseTypes(['compilation'])).toBe('Compilation')
        expect(formatReleaseTypes(['Album', 'Weird'])).toBe('Album · Weird')
    })

    it('returns an empty string when no type is set', () => {
        expect(formatReleaseTypes(undefined)).toBe('')
        expect(formatReleaseTypes([])).toBe('')
        expect(formatReleaseTypes([' '])).toBe('')
    })
})
