import type { Track } from '@/types/metadata'

// dirOf returns the library-relative parent directory of a track path
// ('' = library root).
export function dirOf(path: string): string {
    const i = path.lastIndexOf('/')
    return i === -1 ? '' : path.slice(0, i)
}

// selectionDirs lists the distinct directories a selection spans, sorted so
// callers get a stable primary directory and stable request order.
export function selectionDirs(tracks: Track[]): string[] {
    return [...new Set(tracks.map((t) => dirOf(t.path)))].sort()
}

function norm(s: string): string {
    return s.trim().toLowerCase()
}

// albumKey identifies the album a track belongs to from its TAGS, not its
// location: album name + album artists + MB release id, mirroring the server's
// album key (store.FindOrCreateAlbum). This is what makes a multi-disc release
// laid out as CD 1/, CD 2/ subfolders count as one album.
//
// Untagged files fall back to the directory: without an album name, grouping by
// tags would collapse every unknown album in the tree into one, so location is
// the safer identity there.
// selectionAlbumKey returns the key of the single album a selection covers, or
// null when it spans more than one. Two shapes count as one album:
//   - every track carries the same album identity (a multi-disc release spread
//     over CD 1/, CD 2/ subfolders)
//   - every track sits in the same directory, whatever its album tags say (a
//     compilation folder whose tracks each name a different album still has one
//     folder, hence one set of folder art)
export function selectionAlbumKey(tracks: Track[]): string | null {
    if (tracks.length === 0) return null
    const keys = new Set(tracks.map(albumKey))
    if (keys.size === 1) return [...keys][0]
    const dirs = selectionDirs(tracks)
    if (dirs.length === 1) return `dir\n${dirs[0]}`
    return null
}

export function albumKey(track: Track): string {
    const album = norm(track.album)
    if (album === '') return `dir\n${dirOf(track.path)}`
    return [album, track.album_artists.map(norm).join(''), norm(track.mb_release_id)].join('\n')
}
