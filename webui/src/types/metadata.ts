export interface Folder {
    name: string
    path: string
    has_subfolders: boolean
}

export interface ListFoldersResponse {
    folders: Folder[]
}

export interface Track {
    path: string
    name: string
    title: string
    artists: string[]
    album_artists: string[]
    album: string
    year: number
    compilation: boolean
    mb_artist_ids: string[]
    mb_album_artist_ids: string[]
    error?: string
}

export interface ListTracksResponse {
    tracks: Track[]
}

// Only fields with a defined value (not undefined) are sent over the wire;
// JSON.stringify omits undefined keys, which is exactly the "apply" semantics
// the server expects.
export interface PatchFields {
    title?: string
    album?: string
    artists?: string[]
    album_artists?: string[]
    year?: number
    compilation?: boolean
    artist_mbids?: Record<string, string>
    album_artist_mbids?: Record<string, string>
}

export interface UpdateTracksRequest {
    library_id: number
    paths: string[]
    fields: PatchFields
}

export interface UpdateResult {
    path: string
    ok: boolean
    error?: string
}

export interface UpdateTracksResponse {
    results: UpdateResult[]
}
