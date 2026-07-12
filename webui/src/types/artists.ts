export interface MusicBrainzCandidate {
    mbid: string
    name: string
    type: string
    disambiguation: string
    lifeSpanBegin: string
    lifeSpanEnd: string
    score: number
}

export interface MusicBrainzReleaseCandidate {
    releaseMbid: string
    releaseGroupMbid: string
    title: string
    artist: string
    date: string
    country: string
    trackCount: number
    disambiguation: string
    score: number
}

export interface SetArtistMBIDResponse {
    mbArtistId: string
    imageFetched: boolean
    fetchError: string | null
}
