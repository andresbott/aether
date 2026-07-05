import { ref, unref, watch } from 'vue'
import type { Ref, ComputedRef } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { subsonicClient } from '@/lib/api/subsonic'
import { queryKeys } from '@/composables/useSubsonicQueries'
import type { Album } from '@/types/subsonic'
import { useAlbumIndex } from '@/composables/useAlbumIndex'

export const ALBUM_PAGE_SIZE = 100

export function useAlbumTable(
    folderId: Ref<number | undefined> | ComputedRef<number | undefined>
) {
    const queryClient = useQueryClient()

    const { total, letters, isLoading, error } = useAlbumIndex(folderId)

    const items = ref<(Album | undefined)[]>([])
    let loadedPages = new Set<number>()

    // Reset the sparse array whenever the library or its size changes.
    watch(
        [total, () => unref(folderId)],
        () => {
            items.value = new Array<Album | undefined>(total.value)
            loadedPages = new Set<number>()
        },
        { immediate: true }
    )

    async function ensureRange(first: number, last: number): Promise<void> {
        if (total.value === 0) return
        const firstPage = Math.floor(Math.max(0, first) / ALBUM_PAGE_SIZE)
        const lastPage = Math.floor(Math.min(last, total.value - 1) / ALBUM_PAGE_SIZE)
        for (let page = firstPage; page <= lastPage; page++) {
            if (loadedPages.has(page)) continue
            loadedPages.add(page)
            const offset = page * ALBUM_PAGE_SIZE
            const fid = unref(folderId)
            try {
                const albums = await queryClient.fetchQuery({
                    queryKey: queryKeys.albumList('alphabeticalByName', offset, fid),
                    queryFn: () =>
                        subsonicClient.getAlbumList('alphabeticalByName', ALBUM_PAGE_SIZE, offset, fid),
                    staleTime: 2 * 60 * 1000
                })
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
