import { useMutation, useQueryClient } from '@tanstack/vue-query'
import { parseArtistNumericId, setArtistImageFromSearch } from '@/lib/api/Artists'
import { artistImageSourceKey } from '@/composables/useArtistImageSource'
import { queryKeys } from '@/composables/useSubsonicQueries'

// useSetArtistImageFromSearch commits a pick from the online image search: the
// server fetches that MusicBrainz artist's provider image and stores it as a
// manual upload. Invalidates the same caches as a cover upload — every surface
// that renders this artist's cover is now stale.
export function useSetArtistImageFromSearch() {
    const queryClient = useQueryClient()
    return useMutation({
        mutationFn: (params: { artistId: string; mbid: string; url: string }) =>
            setArtistImageFromSearch(parseArtistNumericId(params.artistId), params.mbid, params.url),
        onSuccess: (_data, params) => {
            queryClient.invalidateQueries({ queryKey: queryKeys.artist(params.artistId) })
            queryClient.invalidateQueries({ queryKey: ['subsonic', 'artistIndex'] })
            queryClient.invalidateQueries({ queryKey: queryKeys.searchAll })
            queryClient.invalidateQueries({ queryKey: artistImageSourceKey(params.artistId) })
        }
    })
}
