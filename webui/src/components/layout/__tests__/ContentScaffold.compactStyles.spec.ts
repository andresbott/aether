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

    it('sets min-width: 12rem on .scaffold-title at base width', () => {
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

    // A wide #actions (Library's three-option tab SelectButton) crushed the title
    // to ~67px while the header row was nowrap. Now flex-wrap is in base rules
    // so title wrapping is width-agnostic; this test verifies the title still
    // owns the first line on phones by taking 100% flex basis.
    it('gives the title a full-width row on phones', () => {
        expect(media).toMatch(/\.scaffold-title\s*\{[^}]*flex:\s*1 1 100%/)
    })

    // Detail views (/album, /artist, /playlist/:id, /genre/:name) render an empty
    // title box. Two outcomes have to hold at once, and they pull opposite ways:
    // the empty box must not claim a full-width row (that pushes the actions below
    // the back button), yet it must still be the flex spacer that keeps the actions
    // right-aligned — .scaffold-back and .scaffold-actions are both flex-shrink: 0
    // with no grower, so removing the box from the flow packs them LEFT.
    // `flex: 1 1 0` is the one value that satisfies both; `display: none` broke the
    // second. `:empty` is 0,2,0 so it wins over `.scaffold-title` regardless of order.
    it('keeps the empty title box as a spacer so the actions stay right-aligned', () => {
        expect(media).toMatch(/\.scaffold-title:empty\s*\{[^}]*flex:\s*1 1 0/)
        expect(media).not.toMatch(/\.scaffold-title:empty\s*\{[^}]*display:\s*none/)
    })

    // The titled case (Library) must be untouched by that fix: the title still takes
    // the whole first row and the actions still wrap below it, left-aligned. Hence no
    // `margin-left: auto` on .scaffold-actions, which would have right-aligned the
    // second row too.
    it('leaves the titled case with a full-width title and no actions offset', () => {
        expect(media).toMatch(/\.scaffold-title\s*\{[^}]*flex:\s*1 1 100%/)
        expect(media).not.toMatch(/\.scaffold-actions\s*\{[^}]*margin-left:\s*auto/)
    })
})
