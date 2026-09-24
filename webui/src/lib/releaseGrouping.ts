import type { Album } from '@/types/subsonic'
import { PRIMARY_RELEASE_TYPES } from '@/lib/releaseTypes'

type PrimaryReleaseType = (typeof PRIMARY_RELEASE_TYPES)[number]

export interface ReleaseGroup {
    type: PrimaryReleaseType
    label: string
    albums: Album[]
}

// Plural section headings, matching the Albums view's release-type tabs.
const LABELS: Record<PrimaryReleaseType, string> = {
    Album: 'Albums',
    Single: 'Singles',
    EP: 'EPs',
    Broadcast: 'Broadcast',
    Other: 'Other'
}

// Discography display order — EP before Single, deliberately unlike the vocabulary
// order; every other primary type trails toward Other at the end.
const DISPLAY_ORDER: PrimaryReleaseType[] = ['Album', 'EP', 'Single', 'Broadcast', 'Other']

// Album and EP always stand alone; every other primary type with fewer than this
// many releases is folded into Other so the page is not littered with one-item
// sections.
const ALWAYS_STANDALONE = new Set<PrimaryReleaseType>(['Album', 'EP'])
const FOLD_THRESHOLD = 4

const CANONICAL = new Map<string, PrimaryReleaseType>(
    PRIMARY_RELEASE_TYPES.map((t) => [t.toLowerCase(), t])
)

// An album's MusicBrainz primary type: the first releaseTypes value in the primary
// vocabulary (case-insensitive). Untyped or secondary-only releases default to
// Album — the friendly bucket for the common "no type tag" case.
function primaryType(album: Album): PrimaryReleaseType {
    for (const raw of album.releaseTypes ?? []) {
        const canonical = CANONICAL.get(raw.toLowerCase())
        if (canonical) return canonical
    }
    return 'Album'
}

/**
 * Group an artist's discography by MusicBrainz release type for display: one
 * ordered group per surviving type (Albums, EPs, Singles, Broadcast, Other),
 * newest first within each. Small non-Album/EP types fold into Other.
 */
export function groupAlbumsByReleaseType(albums: Album[]): ReleaseGroup[] {
    const buckets = new Map<PrimaryReleaseType, Album[]>()
    for (const album of albums) {
        const type = primaryType(album)
        const bucket = buckets.get(type)
        if (bucket) bucket.push(album)
        else buckets.set(type, [album])
    }

    // Fold small non-standalone buckets into Other.
    const other = buckets.get('Other') ?? []
    for (const [type, bucket] of buckets) {
        if (type === 'Other' || ALWAYS_STANDALONE.has(type)) continue
        if (bucket.length < FOLD_THRESHOLD) {
            other.push(...bucket)
            buckets.delete(type)
        }
    }
    if (other.length > 0) buckets.set('Other', other)

    return DISPLAY_ORDER.flatMap((type) => {
        const bucket = buckets.get(type)
        if (!bucket || bucket.length === 0) return []
        const sorted = [...bucket].sort((a, b) => (b.year || 0) - (a.year || 0))
        return [{ type, label: LABELS[type], albums: sorted }]
    })
}
