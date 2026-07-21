import { ref, unref, watch } from 'vue'
import type { Ref, ComputedRef } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { subsonicClient } from '@/lib/api/subsonic'
import { queryKeys } from '@/composables/useSubsonicQueries'
import type { Song } from '@/types/subsonic'

export const GENRE_SONG_PAGE_SIZE = 100

// Sparse lazily-paged song table for a genre, mirroring useAlbumTable. The
// total comes from the genre's songCount (getGenres) because getSongsByGenre
// has no total of its own.
export function useGenreSongsTable(
    genreName: Ref<string> | ComputedRef<string>,
    total: Ref<number> | ComputedRef<number>
) {
    const queryClient = useQueryClient()

    const items = ref<(Song | undefined)[]>([])
    let loadedPages = new Set<number>()

    // Reset the sparse array whenever the genre or its size changes.
    watch(
        [total, () => unref(genreName)],
        () => {
            items.value = new Array<Song | undefined>(unref(total))
            loadedPages = new Set<number>()
        },
        { immediate: true }
    )

    async function ensureRange(first: number, last: number): Promise<void> {
        if (unref(total) === 0) return
        const firstPage = Math.floor(Math.max(0, first) / GENRE_SONG_PAGE_SIZE)
        const lastPage = Math.floor(Math.min(last, unref(total) - 1) / GENRE_SONG_PAGE_SIZE)
        for (let page = firstPage; page <= lastPage; page++) {
            if (loadedPages.has(page)) continue
            loadedPages.add(page)
            const offset = page * GENRE_SONG_PAGE_SIZE
            const genre = unref(genreName)
            try {
                const songs = await queryClient.fetchQuery({
                    queryKey: queryKeys.genreSongs(genre, offset),
                    queryFn: () =>
                        subsonicClient.getSongsByGenre(genre, GENRE_SONG_PAGE_SIZE, offset),
                    staleTime: 2 * 60 * 1000
                })
                const next = items.value.slice()
                for (let i = 0; i < songs.length; i++) {
                    next[offset + i] = songs[i]
                }
                items.value = next
            } catch (e) {
                loadedPages.delete(page)
                throw e
            }
        }
    }

    return {
        items,
        ensureRange
    }
}
