import { ref } from 'vue'
import { getArtistImageCandidates } from '@/lib/api/Artists'
import { apiErrorMessage } from '@/lib/apiError'
import type { ArtistImageCandidate } from '@/types/artists'

// useArtistImageCandidates loads the provider portrait candidates for a chosen
// MusicBrainz artist. The grid loads each thumbnail directly from the provider
// CDN; this only fetches the URL list.
export function useArtistImageCandidates() {
    const candidates = ref<ArtistImageCandidate[]>([])
    const loading = ref(false)
    const error = ref<string | null>(null)

    async function load(mbid: string) {
        loading.value = true
        error.value = null
        candidates.value = []
        try {
            candidates.value = await getArtistImageCandidates(mbid)
        } catch (err: unknown) {
            error.value = apiErrorMessage(err, 'The image lookup could not be completed. Try again in a moment.')
            candidates.value = []
        } finally {
            loading.value = false
        }
    }

    function reset() {
        candidates.value = []
        error.value = null
        loading.value = false
    }

    return { candidates, loading, error, load, reset }
}
