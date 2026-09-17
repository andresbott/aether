import { describe, it, expect } from 'vitest'
import { splitReleaseTypes, mergeReleaseTypes } from '../releaseTypes'

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
