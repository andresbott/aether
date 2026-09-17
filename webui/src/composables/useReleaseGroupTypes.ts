import { getReleaseGroupTypes } from '@/lib/api/Artists'
import { createReleaseGroupCache } from './releaseGroupCache'

// Entry cap, mirroring MAX_GENRE_ENTRIES.
export const MAX_TYPE_ENTRIES = 500

const cache = createReleaseGroupCache(getReleaseGroupTypes, MAX_TYPE_ENTRIES)

/**
 * useReleaseGroupTypes exposes the shared release-group type cache (primary +
 * secondary MusicBrainz types) — the release-type twin of useReleaseGroupGenres,
 * so an identify pick can pre-fill the editor's release-type field.
 */
export function useReleaseGroupTypes() {
    return cache
}
