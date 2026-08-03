import { computed, toValue, unref } from 'vue'
import type { MaybeRefOrGetter, Ref, ComputedRef } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import type { UseQueryOptions, UseMutationOptions } from '@tanstack/vue-query'
import { subsonicClient } from '@/lib/api/subsonic'
import { artistImageSourceKey } from '@/composables/useArtistImageSource'
import type {
    Album,
    AlbumWithSongs,
    Artist,
    Song,
    Playlist,
    SearchParams,
    SubsonicCredentials,
    InternetRadioStation
} from '@/types/subsonic'

// The subset of SearchParams that changes WHICH types a response carries, and so
// has to take part in the search cache key.
type SearchCounts = Pick<
    SearchParams,
    'artistCount' | 'albumCount' | 'songCount' | 'genreCount'
>

export const queryKeys = {
    ping: ['subsonic', 'ping'] as const,
    musicFolders: ['subsonic', 'musicFolders'] as const,
    albumList: (type: string, size: number, offset: number, musicFolderId?: number) =>
        ['subsonic', 'albumList', type, size, offset, musicFolderId] as const,
    album: (id: string) => ['subsonic', 'album', id] as const,
    artist: (id: string) => ['subsonic', 'artist', id] as const,
    // The requested per-type counts are part of the key, not just the term: a
    // scoped search asks for a SUBSET (zeroed counts), so keying on the term
    // alone would let a "songs only" response satisfy a later "everything"
    // request from cache and permanently hide the other types.
    search: (query: string, counts?: SearchCounts) =>
        [
            'subsonic',
            'search',
            query,
            counts?.artistCount ?? 0,
            counts?.albumCount ?? 0,
            counts?.songCount ?? 0,
            counts?.genreCount ?? 0
        ] as const,
    // Prefix of every per-query search cache entry, for invalidating them all.
    searchAll: ['subsonic', 'search'] as const,
    playlists: ['subsonic', 'playlists'] as const,
    playlist: (id: string) => ['subsonic', 'playlist', id] as const,
    genres: ['subsonic', 'genres'] as const,
    genreSongs: (genre: string, offset: number) =>
        ['subsonic', 'genreSongs', genre, offset] as const,
    radioStations: ['subsonic', 'radioStations'] as const,
    randomSongs: (size: number, musicFolderId?: number) =>
        ['subsonic', 'randomSongs', size, musicFolderId] as const,
    // One entry per library scope covers every starred type: getStarred2 returns
    // albums, artists, songs and playlists in a single unpaginated response.
    starred: (musicFolderId?: number) => ['subsonic', 'starred', musicFolderId] as const,
    discovery: (seed: number, musicFolderId?: number) =>
        ['subsonic', 'discovery', seed, musicFolderId] as const
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

// The id may be reactive (and empty) so callers that follow a changing album —
// e.g. the now-playing card — refetch as it changes and stay idle until there
// is one to load.
export function useAlbum(
    id: MaybeRefOrGetter<string | undefined>,
    options?: Omit<UseQueryOptions<AlbumWithSongs | null>, 'queryKey' | 'queryFn'>
) {
    const albumId = computed(() => toValue(id) ?? '')
    return useQuery({
        queryKey: computed(() => queryKeys.album(albumId.value)),
        queryFn: () => subsonicClient.getAlbum(albumId.value),
        staleTime: 5 * 60 * 1000,
        enabled: computed(() => albumId.value.length > 0),
        ...options
    })
}

export function useArtist(
    id: string,
    options?: Omit<UseQueryOptions<(Artist & { album?: Album[] }) | null>, 'queryKey' | 'queryFn'>
) {
    return useQuery({
        queryKey: queryKeys.artist(id),
        queryFn: () => subsonicClient.getArtist(id),
        staleTime: 5 * 60 * 1000,
        ...options
    })
}

// Shortest term that hits the server. Every match is a leading-substring scan
// (`%term%` over the *_norm columns), so one or two letters match most of the
// library and cost a query per keystroke for a result nobody can use.
export const MIN_SEARCH_LENGTH = 3

// Trimmed, because whitespace is not a search term: "  a " must not read as 4
// characters. The query key uses the same trimmed value so "rock" and "rock "
// share one cache entry instead of issuing two identical requests.
export function searchTermIsLongEnough(query: string): boolean {
    return query.trim().length >= MIN_SEARCH_LENGTH
}

export function useSearch(params: Ref<SearchParams> | ComputedRef<SearchParams>) {
    return useQuery({
        queryKey: computed(() => queryKeys.search(unref(params).query.trim(), unref(params))),
        queryFn: () => subsonicClient.search({ ...unref(params), query: unref(params).query.trim() }),
        enabled: computed(() => searchTermIsLongEnough(unref(params).query)),
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

export function useGenres() {
    return useQuery({
        queryKey: queryKeys.genres,
        queryFn: () => subsonicClient.getGenres(),
        staleTime: 5 * 60 * 1000
    })
}

export function useUpdateGenreCover() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (params: { genreId: string; coverFile?: File; coverClear?: boolean }) =>
            subsonicClient.updateGenreCover(params.genreId, params.coverFile, params.coverClear),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: queryKeys.genres })
        }
    })
}

