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
    // to ~67px while the header row was nowrap.
    it('wraps the header row so the title owns the first line', () => {
        expect(media).toMatch(/\.scaffold-header-inner\s*\{[^}]*flex-wrap:\s*wrap/)
        expect(media).toMatch(/\.scaffold-title\s*\{[^}]*flex:\s*1 1 100%/)
    })

    // Detail views render an empty title box; a full-width empty row would push
    // their actions below the back button.
    it('drops the title box when it is empty', () => {
        expect(media).toMatch(/\.scaffold-title:empty\s*\{[^}]*display:\s*none/)
    })
})
