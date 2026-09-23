import { describe, it, expect } from 'vitest'
import { readFileSync, readdirSync } from 'node:fs'
import { dirname, join } from 'node:path'

// Icons are Material Symbols, self-hosted (build/material-symbols). Nothing may
// bring PrimeIcons, or an icon CDN, back.
const thisFile = __filename
const src = dirname(__filename).replace('/__tests__', '')
const pkg = JSON.parse(readFileSync(join(src, '..', 'package.json'), 'utf8'))
const sources = (readdirSync(src, { recursive: true }) as string[])
    .filter((f) => /\.(vue|ts|scss|css)$/.test(f))
    .map((f) => join(src, f))
    .filter((f) => f !== thisFile)

describe('icon stack', () => {
    it('does not depend on primeicons', () => {
        expect({ ...pkg.dependencies, ...pkg.devDependencies }).not.toHaveProperty('primeicons')
    })
    it('uses no PrimeIcons class or icon CDN anywhere in src', () => {
        const offenders = sources.filter((f) =>
            /primeicons|pi pi-|(^|[^\w-])pi-[a-z]|\.pi\b|fonts\.(googleapis|gstatic)\.com|api\.iconify\.design/.test(
                readFileSync(f, 'utf8')
            )
        )
        expect(offenders).toEqual([])
    })
})
