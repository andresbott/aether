import { readFileSync } from 'node:fs'
import { createRequire } from 'node:module'

// The subset of the Iconify JSON format that @iconify-json/material-symbols uses.
export interface IconSet {
    icons: Record<string, { body: string; width?: number; height?: number }>
    aliases?: Record<string, { parent: string }>
    width?: number
    height?: number
}

export type Variant = 'outline' | 'fill'

const require = createRequire(import.meta.url)

export function loadIconSet(): IconSet {
    return JSON.parse(readFileSync(require.resolve('@iconify-json/material-symbols/icons.json'), 'utf8'))
}

// Rounded style only. Outline falls back to the rounded (filled) form because
// Google ships no outline for some glyphs (expand_more, …) — they are the same
// shape either way.
function candidates(kebab: string, variant: Variant): string[] {
    return variant === 'outline' ? [`${kebab}-outline-rounded`, `${kebab}-rounded`] : [`${kebab}-rounded`]
}

function resolve(set: IconSet, name: string) {
    let n = name
    for (let hops = 0; set.aliases?.[n]; hops++) {
        if (hops > 8) return undefined
        n = set.aliases[n].parent
    }
    return set.icons[n]
}

// The full, standalone <svg> for a Material Symbols name (snake or kebab case).
export function iconSvg(set: IconSet, name: string, variant: Variant): string | undefined {
    const kebab = name.replace(/_/g, '-')
    for (const id of candidates(kebab, variant)) {
        const icon = resolve(set, id)
        if (!icon) continue
        const w = icon.width ?? set.width ?? 24
        const h = icon.height ?? set.height ?? 24
        return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${w} ${h}">${icon.body}</svg>`
    }
    return undefined
}
