// @vitest-environment node
// The rail is hidden on phones and --app-rail-clearance collapses to 0 in the
// same breath (_variables.scss). This spec pins the rail half of that pact.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const source = readFileSync(
    fileURLToPath(new URL('../AlphabetRail.vue', import.meta.url)),
    'utf8'
)

describe('AlphabetRail phone styles', () => {
    it('hides the rail below the phone breakpoint', () => {
        const media = source.match(/@media \(max-width: 767\.98px\)\s*\{[\s\S]*?\n\}/)?.[0]
        expect(media).toBeTruthy()
        expect(media).toMatch(/\.alphabet-rail\s*\{[^}]*display:\s*none/)
    })
})
