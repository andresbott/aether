export type LibraryDefaultView = 'albums' | 'artists'

export type LibraryCoverStyle =
    | 'auto'
    | 'classic'
    | 'bauhaus'
    | 'rings'
    | 'waves'
    | 'poster'
    | 'remix'

/**
 * Who owns a library's configuration: 'db' for one managed here, 'config' for
 * one declared in the server's config file. Config libraries are read-only —
 * the server rewrites them from the file on every startup.
 */
export type LibrarySource = 'db' | 'config'

export interface Library {
    id: number
    name: string
    path: string
    exclude_patterns: string[]
    follow_symlinks: boolean
    show_artists: boolean
    default_view: LibraryDefaultView
    icon: string
    cover_style: LibraryCoverStyle
    source: LibrarySource
    last_scan_started_at: string | null
    created_at: string
    updated_at: string
    track_count: number
    path_changed?: boolean
}

export interface LibraryInput {
    name: string
    path: string
    exclude_patterns: string[]
    follow_symlinks: boolean
    show_artists: boolean
    default_view: LibraryDefaultView
    icon: string
    cover_style: LibraryCoverStyle
}

export interface ListLibrariesResponse {
    libraries: Library[]
}

export interface ApiError {
    error: string
    code: 'validation_error' | 'not_found' | 'conflict' | 'config_managed' | 'internal'
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
