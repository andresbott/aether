import { describe, it, expect } from 'vitest'
import {
    ALL_IDENTIFY_FIELD_IDS,
    IDENTIFY_FIELDS,
    overlayKeysForFields,
    pickOverlayFields
} from '@/lib/identifyFields'
import type { TrackOverlay } from '@/types/metadata'

// Every key the two identify mappings can produce: candidateToOverlay's set plus
// album_artists, which only albumPickToOverlay stages.
const full: TrackOverlay = {
    title: 'Song',
    artists: [{ name: 'Artist', mbid: 'artist-id' }],
    album_artists: [{ name: 'Album Artist', mbid: 'album-artist-id' }],
    album: 'Album',
    year: 1999,
    track_number: 3,
    disc_number: 1,
    mb_recording_id: 'rec-id',
    mb_release_id: 'rel-id',
    mb_release_group_id: 'rg-id'
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
        // would be silently dropped from every apply in BOTH dialogs.
        const covered = overlayKeysForFields(ALL_IDENTIFY_FIELD_IDS)
        for (const key of Object.keys(full) as (keyof TrackOverlay)[]) {
            expect(covered.has(key)).toBe(true)
        }
    })

    it('exposes one id per registry entry with no duplicates', () => {
        expect(ALL_IDENTIFY_FIELD_IDS).toHaveLength(IDENTIFY_FIELDS.length)
        expect(new Set(ALL_IDENTIFY_FIELD_IDS).size).toBe(IDENTIFY_FIELDS.length)
    })
})
