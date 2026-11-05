// @vitest-environment node
// Dark-mode legibility contract. Two sources paint dark text: the app's own
// tokens (_variables.scss, `.dark-mode`) and PrimeVue's preset (theme.js), whose
// dark surface scale doubles as its ink scale — Aura reads surface.400 as muted
// text, placeholders and float labels. The preset once shipped a scale that was
// dark from 100 on, and every muted PrimeVue ink went near-black on near-black.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import CustomTheme from '../../../theme'

const scss = readFileSync(fileURLToPath(new URL('../_variables.scss', import.meta.url)), 'utf8')
const darkBlock = scss.slice(scss.indexOf('.dark-mode {'))

function darkToken(name: string): string {
    const m = darkBlock.match(new RegExp(`--${name}:\\s*(#[0-9a-fA-F]{6})\\s*;`))
    if (!m) throw new Error(`--${name} (hex) not found in the .dark-mode block`)
    return m[1]
}

function luminance(hex: string): number {
    const c = [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16) / 255)
    const [r, g, b] = c.map((v) => (v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4))
    return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function contrast(a: string, b: string): number {
    const [hi, lo] = [luminance(a), luminance(b)].sort((x, y) => y - x)
    return (hi + 0.05) / (lo + 0.05)
}

const AA = 4.5

type Preset = { semantic: { colorScheme: { dark: { surface: Record<string, string> } } } }
const surface = (CustomTheme as unknown as Preset).semantic.colorScheme.dark.surface
const STEPS = ['0', '50', '100', '200', '300', '400', '500', '600', '700', '800', '900', '950']

describe('dark theme contrast', () => {
    const grounds = ['app-background', 'app-surface', 'app-surface-2', 'app-bg-subtle']

    it.each(grounds)('secondary text is AA on --%s', (ground) => {
        expect(contrast(darkToken('app-text-secondary'), darkToken(ground))).toBeGreaterThanOrEqual(
            AA
        )
    })

    it('dim navigation text is AA on the navigation rail', () => {
        expect(
            contrast(darkToken('app-nav-text-dim'), darkToken('app-nav-bg'))
        ).toBeGreaterThanOrEqual(AA)
    })

    it('dim player text is AA on the player bar', () => {
        expect(
            contrast(darkToken('app-player-dim'), darkToken('app-player-bg'))
        ).toBeGreaterThanOrEqual(AA)
    })

    it('the PrimeVue dark surface scale runs light to dark', () => {
        const lums = STEPS.map((s) => luminance(surface[s]))
        for (let i = 1; i < lums.length; i++) expect(lums[i]).toBeLessThan(lums[i - 1])
    })

    it.each(['800', '900', '950'])(
        'PrimeVue muted ink (surface.400) is AA on surface.%s',
        (step) => {
            expect(contrast(surface['400'], surface[step])).toBeGreaterThanOrEqual(AA)
        }
    )

    it('PrimeVue panels and fields sit on the app layers', () => {
        expect(surface['900']).toBe(darkToken('app-surface'))
        expect(surface['950']).toBe(darkToken('app-background'))
        expect(surface['400']).toBe(darkToken('app-text-secondary'))
    })
})
