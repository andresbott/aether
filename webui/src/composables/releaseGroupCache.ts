// createReleaseGroupCache builds a module-scoped LRU + in-flight cache around a
// per-release-group MusicBrainz lookup that returns a list of short strings.
// Shared by useReleaseGroupGenres and useReleaseGroupTypes so the cache logic
// lives in one place.
//
// Why a shared module-scoped cache: neither genres nor release types are part of
// an identify answer, so each dialog asks MusicBrainz for them keyed on the
// release GROUP the user settled on. That call goes through the server's
// throttled MusicBrainz client (ONE request per second), and the same group is
// asked for again every time a dialog reopens or another song of the same album
// is identified. Module-scoped (like useIdentifyCache) because the editor view
// remounts on every navigation into Settings → Metadata, so a cache that died
// with the component would miss exactly the case it exists for.
export function createReleaseGroupCache(
    fetchList: (mbid: string) => Promise<string[]>,
    maxEntries = 500
) {
    // Answers, most-recently-used last: a plain Map is an LRU as long as a read
    // re-inserts its key, so eviction can take the first one.
    const answers = new Map<string, string[]>()
    // Requests currently open, so two callers asking for the same group during
    // the same second share one request instead of queueing two against the
    // throttle.
    const inFlight = new Map<string, Promise<string[]>>()

    function touch(mbid: string, value: string[]) {
        answers.delete(mbid)
        answers.set(mbid, value)
        while (answers.size > maxEntries) {
            const oldest = answers.keys().next()
            if (oldest.done) return
            answers.delete(oldest.value)
        }
    }

    // cached is the answer already held for a group, or undefined when it has
    // never been looked up — so a caller can render synchronously with no flicker.
    function cached(mbid: string): string[] | undefined {
        const hit = answers.get(mbid)
        if (hit === undefined) return undefined
        touch(mbid, hit)
        return [...hit]
    }

    // lookup resolves the list for a group, from cache when possible. It never
    // rejects: the list is a nice-to-have on top of an identify match, so a
    // failed lookup resolves to an empty list and the dialog stages nothing
    // rather than losing the picks the user was confirming. A failure is NOT
    // cached (MusicBrainz rate-limits routinely, and a cached failure would leave
    // a group permanently empty); an empty answer IS cached ("nobody tagged it").
    async function lookup(mbid: string): Promise<string[]> {
        if (mbid === '') return []
        const hit = cached(mbid)
        if (hit !== undefined) return hit

        const open = inFlight.get(mbid)
        if (open) return open

        const request = fetchList(mbid)
            .then((list) => {
                const out = list ?? []
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
