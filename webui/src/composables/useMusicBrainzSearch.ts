import { ref } from 'vue'
import { searchMusicBrainzArtists } from '@/lib/api/Artists'
import { apiErrorMessage, isRateLimitError } from '@/lib/apiError'
import type { MusicBrainzCandidate } from '@/types/artists'

export function useMusicBrainzSearch() {
    const results = ref<MusicBrainzCandidate[]>([])
    const loading = ref(false)
    const error = ref<string | null>(null)
    const rateLimited = ref(false)

    async function search(query: string) {
        const q = query.trim()
        if (q.length < 2) {
            results.value = []
            error.value = null
            rateLimited.value = false
            return
        }
        loading.value = true
        error.value = null
        rateLimited.value = false
        try {
            results.value = await searchMusicBrainzArtists(q)
        } catch (err: unknown) {
            error.value = apiErrorMessage(err, 'The artist search could not be completed. Try again in a moment.')
            rateLimited.value = isRateLimitError(err)
            results.value = []
        } finally {
            loading.value = false
        }
    }

    return { results, loading, error, rateLimited, search }
}
