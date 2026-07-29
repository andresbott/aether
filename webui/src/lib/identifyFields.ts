import type { TrackOverlay } from '@/types/metadata'

// The fields an identify match can stage, as both identify dialogs offer them
// (single-song and album). Grouped for the user rather than one checkbox per tag:
// the three MusicBrainz IDs are a single choice because staging one without the
// others yields a file whose IDs disagree with its tags, and "Artists" covers
// both the track credits and the album credits since an album identify derives
// both from the one release the user picked.
export interface IdentifyFieldDef {
    id: IdentifyFieldId
    label: string
    // The TrackOverlay keys this checkbox controls.
    keys: (keyof TrackOverlay)[]
}

export type IdentifyFieldId =
    | 'title'
    | 'artists'
    | 'album'
    | 'genres'
    | 'year'
    | 'track_number'
    | 'disc_number'
    | 'mbids'

export const IDENTIFY_FIELDS: readonly IdentifyFieldDef[] = [
    { id: 'title', label: 'Title', keys: ['title'] },
    { id: 'artists', label: 'Artists', keys: ['artists', 'album_artists'] },
    { id: 'album', label: 'Album', keys: ['album'] },
    // Genres come from the release GROUP, not from the fingerprint match: both
    // identify flows look them up separately once the user has settled on a
    // release. An identify run that resolved no release group stages none, and
    // the checkbox then simply controls nothing.
    { id: 'genres', label: 'Genres', keys: ['genres'] },
    { id: 'year', label: 'Year', keys: ['year'] },
    { id: 'track_number', label: 'Track number', keys: ['track_number'] },
    { id: 'disc_number', label: 'Disc number', keys: ['disc_number'] },
    {
        id: 'mbids',
        label: 'MusicBrainz IDs',
        keys: ['mb_recording_id', 'mb_release_id', 'mb_release_group_id']
    }
] as const

export const ALL_IDENTIFY_FIELD_IDS: readonly IdentifyFieldId[] = IDENTIFY_FIELDS.map((f) => f.id)

/**
 * overlayKeysForFields expands a set of chosen checkbox ids into the overlay
 * keys they cover.
 */
export function overlayKeysForFields(
    fields: readonly IdentifyFieldId[]
): Set<keyof TrackOverlay> {
    const out = new Set<keyof TrackOverlay>()
    for (const def of IDENTIFY_FIELDS) {
        if (!fields.includes(def.id)) continue
        for (const k of def.keys) out.add(k)
    }
    return out
}

/**
 * pickOverlayFields drops every staged value the user did not select. Used to
 * turn a full identify overlay into a partial one — e.g. stage only the album
 * on a batch of files without touching their titles.
 */
export function pickOverlayFields(
    overlay: TrackOverlay,
    fields: readonly IdentifyFieldId[]
): TrackOverlay {
    const allowed = overlayKeysForFields(fields)
    const out: TrackOverlay = {}
    for (const key of Object.keys(overlay) as (keyof TrackOverlay)[]) {
        if (!allowed.has(key)) continue
        // Assigning through a union of value types needs the cast; the key/value
        // pairing itself comes straight from `overlay`, so it stays consistent.
        ;(out as Record<string, unknown>)[key] = overlay[key]
    }
    return out
}
