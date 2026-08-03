// @vitest-environment node
// Node env, not jsdom: this spec compiles stylesheets off disk rather than
// rendering, and under jsdom `import.meta.url` is not a file: URL so the paths
// below cannot be resolved.
import { describe, it, expect } from 'vitest'
import { fileURLToPath } from 'node:url'
import * as sass from 'sass-embedded'

/**
 * The player's rails paint their filled portion with `--app-player-range` when
 * idle and only switch to `--app-accent` on hover. A theme that defines the
 * accent but forgets the neutral token would inherit whatever the light theme
 * set — so every palette that repaints `--app-player-track` must repaint this
 * alongside it. The hidden themes are token-only repaints, which makes a missing
 * token invisible until someone unlocks that theme and looks at the player bar.
 */
// Comments are stripped: everything between `}` and `{` is read as a selector
// below, and a comment sitting there would be captured along with it.
const compile = (file: string) =>
    sass
        .compile(fileURLToPath(new URL(`../${file}`, import.meta.url)), { style: 'expanded' })
        .css.replace(/\/\*[\s\S]*?\*\//g, '')
        .replace(/@charset[^;]*;/g, '')

const variables = compile('_variables.scss')
const hiddenThemes = compile('_hidden-themes.scss')

/** Selectors of every block that declares `token`. */
function declaringSelectors(css: string, token: string): string[] {
    const found: string[] = []
    for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
        if (match[2].includes(`${token}:`)) found.push(match[1].trim())
    }
    return found
}

describe('--app-player-range', () => {
    it('has a base value and a dark-mode override', () => {
        const selectors = declaringSelectors(variables, '--app-player-range')
        expect(selectors).toContain(':root')
        expect(selectors).toContain('.dark-mode')
    })

    it('is defined by every palette that repaints the rail', () => {
        // Same set of blocks as --app-player-track: a palette that restyles the
        // rail's groove has to restyle the fill sitting inside it too.
        for (const css of [variables, hiddenThemes]) {
            expect(declaringSelectors(css, '--app-player-range')).toEqual(
                declaringSelectors(css, '--app-player-track')
            )
        }
    })

    it('is not just the accent under another name', () => {
        // A neutral fill is the whole point: if it resolved to the accent, an
        // idle rail would stay cyan/green and the hover state would be invisible.
        const values = [variables, hiddenThemes].flatMap((css) =>
            [...css.matchAll(/--app-player-range:([^;]*)/g)].map((m) => m[1])
        )
        expect(values.length).toBeGreaterThan(0)
        for (const value of values) expect(value).not.toContain('--app-accent')
    })
})
