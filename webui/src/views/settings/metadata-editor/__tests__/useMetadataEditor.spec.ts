import { describe, it, expect } from 'vitest'
import {
    diffInitialValues,
    distinctArtistMbids,
    mergeUpdateResults,
    partitionFields
} from '@/composables/useMetadataEditor'
import type { Track } from '@/types/metadata'

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'a.mp3',
    name: 'a.mp3',
    title: '',
    artists: [],
    album_artists: [],
    album: '',
    genres: [],
    year: 0,
    track_number: 0,
    disc_number: 0,
    disc_subtitle: '',
    compilation: false,
    mb_artist_ids: [],
    mb_album_artist_ids: [],
    mb_recording_id: '',
    mb_release_id: '',
    mb_release_group_id: '',
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

    it('prefills disc number and subtitle when shared, marks them mixed when they differ', () => {
        const a = mkTrack({ disc_number: 2, disc_subtitle: 'CD 2' })
        const b = mkTrack({ path: 'b.mp3', disc_number: 2, disc_subtitle: 'CD 2' })
        const shared = diffInitialValues([a, b])
        expect(shared.disc_number).toEqual({ shared: true, value: 2 })
        expect(shared.disc_subtitle).toEqual({ shared: true, value: 'CD 2' })

        const c = mkTrack({ path: 'c.mp3', disc_number: 1, disc_subtitle: 'CD 1' })
        const mixed = diffInitialValues([a, c])
        expect(mixed.disc_number).toEqual({ shared: false, value: 0 })
        expect(mixed.disc_subtitle).toEqual({ shared: false, value: '' })
    })

    it('prefills album MusicBrainz IDs when shared, marks them mixed when they differ', () => {
        const a = mkTrack({ mb_release_id: 'rel-1', mb_release_group_id: 'rg-1' })
        const b = mkTrack({ path: 'b.mp3', mb_release_id: 'rel-1', mb_release_group_id: 'rg-1' })
        const shared = diffInitialValues([a, b])
        expect(shared.mb_release_id).toEqual({ shared: true, value: 'rel-1' })
        expect(shared.mb_release_group_id).toEqual({ shared: true, value: 'rg-1' })

        const c = mkTrack({ path: 'c.mp3', mb_release_id: 'rel-2', mb_release_group_id: 'rg-1' })
        const mixed = diffInitialValues([a, c])
        expect(mixed.mb_release_id).toEqual({ shared: false, value: '' })
        expect(mixed.mb_release_group_id).toEqual({ shared: true, value: 'rg-1' })
    })

    it('prefills genres and track number when shared, marks them mixed when they differ', () => {
        const a = mkTrack({ genres: ['Rock', 'Jazz'], track_number: 3 })
        const b = mkTrack({ path: 'b.mp3', genres: ['Rock', 'Jazz'], track_number: 3 })
        const shared = diffInitialValues([a, b])
        expect(shared.genres).toEqual({ shared: true, value: ['Rock', 'Jazz'] })
        expect(shared.track_number).toEqual({ shared: true, value: 3 })

        const c = mkTrack({ path: 'c.mp3', genres: ['Pop'], track_number: 7 })
        const mixed = diffInitialValues([a, c])
        expect(mixed.genres).toEqual({ shared: false, value: [] })
        expect(mixed.track_number).toEqual({ shared: false, value: 0 })
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

describe('partitionFields', () => {
    it('returns null when a names field and its MBID map are not both present', () => {
        expect(partitionFields({ artists: ['A'] })).toBeNull()
        expect(partitionFields({ artist_mbids: { A: 'x' } })).toBeNull()
        // cross pairs (artist names + album-artist MBIDs) do not conflict
        expect(partitionFields({ artists: ['A'], album_artist_mbids: { B: 'y' } })).toBeNull()
    })

    it('splits names from MBIDs when a conflicting pair is present', () => {
        expect(
            partitionFields({ album: 'Al', artists: ['A'], artist_mbids: { A: 'x' } })
        ).toEqual({
            names: { album: 'Al', artists: ['A'] },
            mbids: { artist_mbids: { A: 'x' } }
        })
    })

    it('splits the album-artist pair too, carrying both MBID maps', () => {
        const parts = partitionFields({
            album_artists: ['B'],
            artist_mbids: { A: 'x' },
            album_artist_mbids: { B: 'y' }
        })
        expect(parts).toEqual({
            names: { album_artists: ['B'] },
            mbids: { artist_mbids: { A: 'x' }, album_artist_mbids: { B: 'y' } }
        })
    })
})

describe('mergeUpdateResults', () => {
    it('marks a path failed if either step failed and concatenates errors', () => {
        const first = [
            { path: 'a', ok: true },
            { path: 'b', ok: false, error: 'e1' }
        ]
        const second = [
            { path: 'a', ok: false, error: 'e2' },
            { path: 'b', ok: true }
        ]
        expect(mergeUpdateResults(first, second)).toEqual([
            { path: 'a', ok: false, error: 'e2' },
            { path: 'b', ok: false, error: 'e1' }
        ])
    })

    it('keeps a path ok only when both steps succeed', () => {
        expect(
            mergeUpdateResults([{ path: 'a', ok: true }], [{ path: 'a', ok: true }])
        ).toEqual([{ path: 'a', ok: true, error: undefined }])
    })
})

