import { computed, toValue } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import { useInfiniteQuery } from '@tanstack/vue-query'
import { subsonicClient } from '@/lib/api/subsonic'
import { queryKeys } from '@/composables/useSubsonicQueries'
import type { SearchResult3, Song } from '@/types/subsonic'

export const SONG_LIST_PAGE_SIZE = 50

// Flattens the pages from the infinite query into a single song array.
function flattenSongPages(pages: SearchResult3[]): Song[] {
    const songs: Song[] = []
    for (const page of pages) {
        if (page.song) {
            songs.push(...page.song)
        }
    }
    return songs
}

// Terminal condition for the infinite song list: when a page comes back with
// fewer songs than requested, we've reached the end. Returns the next offset,
// or undefined to stop.
export function nextSongOffset(
    lastPage: SearchResult3,
    allPages: SearchResult3[]
): number | undefined {
    const got = lastPage.song?.length ?? 0
    if (got < SONG_LIST_PAGE_SIZE) return undefined
    return allPages.length * SONG_LIST_PAGE_SIZE
}

// Infinite song list using search3 with an empty query. Library-scoped when
// folderId is defined, cross-collection when undefined. The favoritesOnly
// filter is not implemented here (search3 has no starred filter param), so
// callers must respect that the list is unfiltered.
export function useSongList(
    folderId: MaybeRefOrGetter<number | undefined>,
    favoritesOnly: MaybeRefOrGetter<boolean>,
    enabled?: MaybeRefOrGetter<boolean>
) {
    const query = useInfiniteQuery({
        queryKey: computed(() =>
            queryKeys.songs(toValue(folderId), toValue(favoritesOnly))
        ),
        queryFn: ({ pageParam }) =>
            subsonicClient.search3(
                '',
                SONG_LIST_PAGE_SIZE,
                pageParam as number,
                toValue(folderId)
            ),
        initialPageParam: 0,
        getNextPageParam: nextSongOffset,
        staleTime: 2 * 60 * 1000,
        enabled: enabled !== undefined ? computed(() => toValue(enabled)) : undefined
    })

    return {
        items: computed(() => flattenSongPages(query.data.value?.pages ?? [])),
        isLoading: computed(() => query.isLoading.value),
        isError: computed(() => query.isError.value),
        hasNextPage: computed(() => !!query.hasNextPage.value),
        isFetchingNextPage: computed(() => query.isFetchingNextPage.value),
        fetchNextPage: () => {
            if (query.hasNextPage.value && !query.isFetchingNextPage.value) {
                void query.fetchNextPage()
            }
        }
    }
}
