import type { Album } from '@/types/subsonic'

/**
 * Sort albums newest-to-oldest by year (descending), with a stable name-based
 * tiebreak for albums with the same year. Missing year is treated as 0. Does not
 * mutate the input array.
 */
export function sortAlbumsNewestFirst(albums: Album[]): Album[] {
    return [...albums].sort((a, b) => {
        const yearA = a.year ?? 0
        const yearB = b.year ?? 0
        if (yearA !== yearB) return yearB - yearA
        // Stable tiebreak by name
        return a.name.localeCompare(b.name)
    })
}

/**
 * Pick one random album from the array. Returns null if the array is empty.
 */
export function pickRandomAlbum(albums: Album[]): Album | null {
    if (albums.length === 0) return null
    const index = Math.floor(Math.random() * albums.length)
    return albums[index]
}
