import { describe, it, expect } from 'vitest'
import { valueLabel, optionsFor, summarize, rowErrors, countLabel } from '@/lib/libraryFilters'
import type { LibraryFilterOptions } from '@/types/libraries'

function options(over: Partial<LibraryFilterOptions>): LibraryFilterOptions {
    return { scan_folders: [], formats: [], genres: [], release_types: [], ...over }
}

describe('countLabel', () => {
    it('pluralises everything but exactly one', () => {
        expect(countLabel(0, 'track')).toBe('0 tracks')
        expect(countLabel(1, 'track')).toBe('1 track')
        expect(countLabel(2, 'track')).toBe('2 tracks')
        expect(countLabel(1, 'album')).toBe('1 album')
        expect(countLabel(12, 'album')).toBe('12 albums')
    })
})

describe('valueLabel', () => {
    it('labels the empty release_type value as (none)', () => {
        expect(valueLabel('release_type', '')).toBe('(none)')
    })

    it('labels compilation true/false as Yes/No', () => {
        expect(valueLabel('compilation', 'true')).toBe('Yes')
        expect(valueLabel('compilation', 'false')).toBe('No')
    })

    it('leaves any other value exactly as given, including a trailing space', () => {
        expect(valueLabel('genre', 'Rock ')).toBe('Rock ')
    })
})

describe('optionsFor', () => {
    it('offers the empty "(none)" release type first, then the server list', () => {
        const result = optionsFor('release_type', [], options({ release_types: ['Album'] }))
        expect(result.map((o) => o.value)).toEqual(['', 'Album'])
        expect(result[0].label).toBe('(none)')
        expect(result.every((o) => o.missing === false)).toBe(true)
    })

    it('flags a current scan_folder value the server no longer offers as missing', () => {
        const result = optionsFor('scan_folder', ['Gone', 'Music'], options({ scan_folders: ['Music'] }))
        const music = result.find((o) => o.value === 'Music')
        const gone = result.find((o) => o.value === 'Gone')
        expect(music?.missing).toBe(false)
        expect(gone?.missing).toBe(true)
        expect(gone?.label).toContain('not configured')
    })

    it('treats a trailing-space genre as a different value from its trimmed form', () => {
        const result = optionsFor('genre', ['Rock '], options({ genres: ['Rock'] }))
        expect(result.map((o) => o.value)).toEqual(['Rock', 'Rock '])
        const exact = result.find((o) => o.value === 'Rock')
        const padded = result.find((o) => o.value === 'Rock ')
        expect(exact?.missing).toBe(false)
        expect(padded?.missing).toBe(true)
        expect(padded?.label).toContain('not in the catalog')
    })

    it('returns no options when they have not loaded yet, without throwing', () => {
        expect(optionsFor('genre', [], undefined)).toEqual([])
    })

    // "Not loaded" is not "not offered": before the server's lists arrive (or
    // if they never do) nothing is known about what a field offers, so a
    // stored value must not be labelled gone — an admin who believed that
    // label would remove a perfectly valid value.
    it('shows a stored value plainly while the options have not loaded, never as missing', () => {
        expect(optionsFor('scan_folder', ['Music'], undefined)).toEqual([
            { label: 'Music', value: 'Music', missing: false }
        ])
    })

    it('does not invent the "(none)" release type before the options load', () => {
        expect(optionsFor('release_type', [''], undefined)).toEqual([
            { label: '(none)', value: '', missing: false }
        ])
    })
})

describe('summarize', () => {
    it('lists up to three values and folds the rest into a +N suffix', () => {
        const lines = summarize([{ field: 'genre', values: ['Rock', 'Jazz', 'Pop', 'Blues'] }])
        expect(lines).toEqual(['Genre: Rock, Jazz, Pop +1'])
    })

    it('labels a compilation filter with Yes/No', () => {
        expect(summarize([{ field: 'compilation', values: ['false'] }])).toEqual(['Compilation: No'])
    })

    it('labels an empty release_type value as (none)', () => {
        expect(summarize([{ field: 'release_type', values: [''] }])).toEqual(['Release type: (none)'])
    })
})

describe('rowErrors', () => {
    it('splits filter-row pointers into per-row errors keyed by row and value index, apart from general ones', () => {
        const { rows, general } = rowErrors({
            '/filters/1/values/0': 'a',
            '/filters/1/field': 'b',
            '/filters/0/values': 'c',
            '/filters': 'd',
            '/name': 'e'
        })
        expect(rows[1]).toEqual({ field: 'b', value: { 0: 'a' } })
        expect(rows[0]).toEqual({ value: {}, values: 'c' })
        expect(general).toEqual(['d'])
        // '/name' names neither a row nor the general filters list.
        expect(Object.keys(rows).sort()).toEqual(['0', '1'])
    })
})
