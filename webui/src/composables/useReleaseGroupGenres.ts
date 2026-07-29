import { getReleaseGroupGenres } from '@/lib/api/Artists'

// In-memory cache of release-group genre lookups, shared by both identify
// dialogs. Genres are not part of an identify answer — AcoustID returns none and
// the release lookup carries only the sparse per-release list — so each dialog
// asks MusicBrainz for the genres of the release GROUP the user settled on. That
// call goes through the server's throttled MusicBrainz client, which allows ONE
// request per second, and the same group is asked for again every time a dialog
// is reopened or another song of the same album is identified.
//
// Module-scoped like useIdentifyCache, and for the same reason: the editor view
// is remounted on every navigation into Settings → Metadata, so a cache that
// died with the component would miss exactly the case it exists for. Genres of a
// release group are stable reference data, safe to hold for the page's lifetime.

// Entry cap. One entry is a handful of short strings, so this is generous; the
// LRU only exists so a long session over a big library cannot grow without
// bound.
export const MAX_GENRE_ENTRIES = 500

// Answers, most-recently-used last: a plain Map is an LRU as long as a read
// re-inserts its key, so eviction can take the first one.
const answers = new Map<string, string[]>()

// Requests currently open, so two callers asking for the same group during the
// same second share one request instead of queueing two against the throttle. A
// 12-song selection routinely resolves to a single release group.
const inFlight = new Map<string, Promise<string[]>>()

function touch(mbid: string, value: string[]) {
    answers.delete(mbid)
    answers.set(mbid, value)
    while (answers.size > MAX_GENRE_ENTRIES) {
        const oldest = answers.keys().next()
        if (oldest.done) return
        answers.delete(oldest.value)
    }
}

/**
 * useReleaseGroupGenres exposes the shared release-group genre cache. It holds no
 * reactive state: callers copy what they need into their own refs.
 */
export function useReleaseGroupGenres() {
    /**
     * cached is the answer already held for a release group, or undefined when it
     * has never been looked up. Lets a caller render genres synchronously for a
     * group it has seen before, with no request and no loading flicker.
     */
    function cached(mbid: string): string[] | undefined {
        const hit = answers.get(mbid)
        if (hit === undefined) return undefined
        touch(mbid, hit)
        return [...hit]
    }

    /**
     * lookup resolves the genres of a release group, from cache when possible.
     *
     * It never rejects. Genres are a nice-to-have on top of an identify match, so
     * a failed lookup resolves to no genres — the dialog then stages none rather
     * than losing the picks the user was in the middle of confirming. A failure is
     * NOT cached: MusicBrainz rate-limits and times out routinely, and a cached
     * failure would leave a release group permanently genre-less for the life of
     * the page.
     */
    async function lookup(mbid: string): Promise<string[]> {
        if (mbid === '') return []
        const hit = cached(mbid)
        if (hit !== undefined) return hit

        const open = inFlight.get(mbid)
        if (open) return open

        const request = getReleaseGroupGenres(mbid)
            .then((genres) => {
                // An empty list IS an answer ("nobody has tagged this group"), so
                // it is cached like any other — re-asking would spend a
                // rate-limited request to hear the same nothing.
                const out = genres ?? []
                touch(mbid, out)
                return out
            })
            .catch(() => [] as string[])
            .finally(() => {
                inFlight.delete(mbid)
            })
        inFlight.set(mbid, request)
        // Copy on the way out so a caller mutating its list cannot corrupt the
        // cached entry, matching cached().
        return (await request).slice()
    }

    function clear() {
        answers.clear()
        inFlight.clear()
    }

    return { cached, lookup, clear }
}
