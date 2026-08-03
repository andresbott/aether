import { computed } from 'vue'
import type { Ref, ComputedRef } from 'vue'
import { useAlbumTable } from '@/composables/useAlbumTable'
import { useArtistTable } from '@/composables/useArtistTable'
import { useStarredAlbums, useStarredArtists } from '@/composables/useStarred'
import type { Album, AlbumLetter, Artist } from '@/types/subsonic'

/**
 * Picks the album/artist data source for the library views: the full library
 * (`useAlbumTable` / `useArtistTable`) or just the favorites (`getStarred2`).
 *
 * Both sources are instantiated, each gated on `enabled` so only the active one
 * issues requests — composables cannot be called conditionally, and swapping the
 * source must not cost a request for the one nobody is looking at. Flipping the
 * toggle back finds the previous source's data still in the query cache.
 *
 * The returned shape is the union both sources already share, so the grid/list
 * components render either without a branch. `ensureRange` is the one asymmetry:
 * favorites arrive whole (getStarred2 is unpaginated), so it becomes a no-op and
 * the hosts' lazy-load hooks harmlessly call it.
 */
export function useAlbumSource(
    folderId: Ref<number | undefined> | ComputedRef<number | undefined>,
    favoritesOnly: Ref<boolean> | ComputedRef<boolean>
) {
    const table = useAlbumTable(folderId, {
        enabled: computed(() => !favoritesOnly.value)
    })
    const starred = useStarredAlbums(folderId, {
        enabled: computed(() => favoritesOnly.value)
    })

    const active = computed(() => (favoritesOnly.value ? starred : table))

    return {
        total: computed(() => active.value.total.value),
        letters: computed<AlbumLetter[]>(() => active.value.letters.value),
        items: computed<(Album | undefined)[]>(() => active.value.items.value),
        isLoading: computed(() => active.value.isLoading.value),
        error: computed(() => active.value.error.value),
        ensureRange: async (first: number, last: number): Promise<void> => {
            if (favoritesOnly.value) return
            await table.ensureRange(first, last)
        }
    }
}

export function useArtistSource(
    folderId: Ref<number | undefined> | ComputedRef<number | undefined>,
    favoritesOnly: Ref<boolean> | ComputedRef<boolean>
) {
    const table = useArtistTable(folderId, {
        enabled: computed(() => !favoritesOnly.value)
    })
    const starred = useStarredArtists(folderId, {
        enabled: computed(() => favoritesOnly.value)
    })

    const active = computed(() => (favoritesOnly.value ? starred : table))

    return {
        total: computed(() => active.value.total.value),
        letters: computed<AlbumLetter[]>(() => active.value.letters.value),
        items: computed<Artist[]>(() => active.value.items.value),
        isLoading: computed(() => active.value.isLoading.value),
        error: computed(() => active.value.error.value)
    }
}
