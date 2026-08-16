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

    // The browse page is shown with an empty queue (HomeView redirects there),
    // and then nothing is docked beneath it — so its own body reserves the
    // home-indicator inset.
    it('the browse page reserves the bottom inset under its shelves', () => {
        const src = read('../../../views/MobileBrowseView.vue')
        expect(src).toContain('padding-bottom: calc(1rem + env(safe-area-inset-bottom))')
    })

    // The mini player is hidden on the Now Playing route, so the play view is
    // the bottom-most surface there and reserves the inset itself — on both
    // of its snap panels (the play face and the queue list).
    it('the play view reserves the bottom inset on both panels', () => {
        const src = read('../MobilePlayView.vue')
        expect(src).toContain('calc(0.5rem + env(safe-area-inset-bottom))')
        expect(src).toContain('padding-bottom: env(safe-area-inset-bottom)')
    })

    // It has no scaffold header either, so both panels are the TOPMOST surface
    // too: the face carries the nav chevron where the status bar would land,
    // and the queue heading sits at its panel's top edge.
    it('the play view reserves the top inset on both panels', () => {
        const src = read('../MobilePlayView.vue')
        expect(src).toContain('padding: calc(0.25rem + env(safe-area-inset-top)) 1.5rem')
        expect(src).toContain(
            'padding: calc(0.5rem + env(safe-area-inset-top)) var(--app-content-gutter) 0.5rem'
        )
    })

    it('index.html opts into the insets with viewport-fit=cover', () => {
        const html = read('../../../../index.html')
        expect(html).toContain('viewport-fit=cover')
        expect(html).toContain('rel="manifest"')
    })
})
