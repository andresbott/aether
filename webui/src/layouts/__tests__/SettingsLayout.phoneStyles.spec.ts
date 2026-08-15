// @vitest-environment node
// Scoped styles never apply under vue-test-utils; pin the phone top-bar off
// disk (same technique as the other *.phoneStyles specs).
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const source = readFileSync(
    fileURLToPath(new URL('../SettingsLayout.vue', import.meta.url)),
    'utf8'
)

describe('SettingsLayout phone top bar', () => {
    const media = source.match(/@media \(max-width: 767\.98px\)\s*\{[\s\S]*?\n\}/)?.[0]

    it('has a phone media query', () => {
        expect(media).toBeTruthy()
    })

    // Uses 100dvh (not 100vh) to account for mobile browser UI bars (same
    // rationale as MobileShell's dvh — see its comment).
    it('uses 100dvh for layout height (in the base rule, not media query)', () => {
        const baseBlock = source.match(/\.settings-layout\s*\{[^}]*\}/)?.[0]
        expect(baseBlock).toMatch(/height:\s*100dvh/)
    })

    it('stacks the layout and turns the sidebar into a bar', () => {
        expect(media).toMatch(/\.settings-layout\s*\{[^}]*flex-direction:\s*column/)
        expect(media).toMatch(/\.settings-sidebar[^{]*\{[^}]*width:\s*100%/)
        expect(media).toMatch(/\.sidebar-nav\s*\{[^}]*flex-direction:\s*row/)
    })

    it('hides the desktop-only chrome', () => {
        for (const sel of ['.nav-label', '.nav-section-label', '.collapse-btn', '.sidebar-version']) {
            expect(media).toMatch(new RegExp(`${sel.replace('.', '\\.')}[^{]*\\{[^}]*display:\\s*none`))
        }
    })

    it('moves the active accent to the bottom edge', () => {
        expect(media).toMatch(/\.nav-item\.active\s*\{[^}]*inset 0 -3px 0/)
    })
})
