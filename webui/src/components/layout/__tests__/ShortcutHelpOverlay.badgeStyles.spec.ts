// @vitest-environment node
// Node env, not jsdom: this spec reads the component's <style> block off disk
// rather than rendering. Scoped SFC styles are never applied by vue-test-utils,
// so no mounted test can see the transform that does the actual placing.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

/**
 * The measured `left`/`top` are the control's own coordinates; the transform is
 * what turns them into a placed badge. Two placements exist:
 *
 * - floating (default) — centred horizontally on the control and lifted fully
 *   above it, into the dimmed space over the player bar.
 * - side — just past the control's right edge and vertically centred on it, for
 *   a sidebar nav entry that has another nav item directly above it.
 *
 * Getting these transforms wrong puts a badge over the control it is meant to
 * label, and only a real browser would show it — hence this parse.
 */
const styles = (() => {
    const source = readFileSync(
        fileURLToPath(new URL('../ShortcutHelpOverlay.vue', import.meta.url)),
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

describe('badge placement transforms', () => {
    it('lifts a floating badge fully above its control', () => {
        const bodies = ruleBodies(/\.shortcut-badge\s*$/m)
        const transform = bodies.join('\n').match(/transform:\s*([^;]+)/)?.[1]
        // -100% on Y is what clears the control entirely rather than overlapping it.
        expect(transform).toContain('-50%')
        expect(transform).toContain('-100%')
    })

    it('centres a side badge on its row instead of lifting it', () => {
        const bodies = ruleBodies(/\.shortcut-badge--side/)
        expect(bodies.length).toBeGreaterThan(0)
        const transform = bodies.join('\n').match(/transform:\s*([^;]+)/)?.[1]
        // No X offset (the measured left is already past the control's edge) and
        // -50% on Y to centre it on the entry.
        expect(transform).toContain('-50%')
        expect(transform).not.toContain('-100%')
    })
})

// The panel sits in the top-right corner, out of the way of everything it is
// explaining: the player bar it badges is at the bottom, and the nav entries it
// badges are down the left edge. Centred, it covered the content the badges point
// at.
describe('panel placement', () => {
    const panel = () => ruleBodies(/\.shortcut-panel\s*$/m).join('\n')

    it('pins the panel to the top-right corner', () => {
        const body = panel()
        expect(body).toMatch(/(^|\s)right:/)
        expect(body).toMatch(/(^|\s)top:/)
    })

    it('does not centre the panel', () => {
        const body = panel()
        // A centred panel needs left:50% plus a translate to pull itself back.
        expect(body).not.toMatch(/left:\s*50%/)
        expect(body).not.toMatch(/translate\(-50%, *-50%\)/)
    })

    // The badges are pinned in viewport coordinates and the panel is fixed too, so
    // the panel must not grow into the player bar at the bottom of the screen.
    it('keeps the panel clear of the player bar', () => {
        expect(panel()).toContain('--app-player-height')
    })
})
