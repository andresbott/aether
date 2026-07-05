export interface MusicBrainzCandidate {
    mbid: string
    name: string
    type: string
    disambiguation: string
    lifeSpanBegin: string
    lifeSpanEnd: string
    score: number
}

export interface SetArtistMBIDResponse {
    mbArtistId: string
    imageFetched: boolean
    fetchError: string | null
}
