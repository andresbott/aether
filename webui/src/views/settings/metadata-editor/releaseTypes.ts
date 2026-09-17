// Storage and the wire are a single flat list; the primary/secondary split
// below is UX only. The vocabulary itself now lives in @/lib (shared with the
// Releases browse tabs) — re-exported here so existing editor imports keep
// resolving.
import { PRIMARY_RELEASE_TYPES, SECONDARY_RELEASE_TYPES } from '@/lib/releaseTypes'
export { PRIMARY_RELEASE_TYPES, SECONDARY_RELEASE_TYPES }

function canonical(vocab: readonly string[], value: string): string | undefined {
    const v = value.trim().toLowerCase()
    return vocab.find((x) => x.toLowerCase() === v)
}

/**
 * splitReleaseTypes classifies a flat MusicBrainz album-type list into the
 * editor's primary (single-select) + secondary (multi-select) controls. Casing
 * is canonicalized to the vocabulary, so a Vorbis file's lowercase "album" reads
 * back as "Album". The first primary-vocabulary value wins the primary slot;
 * everything else is a secondary. An unrecognized value is kept verbatim as a
 * secondary so an odd tag round-trips instead of vanishing on save.
 */
export function splitReleaseTypes(list: string[]): { primary: string; secondary: string[] } {
    let primary = ''
    const secondary: string[] = []
    for (const raw of list) {
        const p = canonical(PRIMARY_RELEASE_TYPES, raw)
        if (p && primary === '') {
            primary = p
            continue
        }
        secondary.push(canonical(SECONDARY_RELEASE_TYPES, raw) ?? raw.trim())
    }
    return { primary, secondary }
}

/**
 * mergeReleaseTypes flattens the two controls back into the stored list, primary
 * first (MusicBrainz order), dropping blanks.
 */
export function mergeReleaseTypes(primary: string, secondary: string[]): string[] {
    const out: string[] = []
    // `primary &&` guards against a null from a clearable Select.
    if (primary && primary.trim() !== '') out.push(primary.trim())
    for (const s of secondary) {
        if (s.trim() !== '') out.push(s.trim())
    }
    return out
}
