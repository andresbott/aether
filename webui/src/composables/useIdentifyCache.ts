import type { IdentifyAlbumResponse, IdentifyTrackResult } from '@/types/metadata'

// In-memory cache of identify answers, so closing a dialog and opening it again
// does not pay for the lookups a second time. Identification is by far the most
// expensive thing the editor does — one fpcalc run per file plus rate-limited
// AcoustID and MusicBrainz calls — and it answers a question about the audio
// content, which tag edits do not change. That makes the answers safe to keep
// for as long as the page lives.
//
// Deliberately NOT a vue-query cache: identification is a POST mutation over a
// path list, so there is no stable query key to hang it on, and per-path reuse
// (identify 12 files, then identify 3 of them again) needs a per-path store
// rather than one entry per request.
//
// Module-scoped on purpose, like usePlayer: the editor view is remounted on
// every navigation into Settings → Metadata, and a cache that died with the
// component would miss exactly the case this exists for.

// Caps, so a long editing session over a big library cannot grow the cache
// without bound. LRU: the entries a user keeps coming back to are the ones worth
// keeping. Track entries are small (a handful of candidates) so the cap is
// generous; album entries carry whole tracklists per option, hence the tighter
// one.
export const MAX_TRACK_ENTRIES = 2000
export const MAX_ALBUM_ENTRIES = 50

// Key separator. A tab cannot appear in a library-relative path the server
// hands out, so it can never be confused with path content.
const SEP = '\t'

// A path is only unique within its library, so the key carries the library id.
function trackKey(libraryId: number, path: string): string {
    return `${libraryId}${SEP}${path}`
}

// The album lookup answers a question about a SET of files ("which release are
// these?"), so the whole set is the key — sorted, because the same selection in
// a different order is the same question.
function albumKey(libraryId: number, paths: string[]): string {
    return `${libraryId}${SEP}${[...paths].sort().join(SEP)}`
}

const trackResults = new Map<string, IdentifyTrackResult>()
const albumResponses = new Map<string, IdentifyAlbumResponse>()

// touch moves an entry to the end of the insertion order, which is what makes a
// plain Map an LRU: eviction takes the first key.
function touch<T>(map: Map<string, T>, key: string, value: T) {
    map.delete(key)
    map.set(key, value)
}

function evict<T>(map: Map<string, T>, cap: number) {
    while (map.size > cap) {
        const oldest = map.keys().next()
        if (oldest.done) return
        map.delete(oldest.value)
    }
}

/**
 * mergeTrackResults folds cached and freshly fetched results back into the
 * order the caller asked for, so a partially cached run reads like a single
 * lookup rather than "the cached ones first". A fresh result wins over a cached
 * one for the same path (the caller only refetches a path it wants renewed),
 * and a requested path nothing covers is simply absent.
 */
export function mergeTrackResults(
    paths: string[],
    cached: IdentifyTrackResult[],
    fresh: IdentifyTrackResult[]
): IdentifyTrackResult[] {
    const byPath = new Map<string, IdentifyTrackResult>()
    for (const r of cached) byPath.set(r.path, r)
    for (const r of fresh) byPath.set(r.path, r)
    const out: IdentifyTrackResult[] = []
    for (const path of paths) {
        const r = byPath.get(path)
        if (r) out.push(r)
    }
    return out
}

export interface CachedTrackResults {
    cached: IdentifyTrackResult[]
    // The paths the caller still has to ask the server about.
    missing: string[]
}

/**
 * useIdentifyCache exposes the shared identify cache. It holds no reactive
 * state: callers copy what they need into their own refs, so nothing here has
 * to be watched.
 */
export function useIdentifyCache() {
    function getTrackResults(libraryId: number, paths: string[]): CachedTrackResults {
        const cached: IdentifyTrackResult[] = []
        const missing: string[] = []
        for (const path of paths) {
            const key = trackKey(libraryId, path)
            const hit = trackResults.get(key)
            if (hit === undefined) {
                missing.push(path)
                continue
            }
            touch(trackResults, key, hit)
            cached.push(hit)
        }
        return { cached, missing }
    }

    // A result that carries an error is not stored: the failure is usually
    // transient (a rate-limited lookup, a busy disk) and caching it would make a
    // retry impossible without reloading the page. A result with no candidates
    // and no error IS stored — "this file matches nothing" is a real answer, and
    // re-running a full fingerprint pass to hear it again is the waste this
    // cache exists to avoid.
    function putTrackResults(libraryId: number, results: IdentifyTrackResult[]) {
        for (const r of results) {
            if (r.error) continue
            touch(trackResults, trackKey(libraryId, r.path), r)
        }
        evict(trackResults, MAX_TRACK_ENTRIES)
    }

    function getAlbumResponse(
        libraryId: number,
        paths: string[]
    ): IdentifyAlbumResponse | undefined {
        const key = albumKey(libraryId, paths)
        const hit = albumResponses.get(key)
        if (hit === undefined) return undefined
        touch(albumResponses, key, hit)
        return hit
    }

    // Stored whole, errors included: unlike the per-track lookup there is no
    // partial reuse to be had, and the per-path errors are part of the answer
    // the dialog renders. A response with no options at all is still an answer.
    function putAlbumResponse(
        libraryId: number,
        paths: string[],
        response: IdentifyAlbumResponse
    ) {
        touch(albumResponses, albumKey(libraryId, paths), response)
        evict(albumResponses, MAX_ALBUM_ENTRIES)
    }

    function clear() {
        trackResults.clear()
        albumResponses.clear()
    }

    return {
        getTrackResults,
        putTrackResults,
        getAlbumResponse,
        putAlbumResponse,
        clear
    }
}
