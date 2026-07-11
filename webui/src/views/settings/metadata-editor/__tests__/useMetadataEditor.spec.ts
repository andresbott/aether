import { describe, it, expect } from 'vitest'
import { diffInitialValues, distinctArtistMbids } from '@/composables/useMetadataEditor'
import type { Track } from '@/types/metadata'

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'a.mp3',
    name: 'a.mp3',
    title: '',
    artists: [],
    album_artists: [],
    album: '',
    year: 0,
    compilation: false,
    mb_artist_ids: [],
    mb_album_artist_ids: [],
    ...over
})

describe('diffInitialValues', () => {
    it('prefills when all selected tracks share the same value', () => {
        const a = mkTrack({ title: 'Hello', album: 'X', artists: ['One'] })
        const b = mkTrack({ path: 'b.mp3', title: 'Hello', album: 'X', artists: ['One'] })
        const v = diffInitialValues([a, b])
        expect(v.title).toEqual({ shared: true, value: 'Hello' })
        expect(v.album).toEqual({ shared: true, value: 'X' })
        expect(v.artists).toEqual({ shared: true, value: ['One'] })
    })

    it('marks fields as mixed when values differ', () => {
        const a = mkTrack({ title: 'Hello', album: 'X' })
        const b = mkTrack({ path: 'b.mp3', title: 'World', album: 'X' })
        const v = diffInitialValues([a, b])
        expect(v.title.shared).toBe(false)
        expect(v.album).toEqual({ shared: true, value: 'X' })
    })

    it('compares artist arrays by value, not reference', () => {
        const a = mkTrack({ artists: ['A', 'B'] })
        const b = mkTrack({ path: 'b.mp3', artists: ['A', 'B'] })
        const c = mkTrack({ path: 'c.mp3', artists: ['A', 'C'] })
        expect(diffInitialValues([a, b]).artists.shared).toBe(true)
        expect(diffInitialValues([a, c]).artists.shared).toBe(false)
    })
})

describe('distinctArtistMbids', () => {
    it('returns one row per distinct name with its shared MBID, in first-seen order', () => {
        const a = mkTrack({ artists: ['Daft Punk', 'Pharrell'], mb_artist_ids: ['id-dp', 'id-ph'] })
        const b = mkTrack({ path: 'b.mp3', artists: ['Daft Punk'], mb_artist_ids: ['id-dp'] })
        const rows = distinctArtistMbids([a, b], 'artists', 'mb_artist_ids')
        expect(rows).toEqual([
            { name: 'Daft Punk', mbid: 'id-dp', mixed: false },
            { name: 'Pharrell', mbid: 'id-ph', mixed: false }
        ])
    })

    it('marks a name as mixed when tracks disagree on its MBID', () => {
        const a = mkTrack({ artists: ['X'], mb_artist_ids: ['id-1'] })
        const b = mkTrack({ path: 'b.mp3', artists: ['X'], mb_artist_ids: ['id-2'] })
        const rows = distinctArtistMbids([a, b], 'artists', 'mb_artist_ids')
        expect(rows).toEqual([{ name: 'X', mbid: '', mixed: true }])
    })

    it('treats a missing id slot as empty string', () => {
        const a = mkTrack({ artists: ['X', 'Y'], mb_artist_ids: ['id-x'] })
        const rows = distinctArtistMbids([a], 'artists', 'mb_artist_ids')
        expect(rows).toEqual([
            { name: 'X', mbid: 'id-x', mixed: false },
            { name: 'Y', mbid: '', mixed: false }
        ])
    })
})

