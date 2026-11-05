import { computed, unref } from 'vue'
import type { Ref, ComputedRef, MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { subsonicClient } from '@/lib/api/subsonic'
import { queryKeys } from '@/composables/useSubsonicQueries'
import type { Grouping, AlbumLetter } from '@/types/subsonic'

/** The groupings index (the `albumGroupings` extension), whole catalog or one library. */
export function useGroupingTable(
    folderId: Ref<number | undefined> | ComputedRef<number | undefined>,
    options?: { enabled?: MaybeRefOrGetter<boolean> }
) {
    const query = useQuery({
        queryKey: computed(() => queryKeys.groupingIndex(unref(folderId))),
        queryFn: () => subsonicClient.getGroupingIndex(unref(folderId)),
        staleTime: 2 * 60 * 1000,
        enabled: options?.enabled
    })

    const total = computed(() => query.data.value?.total ?? 0)
    const letters = computed<AlbumLetter[]>(() => query.data.value?.letters ?? [])
    const items = computed<Grouping[]>(() => query.data.value?.items ?? [])

    return { total, letters, items, isLoading: query.isLoading, error: query.error }
}
