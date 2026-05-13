export type LibraryDefaultView = 'albums' | 'artists'

export interface Library {
    id: number
    name: string
    path: string
    exclude_patterns: string[]
    follow_symlinks: boolean
    multi_value_genre: string
    multi_value_artist: string
    multi_value_album_artist: string
    default_view: LibraryDefaultView
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
    multi_value_genre: string
    multi_value_artist: string
    multi_value_album_artist: string
    default_view: LibraryDefaultView
}

export interface ListLibrariesResponse {
    libraries: Library[]
}

export interface ApiError {
    error: string
    code: 'validation_error' | 'not_found' | 'conflict' | 'internal'
}
