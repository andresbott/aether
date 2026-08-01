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
    songCount?: number
    starred?: string
    coverArt?: string
    artistImageUrl?: string
}

// One disc of a multi-disc album that carries a subtitle (OpenSubsonic
// AlbumID3.discTitles). Discs without a subtitle are not listed.
export interface DiscTitle {
    disc: number
    title: string
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
    discTitles?: DiscTitle[]
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
    // Aether's "playlistStar" extension: RFC3339, absent when not starred.
    starred?: string
    // Aether's "playlistStats" extension.
    playCount?: number
    played?: string
}

// Aether's "discovery" extension. The server owns the cross-type ranking and
// reports it as an absolute `rank` on each entity, so the client merges the two
// arrays with a sort rather than reimplementing the formula.
export type DiscoveryReason =
    | 'favorite'
    | 'recentlyAdded'
    | 'mostPlayed'
    | 'recentlyPlayed'
    | 'genreMatch'
    | 'rediscover'

export type DiscoveryAlbum = Album & { rank: number; reason: DiscoveryReason }
export type DiscoveryPlaylist = Playlist & { rank: number; reason: DiscoveryReason }

export interface DiscoveryPage {
    album: DiscoveryAlbum[]
    playlist: DiscoveryPlaylist[]
}

// One flattened feed entry. The discriminated union keeps the rendering
// dispatcher type-safe without casts.
export type DiscoveryFeedEntry =
    | { type: 'album'; rank: number; reason: DiscoveryReason; album: DiscoveryAlbum }
    | { type: 'playlist'; rank: number; reason: DiscoveryReason; playlist: DiscoveryPlaylist }

export interface Genre {
    value: string
    songCount: number
    albumCount: number
    // Aether's "genreCoverArt" OpenSubsonic extension.
    coverArt?: string
}

export interface MusicFolder {
    id: number
    name: string
    defaultView?: 'albums' | 'artists'
    showArtists?: boolean
    // Aether's "musicFolderIcon" OpenSubsonic extension: PrimeIcons name without the "pi pi-" prefix.
    icon?: string
}

export interface SearchResult3 {
    artist?: Artist[]
    album?: Album[]
    song?: Song[]
}

export interface AlbumList {
    album: Album[]
}

export interface AlbumLetter {
    name: string // "#" or "A".."Z"
    offset: number
    count: number
}

export interface AlbumIndex {
    total: number
    index: AlbumLetter[]
}

export interface ArtistIndex {
    total: number
    letters: AlbumLetter[]
    items: Artist[]
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
    repeat: 'none' | 'all'
    shuffle: boolean
    currentTime: number
    duration: number
}

export interface QueueItem extends Song {
    queueId: string
}

// The saved cross-device playback session (getPlayQueueByIndex, the
// "indexBasedQueue" extension). currentIndex is a queue slot rather than a track
// id, because a queue may hold the same track more than once. position is the
// offset in ms within that slot's track — the field that resumes mid-song.
export interface SavedPlayQueue {
    entry: Song[]
    currentIndex: number
    position: number
    changedBy?: string
    changed?: string
}

export interface InternetRadioStation {
    id: string
    name: string
    streamUrl: string
    homepageUrl?: string
    coverArt?: string
}
