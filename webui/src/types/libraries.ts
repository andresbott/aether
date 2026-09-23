/** One way of browsing a library; labels, icons and display order live in lib/libraryViews.ts. */
export type LibraryView = 'discover' | 'artists' | 'releases'
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
    /** The ways the library can be browsed, in display order. */
    views: LibraryView[]
    /** The view it opens on — one of `views`. */
    default_view: LibraryView
    /** Keeps its artists off the main Artists page; its own Artists view still lists them. */
    hide_from_artist_index: boolean
    /** A sidebar section of its own, one entry per view, instead of a single entry. */
    split_views: boolean
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
    views: LibraryView[]
    default_view: LibraryView
    hide_from_artist_index: boolean
    split_views: boolean
    icon: string
    filters: LibraryFilter[]
}

/** The root library's settings — the whole catalog, browsed without a library. */
export interface CatalogSettings {
    /** Each root view its own sidebar entry, instead of one entry whose page switches them. */
    split_views: boolean
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
