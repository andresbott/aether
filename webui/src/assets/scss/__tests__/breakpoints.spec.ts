// @vitest-environment node
// The breakpoints exist twice — TS (useViewport's matchMedia queries) and SCSS
// (media queries in _variables.scss and component styles). This spec is the
// contract that the two never drift.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { BP_PHONE_MAX, BP_DESKTOP_MIN } from '../../../lib/breakpoints'

const scss = readFileSync(
    fileURLToPath(new URL('../_variables.scss', import.meta.url)),
    'utf8'
)

function scssVar(name: string): number {
    const m = scss.match(new RegExp(`\\$${name}:\\s*(\\d+)px`))
    if (!m) throw new Error(`$${name} not found in _variables.scss`)
    return Number(m[1])
}

describe('breakpoint tokens', () => {
    it('SCSS $bp-phone-max equals TS BP_PHONE_MAX', () => {
        expect(scssVar('bp-phone-max')).toBe(BP_PHONE_MAX)
    })

    it('SCSS $bp-desktop-min equals TS BP_DESKTOP_MIN', () => {
        expect(scssVar('bp-desktop-min')).toBe(BP_DESKTOP_MIN)
    })
})

describe('phone token overrides', () => {
    // One media query in _variables.scss re-tokens the phone layout; every
    // view on the recipes adapts from these two lines alone (spec §3.1).
    const block = scss.match(
        /@media \(max-width: \(\$bp-phone-max - 0\.02px\)\)[^{]*\{[\s\S]*?\n\}/
    )?.[0]

    it('has a phone-width :root override block', () => {
        expect(block).toBeTruthy()
    })

    it('shrinks the content gutter on phones', () => {
        expect(block).toContain('--app-content-gutter: 0.75rem')
    })

    it('collapses the rail clearance on phones', () => {
        expect(block).toContain('--app-rail-clearance: 0px')
    })
})

describe('phone dialog rule', () => {
    const main = readFileSync(
        fileURLToPath(new URL('../_main.scss', import.meta.url)),
        'utf8'
    )
    const blocks = main.match(
        /@media \(max-width: \(variables\.\$bp-phone-max - 0\.02px\)\)[\s\S]*?\n\}/g
    )
    // The dialog rule is the last media query block with this breakpoint
    const block = blocks?.[blocks.length - 1]

    it('makes form dialogs full-screen on phones, sparing confirm popups', () => {
        expect(block).toBeTruthy()
        expect(block).toContain('.p-dialog:not(.p-confirmdialog)')
        expect(block).toContain('100dvh')
    })
})
