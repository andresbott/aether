export interface MusicBrainzCandidate {
    mbid: string
    name: string
    type: string
    disambiguation: string
    lifeSpanBegin: string
    lifeSpanEnd: string
    score: number
}

// One credited artist on a release: the credited-as name and the artist's MBID.
export interface ReleaseArtistCredit {
    name: string
    mbid: string
}

export interface MusicBrainzReleaseCandidate {
    releaseMbid: string
    releaseGroupMbid: string
    title: string
    artist: string
    artists: ReleaseArtistCredit[]
    date: string
    country: string
    trackCount: number
    disambiguation: string
    score: number
}

// What the artist picker emits on confirm: only the fields the user left
// checked in the preview are present. An empty-string mbid clears the match.
export interface ArtistMatchPayload {
    name?: string
    mbid?: string
}

// What the album picker emits on confirm: only the fields the user left
// checked in the preview are present. Empty-string IDs clear the match.
export interface AlbumMatchPayload {
    album?: string
    year?: number
    mbReleaseId?: string
    mbReleaseGroupId?: string
    albumArtists?: ReleaseArtistCredit[]
    genres?: string[]
}

// Where the image the server currently serves for an artist comes from:
// "upload" = the user uploaded it, "fetched" = aether auto-fetched it, "folder" =
// a file read from disk next to the artist's albums (path set), "none" = the
// generated avatar. `filename` names the image file in every case but "none".
export interface ArtistImageSource {
    source: 'upload' | 'fetched' | 'folder' | 'none'
    path: string
    filename: string
}

// A staged pick from the online image search: which MusicBrainz artist's provider
// image to use, plus the preview URL so the editor can show it before saving.
export interface ArtistImagePick {
    mbid: string
    name: string
    previewUrl: string
}

export interface SetArtistMBIDResponse {
    mbArtistId: string
    imageFetched: boolean
    fetchError: string | null
}
