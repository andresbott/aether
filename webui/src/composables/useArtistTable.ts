import { computed, unref } from 'vue'
import type { Ref, ComputedRef, MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Artist, AlbumLetter } from '@/types/subsonic'

export function useArtistTable(
    folderId: Ref<number | undefined> | ComputedRef<number | undefined>,
    options?: { enabled?: MaybeRefOrGetter<boolean> }
) {
    const query = useQuery({
        queryKey: computed(() => ['subsonic', 'artistIndex', unref(folderId)] as const),
        queryFn: () => subsonicClient.getArtistIndex(unref(folderId)),
        staleTime: 2 * 60 * 1000,
        enabled: options?.enabled
    })

    const total = computed(() => query.data.value?.total ?? 0)
    const letters = computed<AlbumLetter[]>(() => query.data.value?.letters ?? [])
    const items = computed<Artist[]>(() => query.data.value?.items ?? [])

    return { total, letters, items, isLoading: query.isLoading, error: query.error }
}
