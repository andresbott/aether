import { describe, it, expect } from 'vitest'
import { searchIcons } from '@/lib/iconSearch'
import { SUGGESTED_LIBRARY_ICONS } from '@/lib/libraryIcons'

// Sorted, as the real catalogue is.
const names = [
    '10k', 'album', 'audiotrack', 'folder', 'library_music', 'music_note', 'music_off',
    'nightlife', 'piano', 'queue_music', 'radio', 'theaters'
]

describe('searchIcons', () => {
    it('shows the curated suggestions for an empty query, not the alphabet', () => {
        expect(searchIcons(names, '')).toEqual(SUGGESTED_LIBRARY_ICONS.filter((n) => names.includes(n)))
        expect(searchIcons(names, '   ')).not.toContain('10k')
    })
    it('ranks exact, then prefix, then word start, then substring', () => {
        expect(searchIcons(['a_music', 'music', 'music_note', 'xmusicx'], 'music')).toEqual([
            'music', 'music_note', 'a_music', 'xmusicx'
        ])
    })
    it('keeps alphabetical order within a tier', () => {
        expect(searchIcons(names, 'music')).toEqual(['music_note', 'music_off', 'library_music', 'queue_music'])
    })
    it('matches names typed with spaces, dashes or capitals', () => {
        expect(searchIcons(names, 'queue music')[0]).toBe('queue_music')
        expect(searchIcons(names, 'Queue-Music')[0]).toBe('queue_music')
    })
    it('caps the result count', () => {
        expect(searchIcons(names, 'a', 2)).toHaveLength(2)
    })
    it('returns nothing for gibberish', () => {
        expect(searchIcons(names, 'zzqx')).toEqual([])
    })
})
