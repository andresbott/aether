// @vitest-environment node
// Scoped styles never apply under vue-test-utils; pin the content pane's
// scrolling off disk (same technique as SettingsLayout.phoneStyles.spec.ts).
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const source = readFileSync(
    fileURLToPath(new URL('../SettingsLayout.vue', import.meta.url)),
    'utf8'
)

describe('SettingsLayout content pane', () => {
    const pane = source.match(/\.settings-content\s*\{[^}]*\}/)?.[0]

    // The layout is one viewport tall and the document never scrolls, so a
    // settings view taller than the pane is reachable only if the pane itself
    // scrolls. A clipping pane cut off the libraries list on short screens.
    it('is the vertical scroller for every settings view', () => {
        expect(pane).toMatch(/overflow-y:\s*auto/)
        expect(pane).not.toMatch(/overflow:\s*hidden/)
    })
})
