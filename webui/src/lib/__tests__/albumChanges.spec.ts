import { describe, it, expect } from 'vitest'
import { albumChangeCounts, summarizeAlbumChanges } from '@/lib/albumChanges'
import type { AlbumOption, AlbumAssignment, Track } from '@/types/metadata'

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'a.mp3',
    name: 'a.mp3',
    title: 'Current Title',
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

const mkAssignment = (over: Partial<AlbumAssignment> = {}): AlbumAssignment => ({
    path: 'a.mp3',
    source: 'fingerprint',
    title: '',
    recording_mbid: '',
    artists: [],
    disc_number: 0,
    track_number: 0,
    score: 0,
    ...over
})

const mkOption = (over: Partial<AlbumOption> = {}): AlbumOption => ({
    release_mbid: 'rel',
    release_group_mbid: 'rg',
    album: 'Album A',
    year: 1991,
    artists: [{ name: 'Artist', mbid: 'art-1' }],
    track_count: 3,
    disc_count: 1,
    enriched: true,
    matched_count: 2,
    mean_score: 0.9,
    assignments: [],
    tracks: [],
    ...over
})

describe('albumChangeCounts', () => {
    it('counts every field that would be rewritten across the selection', () => {
        // Two bare files (no title/artist/album/year on disk): the option rewrites
        // all four fields on both rows.
        const option = mkOption({
            assignments: [
                mkAssignment({ path: '01.mp3', title: 'One', artists: [{ name: 'Artist', mbid: 'art-1' }] }),
                mkAssignment({ path: '02.mp3', source: 'inferred', title: 'Two' })
            ]
        })
        const tracks = [mkTrack({ path: '01.mp3' }), mkTrack({ path: '02.mp3' })]
        expect(albumChangeCounts(option, tracks)).toEqual({
            titles: 2,
            artists: 2,
            albums: 2,
            years: 2
        })
    })

    it('does not count a field a file already carries', () => {
        // The file is already fully tagged to match the option, so nothing changes.
        const option = mkOption({
            assignments: [mkAssignment({ path: '01.mp3', title: 'One', artists: [{ name: 'Artist', mbid: 'art-1' }] })]
        })
        const tracks = [mkTrack({ path: '01.mp3', title: 'One', artists: ['Artist'], album: 'Album A', year: 1991 })]
        expect(albumChangeCounts(option, tracks)).toEqual({
            titles: 0,
            artists: 0,
            albums: 0,
            years: 0
        })
    })

    it('counts album and year per row for a mixed selection', () => {
        // 01.mp3 already carries the release album+year; 02.mp3 carries neither.
        const option = mkOption({
            assignments: [
                mkAssignment({ path: '01.mp3', title: 'One' }),
                mkAssignment({ path: '02.mp3', title: 'Two' })
            ]
        })
        const tracks = [
            mkTrack({ path: '01.mp3', album: 'Album A', year: 1991 }),
            mkTrack({ path: '02.mp3' })
        ]
        const counts = albumChangeCounts(option, tracks)
        expect(counts.albums).toBe(1)
        expect(counts.years).toBe(1)
    })

    it('stages no title for a row with no position but still stages the album fields', () => {
        // A source:'none' assignment proposes no title, so the title is left alone,
        // but the release-level album and year still apply.
        const option = mkOption({
            assignments: [
                mkAssignment({ path: '01.mp3', title: 'One' }),
                mkAssignment({ path: '02.mp3', source: 'none', title: '' })
            ]
        })
        const tracks = [mkTrack({ path: '01.mp3' }), mkTrack({ path: '02.mp3' })]
        const counts = albumChangeCounts(option, tracks)
        expect(counts.titles).toBe(1)
        expect(counts.albums).toBe(2)
        expect(counts.years).toBe(2)
    })

    it('falls back to the release artist when an assignment carries no credits', () => {
        // The assignment has no artists of its own, so the album artist is what a
        // save would write — and it differs from the file's empty artist.
        const option = mkOption({
            assignments: [mkAssignment({ path: '01.mp3', title: 'One', artists: [] })]
        })
        const tracks = [mkTrack({ path: '01.mp3' })]
        expect(albumChangeCounts(option, tracks).artists).toBe(1)
    })

    it('counts no year change when the release has no year', () => {
        const option = mkOption({
            year: 0,
            assignments: [mkAssignment({ path: '01.mp3', title: 'One' })]
        })
        const tracks = [mkTrack({ path: '01.mp3', year: 2000 })]
        expect(albumChangeCounts(option, tracks).years).toBe(0)
    })

    it('tolerates null assignment and artist lists from the server', () => {
        // A Go nil slice marshals to JSON null; the helper must not throw on it.
        const option = {
            ...mkOption(),
            artists: null,
            assignments: null
        } as unknown as AlbumOption
        const tracks = [mkTrack({ path: '01.mp3' })]
        expect(() => albumChangeCounts(option, tracks)).not.toThrow()
        // Album and year still apply even with no per-track assignments.
        expect(albumChangeCounts(option, tracks).albums).toBe(1)
    })
})

describe('summarizeAlbumChanges', () => {
    it('lists only the non-zero fields, pluralized', () => {
        expect(summarizeAlbumChanges({ titles: 12, artists: 0, albums: 1, years: 1 })).toBe(
            '12 titles · 1 album · 1 year'
        )
    })

    it('uses the singular form for a single change', () => {
        expect(summarizeAlbumChanges({ titles: 1, artists: 0, albums: 0, years: 0 })).toBe('1 title')
    })

    it('reads "No changes" when nothing would be rewritten', () => {
        expect(summarizeAlbumChanges({ titles: 0, artists: 0, albums: 0, years: 0 })).toBe('No changes')
    })
})
