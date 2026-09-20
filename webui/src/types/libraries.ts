export type LibraryDefaultView = 'albums' | 'artists'
export type LibraryFilterField = 'scan_folder' | 'path' | 'format' | 'release_type' | 'compilation' | 'genre'

/** One condition of a library. A library's filters are AND-ed; the values of one filter are OR-ed. */
export interface LibraryFilter {
    field: LibraryFilterField
    values: string[]
}

/** Something about a stored library that is not an error — today a scan_folder value whose folder is gone. */
export interface LibraryWarning {
    pointer: string
    detail: string
}

export interface Library {
    id: number
    name: string
    show_artists: boolean
    default_view: LibraryDefaultView
    icon: string
    filters: LibraryFilter[]
    warnings?: LibraryWarning[]
    created_at: string
    updated_at: string
    track_count: number
}

/** The write request. `filters` is always sent: the dialog round-trips what it shows. */
export interface LibraryInput {
    name: string
    show_artists: boolean
    default_view: LibraryDefaultView
    icon: string
    filters: LibraryFilter[]
}

export interface LibraryPreview {
    track_count: number
    album_count: number
}

export interface LibraryFilterOptions {
    scan_folders: string[]
    formats: string[]
    genres: string[]
    release_types: string[]
}

export interface ListLibrariesResponse {
    libraries: Library[]
}

export interface ApiError {
    error: string
    code: 'validation_error' | 'not_found' | 'conflict' | 'internal'
}

export interface BrowseFolder {
    name: string
    path: string
    has_subfolders: boolean
    is_symlink: boolean
}

export interface BrowseResponse {
    path: string
    folders: BrowseFolder[]
}
