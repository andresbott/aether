export interface SubsonicResponse<T = unknown> {
    'subsonic-response': {
        status: 'ok' | 'failed'
        version: string
        type?: string
        serverVersion?: string
        openSubsonic?: boolean
        error?: SubsonicError
    } & T
}

export interface SubsonicError {
    code: number
    message: string
}

export interface Artist {
    id: string
    name: string
    albumCount?: number
    starred?: string
    coverArt?: string
    artistImageUrl?: string
}

export interface Album {
    id: string
    name: string
    artist?: string
    artistId?: string
    coverArt?: string
    songCount?: number
    duration?: number
    created?: string
    year?: number
    genre?: string
    starred?: string
}

export interface AlbumWithSongs extends Album {
    song?: Song[]
}

export interface Song {
    id: string
    title: string
    album?: string
    albumId?: string
    artist?: string
    artistId?: string
    coverArt?: string
    duration?: number
    bitRate?: number
    track?: number
    discNumber?: number
    year?: number
    genre?: string
    size?: number
    contentType?: string
    suffix?: string
    path?: string
    starred?: string
    isVideo?: boolean
    streamUrl?: string
}

export interface Playlist {
    id: string
    name: string
    comment?: string
    owner?: string
    public?: boolean
    songCount: number
    duration: number
    created: string
    changed?: string
    coverArt?: string
}

export interface MusicFolder {
    id: number
    name: string
    defaultView?: 'albums' | 'artists'
}

export interface SearchResult3 {
    artist?: Artist[]
    album?: Album[]
    song?: Song[]
}

export interface AlbumList {
    album: Album[]
}

export interface ArtistInfo {
    biography?: string
    musicBrainzId?: string
    lastFmUrl?: string
    smallImageUrl?: string
    mediumImageUrl?: string
    largeImageUrl?: string
    similarArtist?: Artist[]
}

export interface SubsonicCredentials {
    username: string
    password: string
    salt?: string
    token?: string
    serverUrl: string
}

export interface SearchParams {
    query: string
    artistCount?: number
    artistOffset?: number
    albumCount?: number
    albumOffset?: number
    songCount?: number
    songOffset?: number
}

export interface PlayerState {
    currentTrack: Song | null
    queue: Song[]
    currentIndex: number
    isPlaying: boolean
    volume: number
    repeat: 'none' | 'all' | 'one'
    shuffle: boolean
    currentTime: number
    duration: number
}

export interface QueueItem extends Song {
    queueId: string
}

export interface PodcastChannel {
    id: string
    url: string
    title: string
    description?: string
    coverArt?: string
    originalImageUrl?: string
    status: 'new' | 'downloading' | 'completed' | 'error' | 'deleted' | 'skipped'
    errorMessage?: string
    episode?: PodcastEpisode[]
}

export interface PodcastEpisode {
    id: string
    streamId: string
    channelId: string
    title: string
    description?: string
    publishDate?: string
    status: 'new' | 'downloading' | 'completed' | 'error' | 'deleted' | 'skipped'
    duration?: number
    bitRate?: number
    size?: number
    contentType?: string
    suffix?: string
    coverArt?: string
    year?: number
    genre?: string
}

export interface InternetRadioStation {
    id: string
    name: string
    streamUrl: string
    homepageUrl?: string
    coverArt?: string
}
