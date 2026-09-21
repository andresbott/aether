// @vitest-environment node
// Node env, not jsdom: this spec reads the view's <style> block off disk rather
// than rendering. Scoped SFC styles are never applied by vue-test-utils, so no
// mounted test can see which of two competing rules wins.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const styles = (() => {
    const source = readFileSync(
        fileURLToPath(new URL('../UserSettingsView.vue', import.meta.url)),
        'utf8'
    )
    const at = source.indexOf('<style')
    return source
        .slice(source.indexOf('>', at) + 1, source.lastIndexOf('</style>'))
        .replace(/\/\*[\s\S]*?\*\//g, '')
})()

/** Selectors of every rule whose declarations match `pattern`. */
function selectorsDeclaring(pattern: RegExp): string[] {
    const found: string[] = []
    for (const match of styles.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
        if (pattern.test(match[2])) found.push(match[1].trim())
    }
    return found
}

describe('the change-password error keeps the error colour', () => {
    // `.profile-body p` paints every paragraph of the panel in secondary text,
    // and scoping adds the same [data-v-…] to both rules — so a bare `.pw-error`
    // LOSES to it (0,2,0 against 0,2,1) and the validation message renders as
    // ordinary grey. It has to stay the more specific of the two.
    it('selects .pw-error more specifically than .profile-body p', () => {
        const errorRule = selectorsDeclaring(/color:\s*var\(--app-danger/).find((s) =>
            s.includes('.pw-error')
        )
        expect(errorRule).toBeDefined()
        expect(errorRule).toContain('.profile-body')
        expect(errorRule).toMatch(/p\.pw-error/)
    })
})
