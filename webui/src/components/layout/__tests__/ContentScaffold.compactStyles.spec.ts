// @vitest-environment node
// Scoped styles never apply under vue-test-utils, so the compact phone header
// is pinned off disk (same technique as PlayerControls.railStyles.spec.ts).
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const source = readFileSync(
    fileURLToPath(new URL('../ContentScaffold.vue', import.meta.url)),
    'utf8'
)

describe('ContentScaffold base header wrap', () => {
    // Strip the media query so assertions only match the base rules (otherwise
    // they'd pass if the rules moved back inside the phone block).
    const baseRules = source.replace(/@media \(max-width: 767\.98px\)\s*\{[\s\S]*?\n\}/, '')

    it('enables wrap on .scaffold-header-inner at base width', () => {
        expect(baseRules).toMatch(/\.scaffold-header-inner\s*\{[^}]*flex-wrap:\s*wrap/)
    })

    // The pair that makes the phone header work without any phone flex
    // override: the grower keeps the title beside the hamburger/Back button
    // (and the actions right-aligned), the min-width forces a wide #actions
    // to wrap below instead of crushing the title.
    it('keeps .scaffold-title a min-12rem grower at base width', () => {
        expect(baseRules).toMatch(/\.scaffold-title\s*\{[^}]*flex:\s*1;/)
        expect(baseRules).toMatch(/\.scaffold-title\s*\{[^}]*min-width:\s*12rem/)
    })

    it('resets min-width on .scaffold-title:empty at base width', () => {
        expect(baseRules).toMatch(/\.scaffold-title:empty\s*\{[^}]*min-width:\s*0/)
    })
})

describe('ContentScaffold compact phone header', () => {
    const media = source.match(/@media \(max-width: 767\.98px\)\s*\{[\s\S]*?\n\}/)?.[0]

    it('has a phone media query', () => {
        expect(media).toBeTruthy()
    })

    it('shrinks the title', () => {
        expect(media).toMatch(/\.scaffold-title h1\s*\{[^}]*font-size:\s*1\.2rem/)
    })

    it('lets the summary wrap below the title', () => {
        expect(media).toMatch(/\.scaffold-summary\s*\{[^}]*flex-basis:\s*100%/)
        expect(media).toMatch(/\.scaffold-title\s*\{[^}]*flex-wrap:\s*wrap/)
    })

    // Since the nav moved into the hamburger drawer, the hamburger (or Back)
    // and the title share the first row: the base `flex: 1` grower must NOT
    // be re-based to a full-width row on phones — `flex: 1 1 100%` (the
    // pre-hamburger layout) stranded the hamburger alone on its own row.
    // The base min-width: 12rem is what still forces a wide #actions
    // (Library's three-option tab SelectButton) to wrap below the title
    // instead of crushing it.
    it('keeps the title beside the hamburger (no full-width re-basing)', () => {
        expect(media).not.toMatch(/\.scaffold-title[^{]*\{[^}]*flex:\s*1 1 100%/)
    })

    // When the actions DO wrap they land left-aligned. No `margin-left: auto`
    // on .scaffold-actions: it would right-align that wrapped second row too.
    it('leaves wrapped actions left-aligned', () => {
        expect(media).not.toMatch(/\.scaffold-actions\s*\{[^}]*margin-left:\s*auto/)
    })
})
