import type { AlbumOption, AlbumAssignment, Track } from '@/types/metadata'

// How many of the selected files each staged field would rewrite if this album
// option were applied in its default state (every song included, every proposed
// position kept). It mirrors the current-vs-target predicates the review table
// in IdentifyAlbumDialog highlights, so the picker's per-row summary agrees with
// the cells the user sees once they pick the option.
export interface AlbumChangeCounts {
    titles: number
    artists: number
    albums: number
    years: number
}

// A year is compared as the string the dialog renders: an absent year (0) is ''
// like every other missing tag rather than a literal 0.
function yearStr(year: number): string {
    return year > 0 ? String(year) : ''
}

function joinNames(names: Array<{ name: string }>): string {
    return names.map((n) => n.name).filter((n) => n !== '').join(', ')
}

// The artist a save would write for one row: the assignment's own credits, or —
// when it carries none — the release's album artists, exactly as the dialog's
// Artist column resolves it.
function targetArtist(option: AlbumOption, assignment: AlbumAssignment | undefined): string {
    const placed = assignment !== undefined && assignment.source !== 'none'
    const own = placed ? joinNames(assignment!.artists ?? []) : ''
    if (own !== '') return own
    return joinNames(option.artists ?? [])
}

export function albumChangeCounts(option: AlbumOption, tracks: Track[]): AlbumChangeCounts {
    const byPath = new Map<string, AlbumAssignment>()
    for (const a of option.assignments ?? []) byPath.set(a.path, a)

    const targetAlbum = option.album ?? ''
    const targetYear = yearStr(option.year)

    const counts: AlbumChangeCounts = { titles: 0, artists: 0, albums: 0, years: 0 }
    for (const t of tracks) {
        const assignment = byPath.get(t.path)
        const placed = assignment !== undefined && assignment.source !== 'none'

        // Title: only a placed row proposes one; an unplaced row keeps the file's.
        const title = placed ? assignment!.title ?? '' : ''
        if (title !== '' && title !== (t.title ?? '')) counts.titles++

        // Artist: staged for every row, via the album-artist fallback when the
        // row has no credits of its own.
        const artist = targetArtist(option, assignment)
        const currentArtist = (t.artists ?? []).filter((n) => n !== '').join(', ')
        if (artist !== '' && artist !== currentArtist) counts.artists++

        // Album and year are release-level: applied to every row, changed only
        // where the file does not already carry them.
        if (targetAlbum !== '' && targetAlbum !== (t.album ?? '')) counts.albums++
        if (targetYear !== '' && targetYear !== yearStr(t.year)) counts.years++
    }
    return counts
}

// A compact one-line summary of the counts for the candidate table, listing only
// the fields that actually change: "12 titles · 1 album · 1 year". "No changes"
// when the option would rewrite nothing (it already matches the files on disk).
export function summarizeAlbumChanges(counts: AlbumChangeCounts): string {
    const parts: string[] = []
    const push = (n: number, singular: string) => {
        if (n > 0) parts.push(`${n} ${singular}${n === 1 ? '' : 's'}`)
    }
    push(counts.titles, 'title')
    push(counts.artists, 'artist')
    push(counts.albums, 'album')
    push(counts.years, 'year')
    return parts.length > 0 ? parts.join(' · ') : 'No changes'
}
