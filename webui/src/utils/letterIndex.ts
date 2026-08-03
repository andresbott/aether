import type { AlbumLetter } from '@/types/subsonic'

/**
 * First-letter bucket for one name: "A".."Z", or "#" for anything that does not
 * start with a latin letter. Diacritics are folded (Björk → B), matching the
 * server, which buckets on the unidecoded `name_norm` column.
 */
export function firstLetter(name: string): string {
    for (const ch of name.normalize('NFD').replace(/\p{Diacritic}/gu, '')) {
        const upper = ch.toUpperCase()
        if (upper >= 'A' && upper <= 'Z') return upper
    }
    return '#'
}

/**
 * Per-letter offsets/counts for an ALREADY-SORTED list of names — the client-side
 * counterpart of `Store.GetAlbumLetterIndex`, for lists the server hands over
 * whole (getStarred2) instead of paging.
 *
 * The order given is authoritative: this never re-sorts. A letter that reappears
 * after another one merges into its first bucket (count grows, offset stays), the
 * same way the Go index does — the alphabet rail then jumps to that letter's
 * first item, which is the only sensible target.
 */
export function deriveLetterIndex(names: string[]): AlbumLetter[] {
    const letters: AlbumLetter[] = []
    const indexByLetter = new Map<string, number>()
    names.forEach((name, offset) => {
        const letter = firstLetter(name)
        const at = indexByLetter.get(letter)
        if (at !== undefined) {
            letters[at].count += 1
            return
        }
        indexByLetter.set(letter, letters.length)
        letters.push({ name: letter, offset, count: 1 })
    })
    return letters
}
