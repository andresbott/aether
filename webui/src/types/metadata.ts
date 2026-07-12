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
    disc_number: number
    disc_subtitle: string
    compilation: boolean
    mb_artist_ids: string[]
    mb_album_artist_ids: string[]
    mb_release_id: string
    mb_release_group_id: string
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
    disc_number?: number
    disc_subtitle?: string
    compilation?: boolean
    artist_mbids?: Record<string, string>
    album_artist_mbids?: Record<string, string>
    mb_release_id?: string
    mb_release_group_id?: string
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

// A cover candidate returned by the Cover Art Archive lookup.
export interface CoverCandidate {
    id: string
    thumbUrl: string
    imageUrl: string
    isFront: boolean
    // What the image depicts, e.g. ['Front'], ['Back'], ['Booklet'], ['Medium'].
    types: string[]
    // The uploader's free-text note, if any.
    comment: string
}

// Where to save an album cover:
// - 'db': aether's managed store (music files untouched)
// - 'folder': cover.jpg/png in the album folder
// - 'embedded': embedded in the id3 of the selected tracks
export type CoverTarget = 'db' | 'folder' | 'embedded'

export interface ApplyCoverResult {
    ok: boolean
    target: CoverTarget
}

// A cover chosen in the picker but not yet persisted: it previews in the editor
// and is only written when the user clicks Save. Exactly one of file / imageUrl
// is set (a local upload vs. a Cover Art Archive URL).
export interface StagedCover {
    target: CoverTarget
    file: File | null
    imageUrl: string | null
}

// One place a cover was found for a folder (db / folder / embedded).
export interface CoverSourceEntry {
    source: CoverTarget
    detail?: string
}

export interface CoverInfoResponse {
    sources: CoverSourceEntry[]
}
