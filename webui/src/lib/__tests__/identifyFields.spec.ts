import { describe, it, expect } from 'vitest'
import {
    ALL_IDENTIFY_FIELD_IDS,
    IDENTIFY_FIELDS,
    overlayKeysForFields,
    pickOverlayFields
} from '@/lib/identifyFields'
import { albumPickToOverlay, candidateToOverlay } from '@/composables/useEditSession'
import type { AlbumIdentifyPick, IdentifyCandidate, TrackOverlay } from '@/types/metadata'

// Every key the two identify mappings can produce: candidateToOverlay's set plus
// album_artists, which only albumPickToOverlay stages.
const full: TrackOverlay = {
    title: 'Song',
    artists: [{ name: 'Artist', mbid: 'artist-id' }],
    album_artists: [{ name: 'Album Artist', mbid: 'album-artist-id' }],
    album: 'Album',
    genres: ['Grunge', 'Alternative Rock'],
    release_types: ['Album', 'Live'],
    year: 1999,
    track_number: 3,
    disc_number: 1,
    mb_recording_id: 'rec-id',
    mb_release_id: 'rel-id',
    mb_release_group_id: 'rg-id'
}

// Inputs that take every branch of the two identify mappings, so the overlays
// built from them carry every key the mappings can produce.
const candidate: IdentifyCandidate = {
    score: 0.97,
    recording_mbid: 'rec-id',
    title: 'Song',
    artists: [{ name: 'Artist', mbid: 'artist-id' }],
    releases: [
        {
            release_mbid: 'rel-id',
            release_group_mbid: 'rg-id',
            album: 'Album',
            year: 1999,
            track_number: 3,
            disc_number: 1
        }
    ]
}

const albumPick: AlbumIdentifyPick = {
    path: 'a.mp3',
    option: {
        release_mbid: 'rel-id',
        release_group_mbid: 'rg-id',
        album: 'Album',
        year: 1999,
        artists: [{ name: 'Album Artist', mbid: 'album-artist-id' }],
        track_count: 12,
        disc_count: 1,
        enriched: true,
        matched_count: 1,
        mean_score: 0.97,
        assignments: [],
        tracks: []
    },
    assignment: {
        path: 'a.mp3',
        source: 'fingerprint',
        title: 'Song',
        recording_mbid: 'rec-id',
        artists: [{ name: 'Artist', mbid: 'artist-id' }],
        disc_number: 1,
        track_number: 3,
        score: 0.97
    },
    genres: ['Grunge'],
    releaseTypes: ['Album']
}

describe('overlayKeysForFields', () => {
    it('expands the MusicBrainz group into all three id keys', () => {
        expect([...overlayKeysForFields(['mbids'])].sort()).toEqual([
            'mb_recording_id',
            'mb_release_group_id',
            'mb_release_id'
        ])
    })

    it('ignores unknown ids and returns nothing for an empty selection', () => {
        expect(overlayKeysForFields([]).size).toBe(0)
    })

    it('maps the Genres choice onto the genres key', () => {
        expect([...overlayKeysForFields(['genres'])]).toEqual(['genres'])
    })

    it('covers track and album credits with the one Artists choice', () => {
        // An album identify derives both from the single release the user picked,
        // so splitting them would offer a distinction the data does not have.
        expect([...overlayKeysForFields(['artists'])].sort()).toEqual([
            'album_artists',
            'artists'
        ])
    })
})

describe('pickOverlayFields', () => {
    it('keeps every field when everything is selected', () => {
        expect(pickOverlayFields(full, ALL_IDENTIFY_FIELD_IDS)).toEqual(full)
    })

    it('keeps only the selected field — the fill-one-field case', () => {
        expect(pickOverlayFields(full, ['album'])).toEqual({ album: 'Album' })
    })

    it('keeps only the genres when that is the one selected field', () => {
        // The "fill in the genres of an album whose tags are otherwise right"
        // case, which is what the genre lookup exists for.
        expect(pickOverlayFields(full, ['genres'])).toEqual({
            genres: ['Grunge', 'Alternative Rock']
        })
    })

    it('keeps only the release types when that is the one selected field', () => {
        // The same fill-one-field case as genres: take the release group's type
        // for albums whose other tags are already right.
        expect(pickOverlayFields(full, ['release_types'])).toEqual({
            release_types: ['Album', 'Live']
        })
    })

    it('stages nothing when no field is selected', () => {
        expect(pickOverlayFields(full, [])).toEqual({})
    })

    it('does not invent keys the overlay never had', () => {
        // Identify found no release, so there is no album/year to stage even
        // though the user asked for them.
        const partial: TrackOverlay = { title: 'Song', mb_recording_id: 'rec-id' }
        expect(pickOverlayFields(partial, ['album', 'year', 'title'])).toEqual({ title: 'Song' })
    })

    it('covers every overlay key the identify mappings can produce', () => {
        // Guards against a new field appearing in candidateToOverlay/
        // albumPickToOverlay without a checkbox to control it: an uncovered key
        // would be silently dropped from every apply in BOTH dialogs. The keys
        // come from the real mappings, not a hand-kept list — a list is how
        // release_types went uncovered — so feed any new mapping input in here.
        const produced = new Set([
            ...Object.keys(candidateToOverlay(candidate, candidate.releases[0], ['Grunge'], ['Album'])),
            ...Object.keys(albumPickToOverlay(albumPick, albumPick.genres, albumPick.releaseTypes))
        ])
        const covered = overlayKeysForFields(ALL_IDENTIFY_FIELD_IDS)
        expect([...produced].filter((k) => !covered.has(k as keyof TrackOverlay))).toEqual([])
    })

    it('exposes one id per registry entry with no duplicates', () => {
        expect(ALL_IDENTIFY_FIELD_IDS).toHaveLength(IDENTIFY_FIELDS.length)
        expect(new Set(ALL_IDENTIFY_FIELD_IDS).size).toBe(IDENTIFY_FIELDS.length)
    })
})
