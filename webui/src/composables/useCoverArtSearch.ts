import { ref } from 'vue'
import { listReleaseCovers } from '@/lib/api/Metadata'
import type { CoverCandidate } from '@/types/metadata'

// useCoverArtSearch looks up album cover candidates from the Cover Art Archive
// by MusicBrainz release id (and optional release-group id fallback).
export function useCoverArtSearch() {
    const candidates = ref<CoverCandidate[]>([])
    const loading = ref(false)
    const error = ref<string | null>(null)

    async function search(mbid: string, releaseGroup?: string) {
        const rel = mbid.trim()
        const grp = releaseGroup?.trim() ?? ''
        if (!rel && !grp) {
            candidates.value = []
            error.value = null
            return
        }
        loading.value = true
        error.value = null
        try {
            candidates.value = await listReleaseCovers(rel, grp || undefined)
        } catch (err: any) {
            error.value = err?.response?.data?.error ?? err.message ?? 'Cover search failed'
            candidates.value = []
        } finally {
            loading.value = false
        }
    }

    return { candidates, loading, error, search }
}
