import { apiClient } from '@/lib/api/client'
import type { MusicBrainzCandidate, SetArtistMBIDResponse } from '@/types/artists'

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

export async function getArtistMBID(numericId: number): Promise<string> {
    const { data } = await apiClient.get<{ mbArtistId: string }>(`/artists/${numericId}/mbid`)
    return data.mbArtistId
}

export async function setArtistMBID(numericId: number, mbid: string): Promise<SetArtistMBIDResponse> {
    const { data } = await apiClient.put<SetArtistMBIDResponse>(`/artists/${numericId}/mbid`, { mbid })
    return data
}
