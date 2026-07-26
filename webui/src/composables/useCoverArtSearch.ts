import { ref } from 'vue'
import { listReleaseCovers } from '@/lib/api/Metadata'
import { apiErrorMessage, isRateLimitError } from '@/lib/apiError'
import type { CoverCandidate } from '@/types/metadata'

// useCoverArtSearch looks up album cover candidates from the Cover Art Archive
// by MusicBrainz release id (and optional release-group id fallback).
//
// The archive is a third-party service that does go down and does throttle, so
// a failure here is expected traffic, not a bug: the server retries transient
// failures and answers with a ready-to-show sentence (see internal/upstream),
// which `error` carries verbatim. `rateLimited` tells "wait and retry" apart
// from "it is broken" so the UI can offer a retry.
export function useCoverArtSearch() {
    const candidates = ref<CoverCandidate[]>([])
    const loading = ref(false)
    const error = ref<string | null>(null)
    const rateLimited = ref(false)

    async function search(mbid: string, releaseGroup?: string) {
        const rel = mbid.trim()
        const grp = releaseGroup?.trim() ?? ''
        if (!rel && !grp) {
            candidates.value = []
            error.value = null
            rateLimited.value = false
            return
        }
        loading.value = true
        error.value = null
        rateLimited.value = false
        try {
            candidates.value = await listReleaseCovers(rel, grp || undefined)
        } catch (err: unknown) {
            error.value = apiErrorMessage(
                err,
                'Cover art could not be loaded right now. Try again in a moment.'
            )
            rateLimited.value = isRateLimitError(err)
            candidates.value = []
        } finally {
            loading.value = false
        }
    }

    return { candidates, loading, error, rateLimited, search }
}
