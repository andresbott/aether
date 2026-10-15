// @vitest-environment node
// Two rules keep the SPA origin-agnostic and the desktop app contained, and
// nothing else notices when one slips: the code still works in a browser tab and
// breaks later, in the player, as a request to the wrong host.
//
//   1. The page's own origin (location.origin, however it is reached) is read
//      only under lib/host/. Inside the app the page comes from a tauri:// origin,
//      so it is the wrong answer to "where is the server"; everything else asks
//      `serverUrl`.
//   2. @tauri-apps/* is imported only under lib/host/, so the rest of the SPA is
//      the same code in a tab and in the player, and the dependency on the app
//      stays behind one seam.
//
// Pinned off disk, same technique as the style specs: read the sources and
// assert on their text.
import { describe, it, expect } from 'vitest'
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

// The rules are about code, not prose: a comment explaining why the origin is
// off limits has to be free to quote it. Block comments in <style> blocks and
// <!-- --> comments in templates run over many lines without a leading `*`, so
// skipping lines that look like comments would not be enough. One left-to-right
// pass over strings and comments together: a `//` inside 'http://…' is no
// comment, a quote inside a comment opens no string. Strings are kept, so a
// read inside a `${…}` is still seen. A `//` right after a backslash is no
// comment either: it is a regex literal's escaped slash and its closing one.
const COMMENT_OR_STRING =
    /('(?:\\.|[^'\\\n])*'|"(?:\\.|[^"\\\n])*"|`(?:\\[\s\S]|[^`\\])*`)|\/\*[\s\S]*?\*\/|<!--[\s\S]*?-->|(?<!\\)\/\/[^\n]*/g

const stripComments = (text: string): string =>
    text.replace(COMMENT_OR_STRING, (_, quoted?: string) => quoted ?? ' ')

/**
 * True when code (not a comment) reads the page's own origin: any location.origin,
 * however it is reached (window., document., globalThis. or bare).
 */
const readsPageOrigin = (text: string): boolean => /\blocation\.origin\b/.test(stripComments(text))

/**
 * True when code imports @tauri-apps/*: static (a re-export too), side-effect or
 * dynamic. Import statements only, so a string that merely names the package is
 * not one.
 */
