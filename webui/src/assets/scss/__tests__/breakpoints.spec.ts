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
