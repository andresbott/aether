// @vitest-environment node
// Node env, not jsdom: this spec reads the component's <style> block off disk
// rather than rendering. Scoped SFC styles are never applied by vue-test-utils,
// so no mounted test can see them.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

/**
 * An idle rail is fully neutral — no knob and a grey fill; the accent and the
 * knob only appear while it is hovered or dragged (`.rail-active`).
 *
 * How the knob is hidden matters for keyboard access: `visibility: hidden` and
 * `display: none` both take the handle out of the tab order, so hiding it that
 * way silently makes the volume and progress sliders unreachable without a
 * mouse — the handle carries `tabindex="0"` and `role="slider"`, and arrow keys
 * only work once it holds focus. Fading with opacity keeps it focusable.
 */
const styles = (() => {
    const source = readFileSync(
        fileURLToPath(new URL('../PlayerControls.vue', import.meta.url)),
        'utf8'
    )
    const at = source.indexOf('<style')
    return (
        source
            .slice(source.indexOf('>', at) + 1, source.lastIndexOf('</style>'))
            // Comments are stripped: everything between `}` and `{` is read as a
            // selector below, and a comment sitting there (this file has several,
            // some naming .rail-active) would be captured along with it.
            .replace(/\/\*[\s\S]*?\*\//g, '')
    )
})()

/** Declaration bodies of every rule whose selector matches `pattern`. */
function ruleBodies(pattern: RegExp): string[] {
    const bodies: string[] = []
    for (const match of styles.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
        if (pattern.test(match[1])) bodies.push(match[2])
    }
    return bodies
}

describe('slider handle hiding', () => {
    const hidden = ruleBodies(/\.p-slider-handle\)?,?\s*$/m).filter((body) =>
        /opacity:\s*0\b/.test(body)
    )

    it('has a rule that hides the handle by default', () => {
        expect(hidden.length).toBeGreaterThan(0)
    })

    it('fades the handle out instead of removing it from the tab order', () => {
        for (const body of hidden) {
            expect(body).not.toMatch(/visibility:\s*hidden/)
            expect(body).not.toMatch(/display:\s*none/)
        }
    })

    it('reveals the handle when its rail is hovered or dragged', () => {
        const shown = ruleBodies(/\.rail-active/)
        expect(shown.length).toBeGreaterThan(0)
        expect(shown.some((body) => /opacity:\s*1\b/.test(body))).toBe(true)
    })
})

describe('slider range colour', () => {
    /**
     * Rules styling a rail's filled portion, split by whether they need the user
     * to be interacting with it — `.rail-active` (hover/drag) or focus. The
     * unconditional rules are what an untouched player bar renders.
     */
    const rangeRules = (() => {
        const idle: string[] = []
        const interactive: string[] = []
        for (const match of styles.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
            const [, selector, body] = match
            if (!selector.includes('.p-slider-range')) continue
            const needsInteraction = /\.rail-active|:focus/.test(selector)
            ;(needsInteraction ? interactive : idle).push(body)
        }
        return { idle, interactive }
    })()

    it('fills an idle rail with the neutral player token, not the accent', () => {
        expect(rangeRules.idle.length).toBeGreaterThan(0)
        for (const body of rangeRules.idle) {
            const background = /background:([^;]*)/.exec(body)?.[1] ?? ''
            expect(background).toContain('--app-player-range')
            expect(background).not.toContain('--app-accent')
        }
    })

    it('paints the fill with the accent once the rail is hovered, dragged or focused', () => {
        expect(rangeRules.interactive.length).toBeGreaterThan(0)
        for (const body of rangeRules.interactive) {
            expect(body).toMatch(/background:[^;]*--app-accent/)
        }
    })

    it('covers both the progress and the volume rail', () => {
        const selectorsFor = (needle: string) =>
            [...styles.matchAll(/([^{}]+)\{[^{}]*\}/g)]
                .map((m) => m[1])
                .filter((s) => s.includes('.p-slider-range') && s.includes(needle))
        expect(selectorsFor('.progress-slider').length).toBeGreaterThan(0)
        expect(selectorsFor('.volume-slider').length).toBeGreaterThan(0)
    })
})
