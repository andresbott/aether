// @vitest-environment node
// Node env, not jsdom: this spec compiles the stylesheet off disk rather than
// rendering, and under jsdom `import.meta.url` is not a file: URL.
import { describe, it, expect } from 'vitest'
import { fileURLToPath } from 'node:url'
import * as sass from 'sass-embedded'

/**
 * @primeuix's styled mode gives EVERY bottom drawer a fixed height:
 *
 *     .p-drawer-bottom .p-drawer { height: 10rem; }
 *     .p-drawer-bottom .p-drawer-content { height: 100%; }
 *
 * (see node_modules/@primeuix/styles/dist/drawer/index.mjs). At 10rem only about
 * two rows of TrackActionSheet / MobileMoreDrawer were visible and the rest sat
 * behind an unlabelled scroll. _main.scss lifts the cap for sheets that opt in
 * with `.app-bottom-sheet`; this pins that rule, since a Drawer renders in a
 * body-level overlay where no scoped style can reach it.
 *
 * The two components' own specs pin the other half — that the class lands on the
 * `.p-drawer` panel rather than the mask.
 */
const css = sass
    .compile(fileURLToPath(new URL('../_main.scss', import.meta.url)), { style: 'expanded' })
    .css.replace(/\/\*[\s\S]*?\*\//g, '')

/** The declaration block for `selector`, or undefined when there is no such rule. */
const block = (selector: string): string | undefined => {
    const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    return css.match(new RegExp(`(^|\\})\\s*${escaped}\\s*\\{([^}]*)\\}`))?.[2]
}

const PANEL = '.p-drawer-bottom .p-drawer.app-bottom-sheet'
const CONTENT = '.p-drawer-bottom .p-drawer.app-bottom-sheet .p-drawer-content'

describe('bottom sheets size to their content', () => {
    it('drops the theme fixed height for an opt-in bottom sheet', () => {
        const panel = block(PANEL)
        expect(panel).toBeTruthy()
        expect(panel).toMatch(/height:\s*auto/)
    })

    it('caps the panel against the viewport so a long sheet cannot cover the screen', () => {
        expect(block(PANEL)).toMatch(/max-height:\s*80dvh/)
    })

    it('lets the content box grow and scroll only past the cap', () => {
        const content = block(CONTENT)
        expect(content).toBeTruthy()
        // The theme pins this to 100%, which fights an auto-height panel.
        expect(content).toMatch(/height:\s*auto/)
        expect(content).toMatch(/overflow-y:\s*auto/)
    })

    // Specificity, not source order, has to decide this: PrimeVue injects its
    // styled-mode CSS at runtime, so a selector merely TYING with
    // `.p-drawer-bottom .p-drawer` (0,2,0) would leave the winner to injection
    // order. Repeating the position class puts ours at 0,3,0.
    it('out-specifies the theme rule rather than relying on injection order', () => {
        const classCount = (selector: string) => (selector.match(/\./g) ?? []).length
        expect(classCount(PANEL)).toBeGreaterThan(classCount('.p-drawer-bottom .p-drawer'))
        expect(classCount(CONTENT)).toBeGreaterThan(
            classCount('.p-drawer-bottom .p-drawer-content')
        )
    })
})
