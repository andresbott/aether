import { computed, unref } from 'vue'
import type { Ref, ComputedRef, MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { subsonicClient } from '@/lib/api/subsonic'
import type { AlbumLetter } from '@/types/subsonic'

export function useAlbumIndex(
    folderId: Ref<number | undefined> | ComputedRef<number | undefined>,
    options?: { enabled?: MaybeRefOrGetter<boolean> }
) {
    const query = useQuery({
        queryKey: computed(() => ['subsonic', 'albumIndex', unref(folderId)] as const),
        queryFn: () => subsonicClient.getAlbumIndex(unref(folderId)),
        staleTime: 2 * 60 * 1000,
        enabled: options?.enabled
    })

    const total = computed(() => query.data.value?.total ?? 0)
    const letters = computed<AlbumLetter[]>(() => query.data.value?.index ?? [])

    return { total, letters, isLoading: query.isLoading, error: query.error }
}
