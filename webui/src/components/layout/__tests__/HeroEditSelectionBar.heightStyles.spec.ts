// @vitest-environment node
// Scoped styles never apply under vue-test-utils and jsdom does no layout, so we
// pin the fix off disk (same technique as the *.Styles specs). The icon-only
// clear button must be capped to the compact button height (2.125rem): otherwise
// it keeps PrimeVue's taller icon-only default (~40px, measured), which makes this
// pill 6px taller than the sibling HeroVisibilityBar pill and jumps the band when
// a track is selected.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const src = readFileSync(
    fileURLToPath(new URL('../HeroEditSelectionBar.vue', import.meta.url)),
    'utf8'
)

describe('HeroEditSelectionBar clear-button height', () => {
    it('caps the icon-only button to the compact 2.125rem row height', () => {
        expect(src).toMatch(/\.p-button-icon-only\)\s*\{[^}]*height:\s*2\.125rem/)
    })
})
