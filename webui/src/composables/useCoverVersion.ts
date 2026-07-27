import { reactive } from 'vue'

// Cover URLs are stable while the image behind them is not: `getCoverArt.view?id=ar-63`
// returns whatever art currently wins for that artist. The server answers 200
// with a fresh ETag when the image changes, but the browser's *in-memory* image
// cache never revalidates within a session — so after uploading a cover,
// navigating away and back re-requests the same URL and the old bitmap is reused
// until a hard refresh.
//
// A per-entity version counter appended to the URL fixes that. It lives at module
// scope on purpose: a component-local ref dies with the component that bumped
// it, which is exactly the navigate-away case we are fixing.
//
// Reset on page load is fine — a fresh document has no stale in-memory cache, and
// the ETag handles the HTTP cache.
const versions = reactive<Record<string, number>>({})

/** Current cache-busting version for a cover art id (0 = never changed). */
export function coverVersion(coverArtId: string): number {
    return versions[coverArtId] ?? 0
}

/** Records that this entity's cover changed, invalidating URLs built for it. */
export function bumpCoverVersion(coverArtId: string): void {
    versions[coverArtId] = coverVersion(coverArtId) + 1
}

/** Appends the version to a cover URL, leaving never-changed covers untouched. */
export function versionedCoverUrl(url: string, coverArtId: string): string {
    const v = coverVersion(coverArtId)
    return v > 0 ? `${url}&_v=${v}` : url
}

/** Test hook: clears all recorded versions. */
export function resetCoverVersions(): void {
    for (const key of Object.keys(versions)) delete versions[key]
}
