import { computed } from 'vue'
import { useRadioStations } from '@/composables/useSubsonicQueries'
import type { InternetRadioStation } from '@/types/subsonic'

/**
 * Presentation-side view of the internet radio stations, shaped like
 * `useArtistTable` (minus the alphabet `letters`) so the radio grid/list
 * components share the same interface. Stations are sorted by name.
 */
export function useRadioTable() {
    const query = useRadioStations()

    const items = computed<InternetRadioStation[]>(() =>
        [...(query.data.value ?? [])].sort((a, b) =>
            a.name.localeCompare(b.name, undefined, { sensitivity: 'base' })
        )
    )
    const total = computed(() => items.value.length)

    return { total, items, isLoading: query.isLoading, error: query.error }
}
