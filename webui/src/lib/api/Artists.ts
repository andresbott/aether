import { apiClient } from '@/lib/api/client'
import type {
    ArtistImageSource,
    MusicBrainzCandidate,
    MusicBrainzReleaseCandidate,
    SetArtistMBIDResponse
} from '@/types/artists'

export function parseArtistNumericId(subsonicId: string): number {
    const n = Number(subsonicId.split('-').pop())
    if (!Number.isFinite(n)) {
        throw new Error(`invalid artist id: ${subsonicId}`)
    }
    return n
}

export async function searchMusicBrainzArtists(query: string): Promise<MusicBrainzCandidate[]> {
    const { data } = await apiClient.get<MusicBrainzCandidate[]>('/musicbrainz/search', {
        params: { q: query }
    })
    return data
}

export async function searchMusicBrainzReleases(
    query: string
): Promise<MusicBrainzReleaseCandidate[]> {
    const { data } = await apiClient.get<MusicBrainzReleaseCandidate[]>(
        '/musicbrainz/search/releases',
        { params: { q: query } }
    )
    return data
}

// getReleaseGroupGenres looks up the genres of a MusicBrainz release group,
// ordered by vote count descending.
export async function getReleaseGroupGenres(mbid: string): Promise<string[]> {
    const { data } = await apiClient.get<string[]>(
        `/musicbrainz/release-groups/${mbid}/genres`
    )
    return data
}

export async function getArtistMBID(numericId: number): Promise<string> {
    const { data } = await apiClient.get<{ mbArtistId: string }>(`/artists/${numericId}/mbid`)
    return data.mbArtistId
}

// getArtistImageSource reports where the image getCoverArt serves for this
// artist comes from, so the editor can tell the user it is read from disk.
export async function getArtistImageSource(numericId: number): Promise<ArtistImageSource> {
    const { data } = await apiClient.get<ArtistImageSource>(`/artists/${numericId}/image-source`)
    return data
}

export async function setArtistMBID(numericId: number, mbid: string): Promise<SetArtistMBIDResponse> {
    const { data } = await apiClient.put<SetArtistMBIDResponse>(`/artists/${numericId}/mbid`, { mbid })
    return data
}
