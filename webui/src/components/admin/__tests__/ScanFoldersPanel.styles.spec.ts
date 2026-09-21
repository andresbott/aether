// @vitest-environment node
// Node env, not jsdom: this spec reads the component's <style> block off disk
// rather than rendering. Scoped SFC styles are never applied by vue-test-utils,
// and jsdom measures nothing, so no mounted test can see this.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const styles = (() => {
    const source = readFileSync(
        fileURLToPath(new URL('../ScanFoldersPanel.vue', import.meta.url)),
        'utf8'
    )
    const at = source.indexOf('<style')
    return source
        .slice(source.indexOf('>', at) + 1, source.lastIndexOf('</style>'))
        .replace(/\/\*[\s\S]*?\*\//g, '')
})()

/** Declaration bodies of every rule whose selector matches `pattern`. */
function ruleBodies(pattern: RegExp): string[] {
    const bodies: string[] = []
    for (const match of styles.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
        if (pattern.test(match[1])) bodies.push(match[2])
    }
    return bodies
}

describe('the phone path wraps', () => {
    const namePath = ruleBodies(/^\s*\.name-path\s*$/m).join('\n')

    // A table cell with no width constraint grows to fit nowrap content, so the
    // ellipsis never triggers: the whole table became as wide as the path and
    // pushed the Status column (and the "Not usable" reason) off a 390px
    // screen. Wrapping the path is what keeps the row on screen.
    it('does not hold the path on one line', () => {
        expect(namePath).not.toMatch(/white-space:\s*nowrap/)
        expect(namePath).not.toMatch(/text-overflow:\s*ellipsis/)
    })

    it('breaks a long unbroken path rather than widening the table', () => {
        expect(namePath).toMatch(/white-space:\s*normal/)
        expect(namePath).toMatch(/overflow-wrap:\s*anywhere/)
    })
})

describe('an unusable folder’s reason', () => {
    const statusProblem = ruleBodies(/^\s*\.status-problem\s*$/m).join('\n')

    // The reason is a sentence around a filesystem path, and a long path is one
    // unbreakable token: without this it widens the Status column instead of
    // wrapping, which is what put it off the side of a phone screen.
    it('breaks the long path it quotes instead of widening its column', () => {
        expect(statusProblem).toMatch(/overflow-wrap:\s*anywhere/)
    })
})
