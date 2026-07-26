import { ref } from 'vue'
import { searchRadioStations } from '@/lib/api/RadioBrowser'
import { apiErrorMessage, isRateLimitError } from '@/lib/apiError'
import type { RadioBrowserStation } from '@/types/radiobrowser'

export function useRadioBrowserSearch() {
    const results = ref<RadioBrowserStation[]>([])
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
            results.value = await searchRadioStations(q)
        } catch (err: unknown) {
            error.value = apiErrorMessage(err, 'The station directory could not be reached. Try again in a moment.')
            rateLimited.value = isRateLimitError(err)
            results.value = []
        } finally {
            loading.value = false
        }
    }

    return { results, loading, error, rateLimited, search }
}
