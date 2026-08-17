// @vitest-environment node
// Node env, not jsdom: this spec reads the component's <style> block off disk
// rather than rendering. Scoped SFC styles are never applied by vue-test-utils,
// so no mounted test can see them.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const styles = (() => {
    const source = readFileSync(
        fileURLToPath(new URL('../NowPlayingSheet.vue', import.meta.url)),
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

describe('sheet overlay geometry', () => {
    const root = ruleBodies(/^\s*\.now-playing-sheet\s*$/m).join('\n')

    it('overlays the app shell absolutely, above content and below PrimeVue overlays', () => {
        expect(root).toMatch(/position:\s*absolute/)
        expect(root).toMatch(/inset:\s*0/)
        const z = root.match(/z-index:\s*(\d+)/)
        expect(z).toBeTruthy()
        expect(Number(z![1])).toBeLessThan(1000)
    })

    it('contains its overscroll so a drag never chains into pull-to-refresh', () => {
        expect(root).toMatch(/overscroll-behavior:\s*contain/)
    })
})

describe('player-bar palette on the sheet root', () => {
    const root = ruleBodies(/^\s*\.now-playing-sheet\s*$/m).join('\n')

    it('paints the sheet with the player surface and text', () => {
        expect(root).toMatch(/background-color:\s*var\(--app-player-bg\)/)
        expect(root).toMatch(/color:\s*var\(--app-player-text\)/)
    })

    it('remaps the app tokens the shared children colour themselves with', () => {
        expect(root).toMatch(/--app-text-primary:\s*var\(--app-player-text\)/)
        expect(root).toMatch(/--app-text-secondary:\s*var\(--app-player-dim\)/)
        expect(root).toMatch(/--app-hover:/)
        expect(root).toMatch(/--app-border:/)
        expect(root).toMatch(/--app-accent-soft:/)
    })

    it('leaves the accent alone — it is the "this is playing" signal', () => {
        expect(root).not.toMatch(/--app-accent:\s/)
    })
})

describe('motion is CSS-owned and finger-gated', () => {
    it('animates the sheet and track transforms', () => {
        const root = ruleBodies(/^\s*\.now-playing-sheet\s*$/m).join('\n')
        const track = ruleBodies(/\.sheet-track\s*$/m).join('\n')
        expect(root).toMatch(/transition:[^;]*transform/)
        expect(track).toMatch(/transition:[^;]*transform/)
    })

    it('turns every transition off while a finger owns the motion', () => {
        const dragging = ruleBodies(/\.is-dragging/).join('\n')
        expect(dragging).toMatch(/transition:\s*none/)
    })

    it('honours prefers-reduced-motion without any JS', () => {
        expect(styles).toMatch(/@media \(prefers-reduced-motion: reduce\)/)
        const reduced = styles.match(/@media \(prefers-reduced-motion: reduce\)[\s\S]*?\n\}/)?.[0]
        expect(reduced).toMatch(/transition:\s*none/)
    })
})
