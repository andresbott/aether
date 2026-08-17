// @vitest-environment node
// Node env, not jsdom: this spec reads the component's <style> block off disk
// rather than rendering. Scoped SFC styles are never applied by vue-test-utils,
// so no mounted test can see them.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const styles = (() => {
    const source = readFileSync(
        fileURLToPath(new URL('../QueuePanel.vue', import.meta.url)),
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

describe('queue heading layout', () => {
    const heading = ruleBodies(/^\s*\.queue-heading\s*$/m).join('\n')
    const headingText = ruleBodies(/^\s*\.queue-heading-text\s*$/m).join('\n')
    const summary = ruleBodies(/^\s*\.queue-heading-summary\s*$/m).join('\n')

    it('.queue-heading has flex-shrink: 0', () => {
        expect(heading).toMatch(/flex-shrink:\s*0/)
    })

    it('.queue-heading-text has min-width: 0', () => {
        expect(headingText).toMatch(/min-width:\s*0/)
    })

    it('.queue-heading-summary has text-overflow: ellipsis', () => {
        expect(summary).toMatch(/text-overflow:\s*ellipsis/)
    })
})

describe('overscroll containment', () => {
    it('the rule covering .play-queue-list and :deep(.queue-body) has overscroll-behavior-y: contain', () => {
        // Match rules that include both selectors (multi-selector rule).
        const overscrollRules = ruleBodies(/\.play-queue-list.*:deep\(\.queue-body\)|:deep\(\.queue-body\).*\.play-queue-list/).join('\n')
        expect(overscrollRules).toMatch(/overscroll-behavior-y:\s*contain/)
    })
})
