// @vitest-environment node
// Node env, not jsdom: this spec reads the component's <style> block off disk
// rather than rendering. Scoped SFC styles are never applied by vue-test-utils,
// so no mounted test can see them.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const styles = (() => {
    const source = readFileSync(
        fileURLToPath(new URL('../PlayerFace.vue', import.meta.url)),
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

describe('tall-screen transport anchoring', () => {
    const face = ruleBodies(/^\s*\.play-face\s*$/m).join('\n')
    const art = ruleBodies(/^\s*\.play-art\s*$/m).join('\n')
    const meta = ruleBodies(/^\s*\.play-meta\s*$/m).join('\n')

    it('.play-art has margin-top: auto and .play-meta has margin-bottom: auto', () => {
        expect(art).toMatch(/margin-top:\s*auto/)
        expect(meta).toMatch(/margin-bottom:\s*auto/)
    })

    it('.play-face has NO justify-content: center — the auto margins center art+meta', () => {
        expect(face).not.toMatch(/justify-content:\s*center/)
    })
})

describe('nav hint must not shrink on a short screen', () => {
    const navHint = ruleBodies(/^\s*\.play-nav-hint\s*$/m).join('\n')

    it('.play-nav-hint has flex-shrink: 0', () => {
        expect(navHint).toMatch(/flex-shrink:\s*0/)
    })
})

describe('play button color', () => {
    const playBtn = ruleBodies(/^\s*\.play-btn--play\s*$/m).join('\n')

    it('.play-btn--play color is var(--app-player-bg) and not var(--app-background)', () => {
        expect(playBtn).toMatch(/color:\s*var\(--app-player-bg\)/)
        expect(playBtn).not.toMatch(/var\(--app-background\)/)
    })
})
