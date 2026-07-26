import { useQuery } from '@tanstack/vue-query'
import { getArtistImageSource, parseArtistNumericId } from '@/lib/api/Artists'
import type { ArtistImageSource } from '@/types/artists'

// useArtistImageSource fetches where the artist's current image comes from. The
// artist editor uses it to note an image that is read from the music folder
// rather than held in aether's own store. Only enabled while the editor needs
// it, and never retried — a missing note is not worth a retry storm.
export function useArtistImageSource(subsonicId: string, enabled: () => boolean) {
    return useQuery<ArtistImageSource>({
        queryKey: ['artistImageSource', subsonicId],
        queryFn: () => getArtistImageSource(parseArtistNumericId(subsonicId)),
        enabled,
        retry: false,
        staleTime: 5 * 60 * 1000
    })
}
