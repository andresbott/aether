/**
 * A directory the server scans for music. Scan folders are declared in the
 * server's config file and are read-only here: there is nothing to create, edit
 * or delete. `name` is the identifier every metadata-editor request addresses.
 */
export interface ScanFolder {
    name: string
    path: string
    exclude_patterns: string[]
    follow_symlinks: boolean
    /** false when the root cannot be scanned right now; `problem` says why. */
    available: boolean
    problem?: string
    /** tracks indexed under this folder */
    track_count: number
}

export interface ListScanFoldersResponse {
    scan_folders: ScanFolder[]
}
