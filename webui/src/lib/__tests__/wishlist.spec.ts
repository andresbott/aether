import { describe, it, expect } from 'vitest'
import {
    isWishlistKind,
    musicBrainzUrl,
    wishlistDetails,
    wishlistLibraryId,
    wishlistLibraryRoute,
    wishlistSubtitle,
    yearOf
} from '@/lib/wishlist'
import type { WishlistSnapshot } from '@/types/wishlist'

const snap = (over: Partial<WishlistSnapshot>): WishlistSnapshot => ({
    kind: 'album',
    mbid: 'm',
    title: 'Nevermind',
    artist: 'Nirvana',
    album: '',
    date: '1991-09-24',
    type: '',
    disambiguation: '',
    ...over
})

describe('wishlistLibraryId / wishlistLibraryRoute', () => {
    it('addresses artists and albums by their Subsonic ids', () => {
        expect(wishlistLibraryId({ kind: 'artist', in_library: true, match_id: 4 })).toBe('ar-4')
        expect(wishlistLibraryRoute({ kind: 'album', in_library: true, match_id: 7 })).toEqual({
            name: 'album',
            params: { id: 'al-7' }
        })
        expect(wishlistLibraryRoute({ kind: 'artist', in_library: true, match_id: 4 })).toEqual({
            name: 'artist',
            params: { id: 'ar-4' }
        })
    })

    it('sends a found song to the album it is on', () => {
        const m = { kind: 'song' as const, in_library: true, match_id: 99, match_album_id: 7 }
        expect(wishlistLibraryId(m)).toBe('al-7')
        expect(wishlistLibraryRoute(m)).toEqual({ name: 'album', params: { id: 'al-7' } })
    })

    it('has no link while not found', () => {
        expect(wishlistLibraryRoute({ kind: 'album', in_library: false })).toBeNull()
        expect(wishlistLibraryId({ kind: 'song', in_library: true, match_id: 3 })).toBeNull()
    })
})

describe('wishlist text helpers', () => {
    it('names the artist, plus the album for a song', () => {
        expect(wishlistSubtitle(snap({}))).toBe('Nirvana')
        expect(wishlistSubtitle(snap({ kind: 'song', album: 'Nevermind', title: 'Lithium' }))).toBe('Nirvana · Nevermind')
        expect(wishlistSubtitle(snap({ kind: 'artist', artist: '' }))).toBe('')
    })

    it('lists type (or kind), year and disambiguation, dropping blanks', () => {
        expect(wishlistDetails(snap({ type: 'Album · Live', disambiguation: 'remaster' }))).toEqual([
            'Album',
            'Live',
            '1991',
            'remaster'
        ])
        expect(wishlistDetails(snap({ kind: 'song', date: '' }))).toEqual(['Song'])
    })

    it('reads the year off a MusicBrainz date', () => {
        expect(yearOf('1991-09-24')).toBe('1991')
        expect(yearOf('1987')).toBe('1987')
        expect(yearOf('')).toBe('')
    })

    it('marks an entry added by hand, which has no MusicBrainz page', () => {
        expect(wishlistDetails(snap({ mbid: '', type: '', date: '' }))).toEqual(['Album', 'Added by hand'])
        expect(musicBrainzUrl('album', '')).toBeNull()
    })

    it('links each kind to its MusicBrainz entity', () => {
        expect(musicBrainzUrl('album', 'abc')).toBe('https://musicbrainz.org/release-group/abc')
        expect(musicBrainzUrl('song', 'abc')).toBe('https://musicbrainz.org/recording/abc')
        expect(musicBrainzUrl('artist', 'abc')).toBe('https://musicbrainz.org/artist/abc')
    })

    it('recognizes the three kinds only', () => {
        expect(isWishlistKind('song')).toBe(true)
        expect(isWishlistKind('playlist')).toBe(false)
        expect(isWishlistKind(undefined)).toBe(false)
    })
})
