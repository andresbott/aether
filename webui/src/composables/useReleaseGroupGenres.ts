import { getReleaseGroupGenres } from '@/lib/api/Artists'
import { createReleaseGroupCache } from './releaseGroupCache'

// Entry cap. One entry is a handful of short strings, so this is generous; the
// LRU only exists so a long session over a big library cannot grow unbounded.
export const MAX_GENRE_ENTRIES = 500

// One shared cache for the whole page — see createReleaseGroupCache for the
// throttle and module-scope rationale.
const cache = createReleaseGroupCache(getReleaseGroupGenres, MAX_GENRE_ENTRIES)

/**
 * useReleaseGroupGenres exposes the shared release-group genre cache. It holds no
 * reactive state: callers copy what they need into their own refs.
 */
export function useReleaseGroupGenres() {
    return cache
}
