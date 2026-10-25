// @vitest-environment node
// Node env, not jsdom: this spec reads ListCard's <style> block and the token file
// off disk rather than rendering. Scoped SFC styles are never applied by
// vue-test-utils, so no mounted test can see them.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const stripComments = (css: string) => css.replace(/\/\*[\s\S]*?\*\//g, '')
const read = (path: string) => readFileSync(fileURLToPath(new URL(path, import.meta.url)), 'utf8')

const card = (() => {
    const source = read('../ListCard.vue')
    const at = source.indexOf('<style')
    return stripComments(source.slice(source.indexOf('>', at) + 1, source.lastIndexOf('</style>')))
})()
const variables = stripComments(read('../../../assets/scss/_variables.scss'))

/** Declaration bodies of every rule whose selector matches `pattern`. */
function ruleBodies(css: string, pattern: RegExp): string[] {
    const bodies: string[] = []
    for (const match of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
        if (pattern.test(match[1].trim())) bodies.push(match[2])
    }
    return bodies
}

describe('ListCard sizing', () => {
    // The virtual lists reserve cover + 2 × pad + gap per card (useListCardMetrics).
    // A card whose box came out any other size would drift off its slot, misplacing
    // every card below the first and sending a rail jump to the wrong letter.
    it('fills exactly its slot: a border box of cover plus padding, then the gap', () => {
        const [body] = ruleBodies(card, /^\.list-card$/)
        expect(body).toMatch(/box-sizing:\s*border-box/)
        expect(body).toMatch(
            /height:\s*calc\(var\(--list-card-cover\) \+ 2 \* var\(--list-card-pad\)\)/
        )
        expect(body).toMatch(/padding:\s*var\(--list-card-pad\)/)
        expect(body).toMatch(/margin:\s*0 auto var\(--list-card-gap\)/)
    })
})

describe('ListCard heart on touch', () => {
    it('is always shown on a coarse pointer, which has no hover to reveal it', () => {
        const coarse = card.slice(card.indexOf('@media (pointer: coarse)'))
        expect(coarse).toMatch(/\.row-star\s*\{[^}]*opacity:\s*1/)
    })
})

describe('ListCard theme values', () => {
    // Vue compiles `:global(.dark-mode) .list-card` to a bare `.dark-mode` rule,
    // dropping `.list-card`: an override written that way lands on <html>, where
    // the card's own declarations shadow it. Per-theme values are tokens instead.
    it('writes no :global() rule that Vue would collapse to the global part', () => {
        expect(card).not.toMatch(/:global\([^)]*\)\s*[^\s{,]/)
    })

    const root = variables.slice(variables.indexOf(':root'), variables.indexOf('.dark-mode'))
    const dark = variables.slice(variables.indexOf('.dark-mode'))

    const inks = ['--app-list-card-subtitle', '--app-list-card-meta']

    it.each(inks)('%s is set for both themes and read by the card', (token) => {
        expect(root).toContain(`${token}:`)
        expect(dark).toContain(`${token}:`)
        expect(card).toContain(`var(${token})`)
    })
})
