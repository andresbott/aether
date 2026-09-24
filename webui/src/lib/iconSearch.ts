import { SUGGESTED_LIBRARY_ICONS } from '@/lib/libraryIcons'

// The picker renders at most this many; the rest is one keystroke away.
export const MAX_ICON_RESULTS = 120

// Icon names for a query, best first: exact, prefix, word start ("music" →
// "library_music"), then any substring — each tier in catalogue (alphabetical)
// order. An empty query offers the curated suggestions instead of the alphabet.
export function searchIcons(names: readonly string[], query: string, limit = MAX_ICON_RESULTS): string[] {
    const q = query.trim().toLowerCase().replace(/[\s-]+/g, '_')
    if (!q) return SUGGESTED_LIBRARY_ICONS.filter((n) => names.includes(n)).slice(0, limit)
    const tiers: string[][] = [[], [], [], []]
    for (const name of names) {
        if (name === q) tiers[0].push(name)
        else if (name.startsWith(q)) tiers[1].push(name)
        else if (name.includes(`_${q}`)) tiers[2].push(name)
        else if (name.includes(q)) tiers[3].push(name)
    }
    return tiers.flat().slice(0, limit)
}
