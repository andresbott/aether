import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { iconSvg, type IconSet } from './iconset'

// A static icon class: ms-<name> is outlined, msf-<name> filled. The lookbehind
// keeps words like "items-center" and custom properties like --ms-svg out.
const TOKEN_RE = /(?<![\w-])(msf?)-([a-z0-9]+(?:-[a-z0-9]+)*)(?![\w-])/g
// A class assembled at runtime would never reach the scanner, so its CSS would
// silently be missing — refuse it. Runtime-chosen icons go through LibraryIcon.
const DYNAMIC_RE = /(?<![\w-])msf?-\$\{|(?<![\w-])msf?-['"`]\s*\+/

export function scanTokens(source: string, file: string): Set<string> {
    if (DYNAMIC_RE.test(source)) {
        throw new Error(`${file}: dynamic icon class — use a literal ms-/msf- class, or <LibraryIcon> for runtime-chosen icons`)
    }
    return new Set([...source.matchAll(TOKEN_RE)].map((m) => m[0]))
}

// Every icon token in the app's own source; tests are excluded (they query
// classes, they do not render new ones).
export function scanDir(dir: string): Set<string> {
    const out = new Set<string>()
    for (const rel of readdirSync(dir, { recursive: true }) as string[]) {
        if (!/\.(vue|ts)$/.test(rel) || /(^|[/\\])__tests__[/\\]|\.spec\.ts$/.test(rel)) continue
        const file = join(dir, rel)
        for (const t of scanTokens(readFileSync(file, 'utf8'), file)) out.add(t)
    }
    return out
}

// Sized in em so icons follow font-size like the old icon font; the two knobs
// exist because Material glyphs carry ~2/24 padding the PrimeIcons font did not.
const COMMON =
    'display:inline-block;flex:none;width:var(--app-icon-size,1.2em);height:var(--app-icon-size,1.2em);' +
    'vertical-align:var(--app-icon-align,-0.25em);background-color:currentColor;' +
    '-webkit-mask:var(--ms-svg) no-repeat center/100% 100%;mask:var(--ms-svg) no-repeat center/100% 100%'

const SPIN = [
    '@keyframes icon-spin{to{transform:rotate(360deg)}}',
    // PrimeVue hard-codes pi-spin on its own loading icons (Select, Tree, DataTable, …).
    '.icon-spin,.pi-spin{animation:icon-spin 1s linear infinite}'
]

const svgUrl = (svg: string) => `url("data:image/svg+xml,${encodeURIComponent(svg)}")`

export function buildIconCss(set: IconSet, tokens: Iterable<string>): string {
    const selectors = ['.app-icon']
    const rules: string[] = []
    const missing: string[] = []
    for (const token of [...tokens].sort()) {
        const fill = token.startsWith('msf-')
        const svg = iconSvg(set, token.slice(fill ? 4 : 3), fill ? 'fill' : 'outline')
        if (!svg) {
            missing.push(token)
            continue
        }
        selectors.push(`.${token}`)
        rules.push(`.${token}{--ms-svg:${svgUrl(svg)}}`)
    }
    if (missing.length) throw new Error(`Unknown Material Symbols icon class(es): ${missing.join(', ')}`)
    return [`${selectors.join(',')}{${COMMON}}`, ...rules, ...SPIN].join('\n')
}
