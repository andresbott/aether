// @vitest-environment node
// Node env, not jsdom: this spec reads the component's <style> block off disk
// rather than rendering. Scoped SFC styles are never applied by vue-test-utils,
// so no mounted test can see them.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

/**
 * The phone's Now Playing keeps the player-bar palette (dark blue) in BOTH
 * themes — the transport belongs to the player chrome, not the page. The view
 * hosts shared children (ContentScaffold's header, QueueBody's rows) that
 * colour themselves with the app tokens, which in light theme are near black
 * and would land on the dark surface at ~1.1:1 — invisible. The fix is the one
 * the old PlayerSheet used: remap those tokens for the subtree instead of
 * forking the children. This spec pins both the surface and the remap.
 */
const styles = (() => {
    const source = readFileSync(
        fileURLToPath(new URL('../MobilePlayView.vue', import.meta.url)),
        'utf8'
    )
    const at = source.indexOf('<style')
    return (
        source
            .slice(source.indexOf('>', at) + 1, source.lastIndexOf('</style>'))
            // Comments are stripped: everything between `}` and `{` is read as
            // a selector below, and a comment sitting there would be captured
            // along with it.
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

describe('player-bar palette on the view root', () => {
    const root = ruleBodies(/^\s*\.mobile-play-view\s*$/m).join('\n')

    it('paints the view with the player surface and text', () => {
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

describe('play disc icon', () => {
    it('matches the player surface, not the page background', () => {
        const disc = ruleBodies(/\.play-btn--play/).join('\n')
        expect(disc).toMatch(/color:\s*var\(--app-player-bg\)/)
        expect(disc).not.toMatch(/color:\s*var\(--app-background\)/)
    })
})