export function useRadioStations() {
    return useQuery({
        queryKey: queryKeys.radioStations,
        queryFn: () => subsonicClient.getInternetRadioStations(),
        staleTime: 10 * 60 * 1000
    })
}

export function useSetCredentials(options?: UseMutationOptions<void, Error, SubsonicCredentials>) {
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

export function useUpdateArtistCover() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (params: { artistId: string; coverFile?: File; coverClear?: boolean }) =>
            subsonicClient.updateArtistCover(params.artistId, params.coverFile, params.coverClear),
        onSuccess: (_data, params) => {
            // Every cached surface that renders this artist's cover has to go:
            // the detail view, the library index (list/grid), and search results.
            // The image URL is unchanged, so a stale cache would keep showing the
            // old picture.
            queryClient.invalidateQueries({ queryKey: queryKeys.artist(params.artistId) })
            queryClient.invalidateQueries({ queryKey: ['subsonic', 'artistIndex'] })
            queryClient.invalidateQueries({ queryKey: queryKeys.searchAll })
            // An upload moves the image into aether's store and a clear can
            // uncover the music-folder file again — the recorded source is stale
            // either way (drives the "Current image" note and the Remove guard).
            queryClient.invalidateQueries({
                queryKey: artistImageSourceKey(params.artistId)
            })
        }
    })
}

export function useReplacePlaylistTracks() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (params: { playlistId: string; songIds: string[] }) =>
            subsonicClient.replacePlaylistTracks(params.playlistId, params.songIds),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: queryKeys.playlists })
            queryClient.invalidateQueries({ queryKey: ['subsonic', 'playlist'] })
        }
    })
}

export function useUpdatePlaylistCover() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (params: { playlistId: string; coverFile?: File; coverClear?: boolean }) =>
            subsonicClient.updatePlaylistCover(
                params.playlistId,
                params.coverFile,
                params.coverClear
            ),
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
            params.starred ? subsonicClient.unstar(params.id) : subsonicClient.star(params.id),
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
        queryKey: computed(() => queryKeys.randomSongs(unref(size), unref(musicFolderId))),
        queryFn: () => subsonicClient.getRandomSongs(unref(size), unref(musicFolderId)),
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

export function useTogglePlaylistStar() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (params: { id: string; starred: boolean }) =>
            params.starred ? subsonicClient.unstar(params.id) : subsonicClient.star(params.id),
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: queryKeys.playlists })
            queryClient.invalidateQueries({ queryKey: ['subsonic', 'playlist'] })
        }
    })
}
