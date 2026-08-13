// @vitest-environment node
// Scoped styles never apply under vue-test-utils; pin the phone hero stack off
// disk (same technique as ContentScaffold.compactStyles.spec.ts).
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const read = (rel: string): string =>
    readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

describe('hero phone stacking', () => {
    const hero = read('../HeroHeader.vue').match(
        /@media \(max-width: 767\.98px\)\s*\{[\s\S]*?\n\}/
    )?.[0]

    it('stacks the hero vertically and centers it', () => {
        expect(hero).toBeTruthy()
        expect(hero).toMatch(/\.hero-header\s*\{[^}]*flex-direction:\s*column/)
        expect(hero).toMatch(/\.hero-header\s*\{[^}]*align-items:\s*center/)
    })

    it('shrinks the cover and the name', () => {
        expect(hero).toMatch(/\.hero-cover\s*\{[^}]*width:\s*min\(60vw, 250px\)/)
        expect(hero).toMatch(/\.hero-name\)?\s*\{[^}]*font-size:\s*1\.6rem/)
    })

    it('lets the actions wrap without wrapping button labels', () => {
        const actions = read('../HeroActions.vue')
        expect(actions).toMatch(/flex-wrap:\s*wrap/)
        expect(actions).toMatch(/white-space:\s*nowrap/)
    })
})
