import { iconSvg, type IconSet } from './iconset'

export interface Catalogue {
    // Every Rounded icon, as the snake_case name a library stores, sorted.
    names: string[]
    // Its standalone SVG (outlined where Google ships one), for icons/ms/<name>.svg.
    svgs: Map<string, string>
}

// Every icon has a filled "<name>-rounded" entry (the outline one is optional),
// so those keys enumerate the Rounded set. Google's names are snake_case and
// never contain a dash, so the Iconify kebab maps back losslessly.
export function buildCatalogue(set: IconSet): Catalogue {
    const bases = new Set<string>()
    for (const key of [...Object.keys(set.icons), ...Object.keys(set.aliases ?? {})]) {
        if (key.endsWith('-rounded') && !key.endsWith('-outline-rounded')) bases.add(key.slice(0, -'-rounded'.length))
    }
    const names: string[] = []
    const svgs = new Map<string, string>()
    for (const kebab of bases) {
        const svg = iconSvg(set, kebab, 'outline')
        if (!svg) continue
        const name = kebab.replace(/-/g, '_')
        names.push(name)
        svgs.set(name, svg)
    }
    // Sorted after the kebab→snake swap: "-" and "_" collate differently.
    return { names: names.sort(), svgs }
}
