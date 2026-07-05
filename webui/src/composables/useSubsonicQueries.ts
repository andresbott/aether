import { computed, unref } from 'vue'
import type { Ref, ComputedRef } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import type { UseQueryOptions, UseMutationOptions } from '@tanstack/vue-query'
import { subsonicClient } from '@/lib/api/subsonic'
import type {
    Album,
    AlbumWithSongs,
    Artist,
    Song,
    Playlist,
    SearchParams,
    SubsonicCredentials,
    PodcastChannel,
    InternetRadioStation
} from '@/types/subsonic'

export const queryKeys = {
    ping: ['subsonic', 'ping'] as const,
    musicFolders: ['subsonic', 'musicFolders'] as const,
    albumList: (type: string, offset: number, musicFolderId?: number) =>
        ['subsonic', 'albumList', type, offset, musicFolderId] as const,
    album: (id: string) => ['subsonic', 'album', id] as const,
    artist: (id: string) => ['subsonic', 'artist', id] as const,
    search: (query: string) => ['subsonic', 'search', query] as const,
    playlists: ['subsonic', 'playlists'] as const,
    playlist: (id: string) => ['subsonic', 'playlist', id] as const,
    podcasts: (includeEpisodes: boolean) =>
        ['subsonic', 'podcasts', includeEpisodes] as const,
    podcastChannel: (id: string) => ['subsonic', 'podcastChannel', id] as const,
    newestPodcasts: (count: number) => ['subsonic', 'newestPodcasts', count] as const,
    radioStations: ['subsonic', 'radioStations'] as const,
    randomSongs: (size: number, musicFolderId?: number) =>
        ['subsonic', 'randomSongs', size, musicFolderId] as const
}

export function usePing() {
    return useQuery({
        queryKey: queryKeys.ping,
        queryFn: () => subsonicClient.ping(),
        staleTime: 5 * 60 * 1000,
        retry: 1
    })
}

export function useMusicFolders() {
    return useQuery({
        queryKey: queryKeys.musicFolders,
        queryFn: () => subsonicClient.getMusicFolders(),
        staleTime: 10 * 60 * 1000
    })
}

export function useAlbum(
    id: string,
    options?: Omit<UseQueryOptions<AlbumWithSongs | null>, 'queryKey' | 'queryFn'>
) {
    return useQuery({
        queryKey: queryKeys.album(id),
        queryFn: () => subsonicClient.getAlbum(id),
        staleTime: 5 * 60 * 1000,
        ...options
    })
}

export function useArtist(
    id: string,
    options?: Omit<
        UseQueryOptions<(Artist & { album?: Album[] }) | null>,
        'queryKey' | 'queryFn'
    >
) {
    return useQuery({
        queryKey: queryKeys.artist(id),
        queryFn: () => subsonicClient.getArtist(id),
        staleTime: 5 * 60 * 1000,
        ...options
    })
}

export function useSearch(params: Ref<SearchParams> | ComputedRef<SearchParams>) {
    return useQuery({
        queryKey: computed(() => queryKeys.search(unref(params).query)),
        queryFn: () => subsonicClient.search(unref(params)),
        enabled: computed(() => unref(params).query.length > 0),
        staleTime: 2 * 60 * 1000
    })
}

export function usePlaylists() {
    return useQuery({
        queryKey: queryKeys.playlists,
        queryFn: () => subsonicClient.getPlaylists(),
        staleTime: 5 * 60 * 1000
    })
}

export function usePlaylist(id: string) {
    return useQuery({
        queryKey: queryKeys.playlist(id),
        queryFn: () => subsonicClient.getPlaylist(id),
        staleTime: 5 * 60 * 1000
    })
}

export function usePodcasts(
    includeEpisodes: boolean | Ref<boolean> | ComputedRef<boolean> = true
) {
    return useQuery({
        queryKey: computed(() => queryKeys.podcasts(unref(includeEpisodes))),
        queryFn: () => subsonicClient.getPodcasts(unref(includeEpisodes)),
        staleTime: 5 * 60 * 1000
    })
}

export function usePodcastChannel(
    id: string,
    options?: Omit<UseQueryOptions<PodcastChannel | null>, 'queryKey' | 'queryFn'>
) {
    return useQuery({
        queryKey: queryKeys.podcastChannel(id),
        queryFn: () => subsonicClient.getPodcastChannel(id),
        staleTime: 5 * 60 * 1000,
        ...options
    })
}

export function useNewestPodcasts(
    count: number | Ref<number> | ComputedRef<number> = 20
) {
    return useQuery({
        queryKey: computed(() => queryKeys.newestPodcasts(unref(count))),
        queryFn: () => subsonicClient.getNewestPodcasts(unref(count)),
        staleTime: 2 * 60 * 1000
    })
}

export function useRadioStations() {
    return useQuery({
        queryKey: queryKeys.radioStations,
        queryFn: () => subsonicClient.getInternetRadioStations(),
        staleTime: 10 * 60 * 1000
    })
}

export function useSetCredentials(
    options?: UseMutationOptions<void, Error, SubsonicCredentials>
) {
    const queryClient = useQueryClient()

    return useMutation({
        mutationFn: async (credentials: SubsonicCredentials) => {
            subsonicClient.setCredentials(credentials)
        },
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['subsonic'] })
        },
        ...options
    })
}

export function useCreatePlaylist() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (params: { name: string; songIds?: string[] }) =>
            subsonicClient.createPlaylist(params.name, params.songIds),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: queryKeys.playlists })
        }
    })
}

export function useUpdatePlaylist() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (params: {
            playlistId: string
            name?: string
            comment?: string
            songIdsToAdd?: string[]
            songIndexesToRemove?: number[]
        }) => subsonicClient.updatePlaylist(params.playlistId, params),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: queryKeys.playlists })
            queryClient.invalidateQueries({ queryKey: ['subsonic', 'playlist'] })
        }
    })
}

export function useDeletePlaylist() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (id: string) => subsonicClient.deletePlaylist(id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: queryKeys.playlists })
        }
    })
}

export function useToggleStar() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (params: { id: string; starred: boolean }) =>
            params.starred
                ? subsonicClient.unstar(params.id)
                : subsonicClient.star(params.id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ['subsonic'] })
        }
    })
}

export function useRandomSongs(
    size: number | Ref<number> | ComputedRef<number> = 50,
    musicFolderId?: number | Ref<number | undefined> | ComputedRef<number | undefined>
) {
    return useQuery({
        queryKey: computed(() =>
            queryKeys.randomSongs(unref(size), unref(musicFolderId))
        ),
        queryFn: () =>
            subsonicClient.getRandomSongs(unref(size), unref(musicFolderId)),
        staleTime: 2 * 60 * 1000
    })
}

export function useCreateRadioStation() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (params: {
            name: string
            streamUrl: string
            homepageUrl?: string
            coverFile?: File
        }) =>
            subsonicClient.createInternetRadioStation(
                params.name,
                params.streamUrl,
                params.homepageUrl,
                params.coverFile
            ),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: queryKeys.radioStations })
        }
    })
}

export function useUpdateRadioStation() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (params: {
            id: string
            name: string
            streamUrl: string
            homepageUrl?: string
            coverFile?: File
            coverClear?: boolean
        }) =>
            subsonicClient.updateInternetRadioStation(
                params.id,
                params.name,
                params.streamUrl,
                params.homepageUrl,
                params.coverFile,
                params.coverClear
            ),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: queryKeys.radioStations })
        }
    })
}

export function useDeleteRadioStation() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (id: string) => subsonicClient.deleteInternetRadioStation(id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: queryKeys.radioStations })
        }
    })
}
