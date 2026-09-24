import { describe, it, expect } from 'vitest'
import { loadIconSet, iconSvg } from '../iconset'
import { scanTokens, buildIconCss } from '../iconCss'
import { buildCatalogue } from '../catalogue'
import { DEFAULT_LIBRARY_ICON, SUGGESTED_LIBRARY_ICONS } from '../../../src/lib/libraryIcons'

const set = loadIconSet()

describe('iconSvg', () => {
    it('resolves the outline-rounded variant, snake or kebab', () => {
        const svg = iconSvg(set, 'queue_music', 'outline')
        expect(svg).toMatch(/^<svg xmlns="http:\/\/www\.w3\.org\/2000\/svg" viewBox="0 0 24 24">/)
        expect(iconSvg(set, 'queue-music', 'outline')).toBe(svg)
    })
    it('falls back to the rounded form when Google ships no outline', () => {
        expect(iconSvg(set, 'expand-more', 'outline')).toBe(iconSvg(set, 'expand-more', 'fill'))
    })
    it('tells outline and fill apart', () => {
        expect(iconSvg(set, 'favorite', 'outline')).not.toBe(iconSvg(set, 'favorite', 'fill'))
    })
    it('returns undefined for an unknown name', () => {
        expect(iconSvg(set, 'no-such-icon-xyz', 'outline')).toBeUndefined()
    })
})

describe('scanTokens', () => {
    it('finds ms- and mso- class tokens', () => {
        const src = `<i class="ms-play-arrow"></i> icon: 'mso-favorite' :class="x ? 'ms-pause' : ''"`
        expect([...scanTokens(src, 'a.vue')].sort()).toEqual(['ms-pause', 'ms-play-arrow', 'mso-favorite'])
    })
    it('ignores look-alikes inside other words and custom properties', () => {
        expect(scanTokens('items-center rooms-list --ms-svg: x; atoms-x', 'a.vue').size).toBe(0)
    })
    it('rejects a dynamically built icon class', () => {
        expect(() => scanTokens('const c = `ms-${name}`', 'a.ts')).toThrow(/a\.ts/)
        expect(() => scanTokens("const c = 'mso-' + name", 'a.ts')).toThrow(/dynamic/)
    })
})

describe('buildIconCss', () => {
    const css = buildIconCss(set, ['ms-play-arrow', 'mso-favorite'])
    it('gives every icon class and .app-icon the shared mask rule', () => {
        const common = css.split('\n')[0]
        expect(common).toContain('.app-icon')
        expect(common).toContain('.ms-play-arrow')
        expect(common).toContain('.mso-favorite')
        expect(common).toContain('mask:var(--ms-svg)')
        expect(common).toContain('background-color:currentColor')
    })
    it('inlines each icon as a data URI', () => {
        expect(css).toMatch(/\.ms-play-arrow\{--ms-svg:url\("data:image\/svg\+xml,%3Csvg/)
    })
    it('draws ms- filled and mso- outlined', () => {
        const both = buildIconCss(set, ['ms-favorite', 'mso-favorite'])
        const url = (svg: string | undefined) => `url("data:image/svg+xml,${encodeURIComponent(svg!)}")`
        expect(both).toContain(`.ms-favorite{--ms-svg:${url(iconSvg(set, 'favorite', 'fill'))}}`)
        expect(both).toContain(`.mso-favorite{--ms-svg:${url(iconSvg(set, 'favorite', 'outline'))}}`)
    })
    it('keeps PrimeVue loading icons spinning', () => {
        expect(css).toMatch(/\.icon-spin,\.pi-spin\{animation:icon-spin/)
        expect(css).toContain('@keyframes icon-spin')
    })
    it('fails loudly on an unknown icon', () => {
        expect(() => buildIconCss(set, ['ms-not-an-icon-xyz'])).toThrow(/ms-not-an-icon-xyz/)
    })
})

describe('buildCatalogue', () => {
    const cat = buildCatalogue(set)
    it('lists every Rounded icon once, snake_case, sorted', () => {
        expect(cat.names.length).toBeGreaterThan(3500)
        expect(new Set(cat.names).size).toBe(cat.names.length)
        expect([...cat.names].sort()).toEqual(cat.names)
        for (const n of cat.names) expect(n).toMatch(/^[a-z0-9]+(_[a-z0-9]+)*$/)
        expect(cat.names).toContain('queue_music')
        expect(cat.names.some((n) => n.endsWith('_outline'))).toBe(false)
    })
    it('draws library icons filled, like the ms- classes', () => {
        expect(cat.svgs.get('favorite')).toBe(iconSvg(set, 'favorite', 'fill'))
    })
    it('has an SVG for every name, including the default', () => {
        for (const n of cat.names) expect(cat.svgs.has(n)).toBe(true)
        expect(cat.svgs.has(DEFAULT_LIBRARY_ICON)).toBe(true)
    })
    it('covers every suggested icon, each listed once, all fitting on one page', () => {
        expect(SUGGESTED_LIBRARY_ICONS.filter((n) => !cat.svgs.has(n))).toEqual([])
        expect(new Set(SUGGESTED_LIBRARY_ICONS).size).toBe(SUGGESTED_LIBRARY_ICONS.length)
        expect(SUGGESTED_LIBRARY_ICONS.length).toBeLessThanOrEqual(120) // MAX_ICON_RESULTS
    })
})
