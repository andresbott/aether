// @vitest-environment node
// jsdom knows nothing of env(safe-area-inset-*); pin the three notch-safety
// declarations off disk (same technique as the other style specs).
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const read = (rel: string): string =>
    readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

describe('mobile chrome safe-area insets', () => {
    it('tab bar reserves the bottom inset', () => {
        const src = read('../MobileTabBar.vue')
        expect(src).toContain('padding-bottom: env(safe-area-inset-bottom)')
        expect(src).toContain('calc(var(--app-mobile-tabbar-height) + env(safe-area-inset-bottom))')
    })

    it('player sheet reserves top and bottom insets', () => {
        const src = read('../PlayerSheet.vue')
        expect(src).toContain('padding-top: env(safe-area-inset-top)')
        expect(src).toContain('padding-bottom: env(safe-area-inset-bottom)')
    })

    it('index.html opts into the insets with viewport-fit=cover', () => {
        const html = read('../../../../index.html')
        expect(html).toContain('viewport-fit=cover')
        expect(html).toContain('rel="manifest"')
    })
})
