// @vitest-environment node
// Node env, not jsdom: this spec reads the component's <style> block off disk
// rather than rendering. Scoped SFC styles are never applied by vue-test-utils,
// so no mounted test can see them — and the bug this pins is purely a cascade
// one (child-component rows reading tokens the host never overrode).
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

const read = (rel: string): string =>
    readFileSync(fileURLToPath(new URL(rel, import.meta.url)), 'utf8')

/** The <style> body with comments stripped (they name selectors and tokens). */
const styleBlock = (source: string): string => {
    const at = source.indexOf('<style')
    return source
        .slice(source.indexOf('>', at) + 1, source.lastIndexOf('</style>'))
        .replace(/\/\*[\s\S]*?\*\//g, '')
}

/** Declaration body of the first rule whose selector matches `pattern`. */
const ruleBody = (css: string, pattern: RegExp): string | null => {
    const rules = css.match(/[^{}]+\{[^{}]*\}/g) ?? []
    for (const rule of rules) {
        const [selector, body] = rule.split('{')
        if (pattern.test(selector.trim())) return body.replace('}', '')
    }
    return null
}

/** Every `--app-*` token the given components' styles actually read. */
const consumedTokens = (files: string[]): Set<string> => {
    const found = new Set<string>()
    for (const file of files) {
        for (const match of read(file).matchAll(/var\((--app-[a-z0-9-]+)/g)) {
            found.add(match[1])
        }
    }
    return found
}

const sheetCss = styleBlock(read('../PlayerSheet.vue'))

describe('PlayerSheet queue face token overrides', () => {
    const queueRule = ruleBody(sheetCss, /^\.sheet-queue$/)

    it('scopes an override block on .sheet-queue', () => {
        expect(queueRule).not.toBeNull()
        expect(queueRule).toMatch(/--app-text-primary\s*:/)
    })

    /**
     * The sheet paints itself on --app-player-bg (dark in BOTH themes), but the
     * queue rows it hosts are QueueBody's sidebar variant, written for the app
     * surface. In light theme --app-text-primary is near black: unremapped, a row
     * title sat at ~1.1:1 on the sheet. Each token the rows consume must be
     * remapped to something derived from the player palette.
     */
    it('remaps every surface token the reused queue rows consume', () => {
        const surfaceTokens = [...consumedTokens([
            '../QueueBody.vue',
            '../QueueRow.vue',
            '../../library/TrackFavoriteButton.vue'
        ])].filter(
            (token) =>
                // Layout metrics carry no colour, and the accent is intentionally
                // shared: it reads on the player background in both themes.
                !/^--app-(content-|rail-)/.test(token) && token !== '--app-accent'
        )
        expect(surfaceTokens.length).toBeGreaterThan(0)
        for (const token of surfaceTokens) {
            expect(queueRule, `${token} must be remapped for the sheet`).toContain(`${token}:`)
        }
    })

    it('derives the overrides from the player palette, never from app-surface tokens', () => {
        const declarations = (queueRule ?? '')
            .split(';')
            .map((line) => line.trim())
            .filter((line) => line.startsWith('--app-'))
        expect(declarations.length).toBeGreaterThan(0)
        for (const declaration of declarations) {
            const value = declaration.slice(declaration.indexOf(':') + 1)
            expect(value, declaration).toMatch(/--app-(player-text|player-dim|accent)\b/)
        }
    })
})

describe('PlayerSheet stacking order', () => {
    /**
     * PrimeVue stacks Toast and Drawer from `zIndex.modal` (1100 by default, see
     * @primevue/core/config). A sheet at or above that hid toasts fired while it
     * was open. It must still clear the app's own chrome (the mobile tab bar's
     * 100).
     */
    it('sits below PrimeVue overlays and above app chrome', () => {
        const body = ruleBody(sheetCss, /^\.player-sheet$/)
        expect(body).not.toBeNull()
        const zIndex = Number(/z-index:\s*(\d+)/.exec(body ?? '')?.[1])
        expect(zIndex).toBeGreaterThan(100)
        expect(zIndex).toBeLessThan(1100)
    })
})
