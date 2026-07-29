import { computed, toValue } from 'vue'
import type { MaybeRefOrGetter } from 'vue'
import type { Playlist } from '@/types/subsonic'
import { useAlbumListByType, usePlaylists } from '@/composables/useSubsonicQueries'

export interface DiscoverySectionDef {
    key: string
    title: string
    // getAlbumList2 "type" parameter backing this section's album block.
    albumListType: string
    icon: string
}

// The single source of truth for what Discovery shows. Both DiscoveryView and
// DiscoverySectionView read the sections from here.
export const DISCOVERY_SECTIONS: readonly DiscoverySectionDef[] = [
    { key: 'recently-added', title: 'Recently added', albumListType: 'newest', icon: 'pi pi-clock' },
    { key: 'favorites', title: 'Favorites', albumListType: 'starred', icon: 'pi pi-star' },
    { key: 'most-played', title: 'Most played', albumListType: 'frequent', icon: 'pi pi-chart-bar' },
    {
        key: 'recently-played',
        title: 'Recently played',
        albumListType: 'recent',
        icon: 'pi pi-history'
    },
    { key: 'random', title: 'Random', albumListType: 'random', icon: 'pi pi-sparkles' }
] as const

export const SHELF_ALBUM_COUNT = 12
export const SHELF_PLAYLIST_COUNT = 6
export const SECTION_PAGE_ALBUM_COUNT = 100
// Random reshuffles per request, so its full page cannot be offset-paged — it
// fetches one larger batch and offers a refetch instead.
export const RANDOM_PAGE_ALBUM_COUNT = 200

export function findSection(key: string): DiscoverySectionDef | undefined {
    return DISCOVERY_SECTIONS.find((s) => s.key === key)
}

const byTimeDesc = (a?: string, b?: string): number =>
    new Date(b ?? 0).getTime() - new Date(a ?? 0).getTime()

// Pure: returns a new array and never mutates the caller's. Sections whose
// signal is optional (starred, playCount, played) drop the playlists that lack
// it rather than ranking them last — an unstarred playlist does not belong in
// Favorites at all.
export function sortPlaylistsForSection(playlists: Playlist[], key: string): Playlist[] {
    const all = [...playlists]
    switch (key) {
        case 'recently-added':
            return all.sort((a, b) => byTimeDesc(a.created, b.created))
        case 'favorites':
            return all.filter((p) => !!p.starred).sort((a, b) => byTimeDesc(a.starred, b.starred))
        case 'most-played':
            return all
                .filter((p) => (p.playCount ?? 0) > 0)
                .sort((a, b) => (b.playCount ?? 0) - (a.playCount ?? 0))
        case 'recently-played':
            return all.filter((p) => !!p.played).sort((a, b) => byTimeDesc(a.played, b.played))
        case 'random':
            return all.sort(() => Math.random() - 0.5)
        default:
            return []
    }
}

// One section's data: its album query plus the shared playlist query sorted for
// this section. Every section reuses the same cached getPlaylists result.
//
// `key` is reactive because vue-router reuses a route component when only a
// param changes — /discover/favorites -> /discover/random must reload, and
// setup() will not re-run.
export function useDiscoverySection(
    key: MaybeRefOrGetter<string>,
    albumSize: MaybeRefOrGetter<number>
) {
    const section = computed(() => findSection(toValue(key)))
    const albumQuery = useAlbumListByType(
        () => section.value?.albumListType ?? 'newest',
        albumSize
    )
    const playlistQuery = usePlaylists()

    return {
        section,
        albums: computed(() => albumQuery.data.value ?? []),
        playlists: computed(() =>
            sortPlaylistsForSection(playlistQuery.data.value ?? [], toValue(key))
        ),
        isLoading: computed(() => albumQuery.isLoading.value || playlistQuery.isLoading.value),
        isError: computed(() => albumQuery.isError.value || playlistQuery.isError.value),
        refetchAlbums: () => albumQuery.refetch()
    }
}