const importsTauri = (text: string): boolean =>
    /\b(?:from\s*|import\s*\(?\s*)['"`]@tauri-apps\//.test(stripComments(text))

describe('the @tauri-apps import matcher', () => {
    it.each([
        ['a static import', "import { invoke } from '@tauri-apps/api/core'"],
        ['a double-quoted import', 'import { fetch } from "@tauri-apps/plugin-http"'],
        ['a type import', "import type { InvokeArgs } from '@tauri-apps/api/core'"],
        ['a dynamic import', "const { fetch } = await import('@tauri-apps/plugin-http')"],
        ['a re-export', "export { invoke } from '@tauri-apps/api/core'"],
        ['a side-effect import', "import '@tauri-apps/api/mocks'"],
        ['a multi-line import', "import {\n    invoke,\n    isTauri\n} from '@tauri-apps/api/core'"]
    ])('sees %s', (_, code) => {
        expect(importsTauri(code)).toBe(true)
    })

    it.each([
        ['a line comment', "// the plugin is loaded from '@tauri-apps/plugin-http' lazily"],
        ['a commented-out import', "// import { invoke } from '@tauri-apps/api/core'"],
        [
            'a doc comment',
            "/**\n * Wraps @tauri-apps/api/core, as in\n * import { invoke } from '@tauri-apps/api/core'\n */"
        ],
        ['an unprefixed block comment', "/*\n  import('@tauri-apps/plugin-http') is lazy\n*/"],
        ['an HTML comment', "<!-- import { invoke } from '@tauri-apps/api/core' -->"],
        ['a string that only names the package', "const pkg = '@tauri-apps/api/core'"],
        ['an import of another package', "import { ref } from 'vue'"]
    ])('skips %s', (_, code) => {
        expect(importsTauri(code)).toBe(false)
    })
})

describe('the page-origin matcher', () => {
    it.each([
        ['a plain read', 'serverUrl.value = window.location.origin'],
        ['a read through document', 'const here = document.location.origin'],
        ['a read through globalThis', 'const here = globalThis.location.origin'],
        ['a bare read', 'const here = location.origin'],
        [
            'a read after a string holding //',
            "const base = dev ? 'http://dev:8075' : window.location.origin"
        ],
        ['a read inside a template literal', 'fetch(`${window.location.origin}/rest/ping`)'],
        ['a read before a trailing comment', 'const origin = window.location.origin // the tab'],
        // The escaped slash and the literal's closing one read as `//`: taken for
        // a comment, it would hide the rest of the line.
        [
            'a read after a regex literal ending in an escaped slash',
            'const scheme = /^https?:\\/\\//; const here = window.location.origin'
        ]
    ])('sees %s', (_, code) => {
        expect(readsPageOrigin(code)).toBe(true)
    })

    it.each([
        ['a line comment', '// never read window.location.origin outside lib/host'],
        ['a trailing comment', 'const reload = true // not window.location.origin'],
        ['a doc comment', '/**\n * The server, not window.location.origin.\n */'],
        ['an unprefixed block comment', '/*\n  window.location.origin is wrong in the app\n*/'],
        ['an HTML comment', '<!-- window.location.origin is the tab, not the server -->'],
        ['a comment naming the bare form', '// location.origin is the tab, not the server'],
        ['another location read', 'window.location.reload()'],
        ['another origin', 'const origin = new URL(url).origin']
    ])('skips %s', (_, code) => {
        expect(readsPageOrigin(code)).toBe(false)
    })
})

// webui/src, three levels up from here.
const srcDir = fileURLToPath(new URL('../../../', import.meta.url))

// The app's own code: every .ts and .vue file. Specs and their fixtures
// (__tests__) are out — they fake @tauri-apps/* and assert on the page origin.
const sourceFiles = (readdirSync(srcDir, { recursive: true }) as string[]).filter(
    (rel) =>
        /\.(ts|vue)$/.test(rel) &&
        !rel.endsWith('.spec.ts') &&
        !rel.split('/').includes('__tests__')
)

const inHostLayer = (rel: string): boolean => rel.startsWith('lib/host/')
const read = (rel: string): string => readFileSync(join(srcDir, rel), 'utf8')

/** The files outside the host layer, as src/… paths, whose text `matches`. */
const outsideHost = (matches: (text: string) => boolean): string[] =>
    sourceFiles.filter((rel) => !inHostLayer(rel) && matches(read(rel))).map((rel) => `src/${rel}`)

// An empty list is the only passing answer; otherwise the message names the files.
function expectNone(offenders: string[], rule: string): void {
    expect(offenders, `${rule}: ${offenders.join(', ')}`).toEqual([])
}

describe('code outside lib/host/', () => {
    it('never reads location.origin', () => {
        expectNone(
            outsideHost(readsPageOrigin),
            "location.origin is read outside src/lib/host/ (ask serverUrl from '@/lib/host' instead)"
        )
    })

    it('never imports @tauri-apps/*', () => {
        expectNone(
            outsideHost(importsTauri),
            '@tauri-apps/* is imported outside src/lib/host/ (go through the host layer instead)'
        )
    })
})

describe('the scan', () => {
    // A wrong root or a matcher that never fires would pass both checks above
    // for good; the host layer is the one place that must show up.
    it("finds the host layer's own uses, so a pass is not for lack of looking", () => {
        const host = sourceFiles.filter(inHostLayer).map(read)
        expect(host.some(readsPageOrigin)).toBe(true)
        expect(host.some(importsTauri)).toBe(true)
    })

    // The host layer is all .ts: a walk that never reached a component would
    // still pass the check above.
    it('reaches the components too', () => {
        expect(sourceFiles.some((rel) => rel.endsWith('.vue'))).toBe(true)
    })
})
