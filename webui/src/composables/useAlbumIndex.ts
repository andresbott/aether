import { computed, unref, toValue } from 'vue'
import type { Ref, ComputedRef, MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { subsonicClient } from '@/lib/api/subsonic'
import { queryKeys } from '@/composables/useSubsonicQueries'
import type { AlbumLetter } from '@/types/subsonic'

export function useAlbumIndex(
    folderId: Ref<number | undefined> | ComputedRef<number | undefined>,
    options?: { enabled?: MaybeRefOrGetter<boolean> },
    releaseType?: MaybeRefOrGetter<string | undefined>
) {
    const query = useQuery({
        queryKey: computed(() => queryKeys.albumIndex(unref(folderId), toValue(releaseType))),
        queryFn: () => subsonicClient.getAlbumIndex(unref(folderId), toValue(releaseType)),
        staleTime: 2 * 60 * 1000,
        enabled: options?.enabled
    })

    const total = computed(() => query.data.value?.total ?? 0)
    const letters = computed<AlbumLetter[]>(() => query.data.value?.index ?? [])

    return { total, letters, isLoading: query.isLoading, error: query.error }
}
