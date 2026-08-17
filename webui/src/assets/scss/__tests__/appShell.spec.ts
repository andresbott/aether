// @vitest-environment node
// jsdom resolves neither dvh nor `(pointer: coarse)`, and both regressions here
// are invisible under device emulation (no retractable URL bar, desktop-style
// scrollbars) — so pin the declarations off disk, same technique as the other
// style specs.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const read = (rel: string): string =>
    readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

const main = read('../_main.scss')
const rule = (selector: string, src = main): string | undefined =>
    src.match(new RegExp(`(^|\\n)${selector}\\s*\\{[^}]*\\}`))?.[0]

// The "no vh / no ungated scrollbar" checks below are searches for DECLARATIONS;
// the comments explaining why those declarations are gone name them verbatim, so
// strip comments first or the prose fails its own rule.
const code = (src: string): string =>
    src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '')

describe('app-shell height chain', () => {
    // A `100vh` box is the URL-BAR-HIDDEN viewport on mobile browsers: the
    // document outgrows the screen, the page scrolls, and the app's header ends
    // up under the URL bar with dead space below the shell.
    it('html is the one viewport-sized box, measured in dvh', () => {
        const html = rule('html')
        expect(html).toBeTruthy()
        expect(html).toMatch(/height:\s*100dvh/)
        expect(html).toMatch(/overflow:\s*hidden/)
    })

    it('html blocks page-level rubber band and pull-to-refresh', () => {
        expect(rule('html')).toMatch(/overscroll-behavior:\s*none/)
    })

    it('body and #app inherit that height instead of re-measuring the viewport', () => {
        expect(rule('body')).toMatch(/height:\s*100%/)
        expect(rule('#app')).toMatch(/height:\s*100%/)
    })

    it('no vh units anywhere in the global stylesheet', () => {
        expect(code(main)).not.toMatch(/\d(?:\.\d+)?vh\b/)
    })

    it('neither player shell re-measures the viewport in vh', () => {
        const layout = read('../../../layouts/PlayerLayout.vue')
        expect(code(layout)).not.toMatch(/\d(?:\.\d+)?vh\b/)
        expect(rule('\\.player-shell\\.mobile-shell', layout)).toMatch(/height:\s*100%/)
        expect(rule('\\.player-shell\\.desktop-shell', layout)).toMatch(/height:\s*100%/)
    })
})

describe('scrollbar chrome is pointer-gated', () => {
    // Styling ::-webkit-scrollbar at all opts mobile Chrome out of its
    // auto-hiding overlay scrollbars: they turn into permanently visible
    // classic bars that also claim layout width.
    const fine = main.match(/@media \(pointer: fine\)[\s\S]*?\n\}/)?.[0]
    const coarse = main.match(/@media \(pointer: coarse\)[\s\S]*?\n\}/)?.[0]

    it('styles scrollbars only where a mouse can grab them', () => {
        expect(fine).toBeTruthy()
        expect(fine).toContain('::-webkit-scrollbar')
        expect(fine).toContain('::-webkit-scrollbar-thumb')
    })

    it('hides them outright on touch', () => {
        expect(coarse).toBeTruthy()
        expect(coarse).toMatch(/scrollbar-width:\s*none/)
        expect(coarse).toMatch(/\*::-webkit-scrollbar\s*\{\s*display:\s*none/)
    })

    it('leaves no ungated scrollbar rule behind', () => {
        const ungated = code(main).replace(/@media \(pointer: (?:fine|coarse)\)[\s\S]*?\n\}/g, '')
        expect(ungated).not.toContain('::-webkit-scrollbar')
    })
})
