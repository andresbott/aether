import { ref } from 'vue'
import { searchMusicBrainzReleases } from '@/lib/api/Artists'
import type { MusicBrainzReleaseCandidate } from '@/types/artists'

export function useMusicBrainzReleaseSearch() {
    const results = ref<MusicBrainzReleaseCandidate[]>([])
    const loading = ref(false)
    const error = ref<string | null>(null)

    async function search(query: string) {
        const q = query.trim()
        if (q.length < 2) {
            results.value = []
            error.value = null
            return
        }
        loading.value = true
        error.value = null
        try {
            results.value = await searchMusicBrainzReleases(q)
        } catch (err: any) {
            error.value = err?.response?.data?.error ?? err.message ?? 'Search failed'
            results.value = []
        } finally {
            loading.value = false
        }
    }

    return { results, loading, error, search }
}
