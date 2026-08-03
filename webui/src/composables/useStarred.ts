import { computed, unref } from 'vue'
import type { Ref, ComputedRef, MaybeRefOrGetter } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import { subsonicClient } from '@/lib/api/subsonic'
import { queryKeys } from '@/composables/useSubsonicQueries'
import { deriveLetterIndex } from '@/utils/letterIndex'
import type { Album, AlbumLetter, Artist } from '@/types/subsonic'

/**
 * The favorites (getStarred2) counterparts of `useAlbumTable` / `useArtistTable`,
 * returning the SAME `{ total, letters, items, isLoading, error }` shape so
 * `AlbumGrid`/`AlbumListView`/`ArtistGrid`/`ArtistListView` can render either
 * source without knowing which one they got.
 *
 * Two differences from the full-library composables, both inherent to the
 * endpoint rather than choices made here:
 *
 * - **No paging.** `getStarred2` is unpaginated by spec, so the whole set arrives
 *   at once and there is no `ensureRange` — the hosts' lazy-load hooks become
 *   no-ops. Favorites are a hand-curated subset; there is no size to page.
 * - **Letters are derived client-side** (`deriveLetterIndex`) rather than fetched:
 *   the server has no per-letter index for a starred subset. It does order both
 *   arrays by `name_norm ASC`, which is what makes the derivation valid.
 *
 * One query backs both composables (same key), so a view reading albums and the
 * header reading a count cost one request between them.
 */
const STARRED_STALE_TIME = 2 * 60 * 1000

function useStarredQuery(
    folderId: Ref<number | undefined> | ComputedRef<number | undefined>,
    options?: { enabled?: MaybeRefOrGetter<boolean> }
) {
    return useQuery({
        queryKey: computed(() => queryKeys.starred(unref(folderId))),
        queryFn: () => subsonicClient.getStarred(unref(folderId)),
        staleTime: STARRED_STALE_TIME,
        enabled: options?.enabled
    })
}

export function useStarredAlbums(
    folderId: Ref<number | undefined> | ComputedRef<number | undefined>,
    options?: { enabled?: MaybeRefOrGetter<boolean> }
) {
    const query = useStarredQuery(folderId, options)

    const items = computed<Album[]>(() => query.data.value?.album ?? [])
    const total = computed(() => items.value.length)
    const letters = computed<AlbumLetter[]>(() =>
        deriveLetterIndex(items.value.map((a) => a.name))
    )

    return { total, letters, items, isLoading: query.isLoading, error: query.error }
}

export function useStarredArtists(
    folderId: Ref<number | undefined> | ComputedRef<number | undefined>,
    options?: { enabled?: MaybeRefOrGetter<boolean> }
) {
    const query = useStarredQuery(folderId, options)

    const items = computed<Artist[]>(() => query.data.value?.artist ?? [])
    const total = computed(() => items.value.length)
    const letters = computed<AlbumLetter[]>(() =>
        deriveLetterIndex(items.value.map((a) => a.name))
    )

    return { total, letters, items, isLoading: query.isLoading, error: query.error }
}
