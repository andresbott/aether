// MusicBrainz release-type vocabulary shared by the metadata editor (primary +
// secondary controls), the Albums view's release-type tabs and the album hero.
// A release group has exactly one primary type and zero-or-more secondary types.
// Storage and the wire are a single flat list; the primary/secondary split below
// is presentation only.
export const PRIMARY_RELEASE_TYPES = ['Album', 'Single', 'EP', 'Broadcast', 'Other'] as const

export const SECONDARY_RELEASE_TYPES = [
    'Compilation',
    'Soundtrack',
    'Spokenword',
    'Interview',
    'Audiobook',
    'Audio drama',
    'Live',
    'Remix',
    'DJ-mix',
    'Mixtape/Street',
    'Demo',
    'Field recording'
] as const

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
export function splitReleaseTypes(list: readonly string[]): { primary: string; secondary: string[] } {
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

/**
 * formatReleaseTypes renders an album's release types as one display label —
 * primary first, in vocabulary casing, joined with " · " ("Album · Soundtrack").
 * Returns '' when the album carries none, so callers pick their own fallback.
 */
export function formatReleaseTypes(list: readonly string[] | undefined): string {
    const { primary, secondary } = splitReleaseTypes(list ?? [])
    return mergeReleaseTypes(primary, secondary).join(' · ')
}
