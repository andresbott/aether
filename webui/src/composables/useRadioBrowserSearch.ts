import { ref } from 'vue'
import { searchRadioStations } from '@/lib/api/RadioBrowser'
import type { RadioBrowserStation } from '@/types/radiobrowser'

export function useRadioBrowserSearch() {
    const results = ref<RadioBrowserStation[]>([])
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
            results.value = await searchRadioStations(q)
        } catch (err: any) {
            error.value = err?.response?.data?.error ?? err.message ?? 'Search failed'
            results.value = []
        } finally {
            loading.value = false
        }
    }

    return { results, loading, error, search }
}
