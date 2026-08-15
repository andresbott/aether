// @vitest-environment node
// jsdom knows nothing of env(safe-area-inset-*); pin the three notch-safety
// declarations off disk (same technique as the other style specs).
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const read = (rel: string): string =>
    readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

describe('mobile chrome safe-area insets', () => {
    it('mini player (the bottom-most mobile chrome) reserves the bottom inset', () => {
        const src = read('../MiniPlayer.vue')
        expect(src).toContain('padding: 0 0.75rem env(safe-area-inset-bottom)')
        expect(src).toContain('calc(var(--app-mini-player-height) + env(safe-area-inset-bottom))')
    })

    it('nav drawer pads the notch insets (left edge in landscape)', () => {
        const scss = read('../../../assets/scss/_main.scss')
        const rule = scss.match(/\.p-drawer-left \.p-drawer\.mobile-nav-drawer\s*\{[^}]*\}/)?.[0]
        expect(rule).toBeTruthy()
        expect(rule).toContain('padding-top: env(safe-area-inset-top)')
        expect(rule).toContain('padding-bottom: env(safe-area-inset-bottom)')
        expect(rule).toContain('padding-left: env(safe-area-inset-left)')
    })

    // The mini player is hidden on the Now Playing route, so the play view is
    // the bottom-most surface there and reserves the inset itself — on both
    // of its snap panels (the play face and the queue list).
    it('the play view reserves the bottom inset on both panels', () => {
        const src = read('../MobilePlayView.vue')
        expect(src).toContain('padding: 0 1.5rem calc(0.5rem + env(safe-area-inset-bottom))')
        expect(src).toContain('padding-bottom: env(safe-area-inset-bottom)')
    })

    it('index.html opts into the insets with viewport-fit=cover', () => {
        const html = read('../../../../index.html')
        expect(html).toContain('viewport-fit=cover')
        expect(html).toContain('rel="manifest"')
    })
})
