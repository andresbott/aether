// @vitest-environment node
// Node env, not jsdom: this spec compiles a stylesheet off disk rather than
// rendering, and under jsdom `import.meta.url` is not a file: URL so the path
// below cannot be resolved.
import { describe, it, expect } from 'vitest'
import { fileURLToPath } from 'node:url'
import * as sass from 'sass-embedded'

/**
 * The CRT theme's scanline overlay is pure CSS, so no component test can catch
 * it disappearing — the class plumbing in useTheme.spec.ts stays green either
 * way. These assertions compile the real stylesheet and check the properties
 * that decide whether the effect is actually painted.
 *
 * Regression guarded: the overlay was once wrapped in
 * `@media (prefers-reduced-motion: reduce) { display: none }`, which hid it
 * entirely on any desktop reporting reduced motion (KDE with a low
 * AnimationDurationFactor does). The palette still applied, so the theme looked
 * like "colours changed but no effect".
 */
// Resolved relative to this file, so the spec passes from any working directory
// (`make ui-test` runs vitest from the repo root, not from webui/).
const compiled = sass.compile(fileURLToPath(new URL('../_hidden-themes.scss', import.meta.url)), {
    style: 'expanded'
}).css

/** The `@media (...) { ... }` block whose query matches, or undefined. */
function mediaBlock(query: string): string | undefined {
    const at = compiled.indexOf(`@media ${query}`)
    if (at === -1) return undefined
    const open = compiled.indexOf('{', at)
    let depth = 0
    for (let i = open; i < compiled.length; i++) {
        if (compiled[i] === '{') depth++
        else if (compiled[i] === '}' && --depth === 0) return compiled.slice(open, i)
    }
    return compiled.slice(open)
}

/** Declaration body of the first rule whose selector contains `selector`. */
function ruleBody(selector: string): string {
    const at = compiled.indexOf(selector)
    expect(at, `no rule for ${selector}`).toBeGreaterThan(-1)
    const open = compiled.indexOf('{', at)
    return compiled.slice(open + 1, compiled.indexOf('}', open))
}

describe('CRT scanline overlay', () => {
    const overlay = ruleBody('.dark-mode.theme-crt::after')

    it('is painted as a fixed, non-interactive layer above the app', () => {
        expect(overlay).toContain('position: fixed')
        expect(overlay).toContain('pointer-events: none')
        expect(overlay).toMatch(/z-index:\s*9999/)
        expect(overlay).toMatch(/content:\s*(''|"")/)
        expect(overlay).toContain('repeating-linear-gradient')
    })

    it('is not hidden by a reduced-motion preference', () => {
        // The overlay is a static gradient — nothing animates, so the query
        // does not apply. Gating on it silently disables the whole effect.
        expect(mediaBlock('(prefers-reduced-motion: reduce)')).toBeUndefined()
    })

    it('is dropped when the user asks for more contrast', () => {
        const block = mediaBlock('(prefers-contrast: more)')
        expect(block).toBeDefined()
        expect(block).toContain('.dark-mode.theme-crt::after')
        expect(block).toContain('display: none')
    })

    it('stripes with a light line, not only a dark one', () => {
        // --app-background is #040804; a black stripe over it composites to
        // roughly #030603, so dark-only stripes are invisible on empty canvas.
        // A faint phosphor line is what makes the raster readable.
        const lightStops = overlay.match(/rgba\(\s*51,\s*255,\s*102/g) ?? []
        expect(lightStops.length).toBeGreaterThan(0)
    })

    it('keeps the stripe period tight enough to read as a raster', () => {
        // Gradient stops are authored in px; the largest is the period.
        const period = Math.max(...[...overlay.matchAll(/(\d+)px/g)].map((m) => +m[1]))
        expect(period).toBeGreaterThan(0)
        expect(period).toBeLessThanOrEqual(4)
    })
})

describe('Winamp theme', () => {
    it('adds no overlay of its own', () => {
        // Documented as effect-free by design, so it cannot render oddly.
        expect(compiled).not.toContain('.dark-mode.theme-winamp::after')
    })
})
