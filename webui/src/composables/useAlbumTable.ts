import { ref, unref, toValue, watch } from 'vue'
import type { Ref, ComputedRef, MaybeRefOrGetter } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { subsonicClient } from '@/lib/api/subsonic'
import { queryKeys } from '@/composables/useSubsonicQueries'
import type { Album } from '@/types/subsonic'
import { useAlbumIndex } from '@/composables/useAlbumIndex'

export const ALBUM_PAGE_SIZE = 100

export function useAlbumTable(
    folderId: Ref<number | undefined> | ComputedRef<number | undefined>,
    options?: { enabled?: MaybeRefOrGetter<boolean> },
    releaseType?: MaybeRefOrGetter<string | undefined>
) {
    const queryClient = useQueryClient()

    const { total, letters, isLoading, error } = useAlbumIndex(folderId, options, releaseType)

    const items = ref<(Album | undefined)[]>([])
    let loadedPages = new Set<number>()
    let generation = 0

    // Reset the sparse array whenever the library, its size, OR the release-type
    // filter changes — a new filter is a different dataset.
    watch(
        [total, () => unref(folderId), () => toValue(releaseType)],
        () => {
            items.value = new Array<Album | undefined>(total.value)
            loadedPages = new Set<number>()
            generation++
        },
        { immediate: true }
    )

    async function ensureRange(first: number, last: number): Promise<void> {
        if (total.value === 0) return
        // Captured once per call: if a reset bumps `generation` while one of this
        // call's page fetches is still in flight (e.g. a slow page for the old
        // filter resolving after a fast reset for the new one), the stale result
        // is dropped instead of being written into the now-current window.
        const gen = generation
        const firstPage = Math.floor(Math.max(0, first) / ALBUM_PAGE_SIZE)
        const lastPage = Math.floor(Math.min(last, total.value - 1) / ALBUM_PAGE_SIZE)
        for (let page = firstPage; page <= lastPage; page++) {
            if (loadedPages.has(page)) continue
            loadedPages.add(page)
            const offset = page * ALBUM_PAGE_SIZE
            const fid = unref(folderId)
            const rt = toValue(releaseType)
            try {
                const albums = await queryClient.fetchQuery({
                    queryKey: queryKeys.albumList('alphabeticalByName', ALBUM_PAGE_SIZE, offset, fid, rt),
                    queryFn: () =>
                        subsonicClient.getAlbumList('alphabeticalByName', ALBUM_PAGE_SIZE, offset, fid, rt),
                    staleTime: 2 * 60 * 1000
                })
                if (gen !== generation) return
                const next = items.value.slice()
                for (let i = 0; i < albums.length; i++) {
                    next[offset + i] = albums[i]
                }
                items.value = next
            } catch (e) {
                loadedPages.delete(page)
                throw e
            }
        }
    }

    return {
        total,
        letters,
        items,
        isLoading,
        error,
        ensureRange
    }
}
