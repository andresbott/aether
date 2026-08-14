// @vitest-environment node
// Node env, not jsdom: this spec reads the component's <style> block off disk
// rather than rendering. Scoped SFC styles are never applied by vue-test-utils,
// so no mounted test can see them.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

/**
 * On a tall narrow screen the player face must anchor the seek bar and
 * transport to the bottom of the viewport (where thumbs are), with the cover
 * art + title/artist centered as one group in the spare height — not center
 * the whole stack so the controls float mid-screen. That is done by leaving
 * the face's main axis unjustified and bracketing the group with auto
 * margins: one above the art, one below the meta.
 */
const styles = (() => {
    const source = readFileSync(
        fileURLToPath(new URL('../MobilePlayView.vue', import.meta.url)),
        'utf8'
    )
    const at = source.indexOf('<style')
    return (
        source
            .slice(source.indexOf('>', at) + 1, source.lastIndexOf('</style>'))
            // Comments are stripped: everything between `}` and `{` is read as
            // a selector below, and a comment sitting there would be captured
            // along with it.
            .replace(/\/\*[\s\S]*?\*\//g, '')
    )
})()

/** Declaration bodies of every rule whose selector matches `pattern`. */
function ruleBodies(pattern: RegExp): string[] {
    const bodies: string[] = []
    for (const match of styles.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
        if (pattern.test(match[1])) bodies.push(match[2])
    }
    return bodies
}

/**
 * The queue is revealed by swiping the play face up: the two panels live in a
 * vertical scroller with mandatory snap points, so the view always rests on a
 * whole face, never in between. The way back is NOT native scroll chaining
 * (a chained drag hands the snap container no fling momentum, so it settles
 * straight back on the queue): the list contains its overscroll and the
 * component's touch handler owns the queue → face switch.
 */
describe('swipe-up queue reveal', () => {
    it('the panel container is a vertical scroller with mandatory snap, contained', () => {
        const panels = ruleBodies(/^\s*\.play-panels\s*$/m).join('\n')
        expect(panels).toMatch(/overflow-y:\s*auto/)
        expect(panels).toMatch(/scroll-snap-type:\s*y mandatory/)
        // Never chain into the page — an overshot swipe-back would trigger
        // pull-to-refresh on Android.
        expect(panels).toMatch(/overscroll-behavior-y:\s*contain/)
    })

    it('each panel fills the view and snaps to its start edge', () => {
        const panel = ruleBodies(/^\s*\.play-panel\s*$/m).join('\n')
        expect(panel).toMatch(/height:\s*100%/)
        expect(panel).toMatch(/scroll-snap-align:\s*start/)
    })

    it('the queue list scrolls inside its panel and contains its overscroll', () => {
        const list = ruleBodies(/^\s*\.play-queue-list\s*$/m).join('\n')
        expect(list).toMatch(/overflow-y:\s*auto/)
        // Both scrollers in the panel — the list and the QueueBody scroller
        // inside it — must be contained, or a hard fling to the list's top
        // yanks the player face in by accident.
        const contained = ruleBodies(/\.play-queue-list\s*:deep\(\.queue-body\)/).join('\n')
        expect(contained).toMatch(/overscroll-behavior-y:\s*contain/)
    })
})

/**
 * The queue heading ("Queue" + track-summary stack + actions) is rendered in
 * BOTH panel states, so the header's height comes from content that never
 * changes — and the switch is a fade (opacity), revealed mid-swipe rather
 * than popping in after the snap settles. `visibility` keeps the hidden
 * heading out of the accessibility tree and off the tap surface while it
 * still occupies layout.
 */
describe('constant header height across the panel switch', () => {
    it('the header is one non-wrapping, center-aligned row', () => {
        const inner = ruleBodies(/:deep\(\.scaffold-header-inner\)/).join('\n')
        expect(inner).toMatch(/flex-wrap:\s*nowrap/)
        // The stacked heading has no single baseline for the buttons.
        expect(inner).toMatch(/align-items:\s*center/)
    })

    it('the summary stacks under the title and ellipsizes on overflow', () => {
        const title = ruleBodies(/^\s*\.mobile-play-view\s+:deep\(\.scaffold-title\)\s*$/m).join(
            '\n'
        )
        expect(title).toMatch(/flex-direction:\s*column/)
        // The scaffold's 12rem title floor would force an overflow on phones
        // once the actions sit beside it; "Queue" never needs the floor.
        expect(title).toMatch(/min-width:\s*0/)
        const summary = ruleBodies(/:deep\(\.scaffold-summary\)/).join('\n')
        expect(summary).toMatch(/flex-basis:\s*auto/)
        expect(summary).toMatch(/text-overflow:\s*ellipsis/)
    })

    it('the heading hides by opacity + delayed visibility, never by layout', () => {
        const hidden = ruleBodies(/^\s*\.mobile-play-view\s+:deep\(\.scaffold-title\),/m).join('\n')
        expect(hidden).toMatch(/opacity:\s*0/)
        expect(hidden).toMatch(/visibility:\s*hidden/)
        expect(hidden).toMatch(/transition:[\s\S]*opacity/)
        expect(hidden).not.toMatch(/display:\s*none/)
    })

    it('the queue-up state fades the heading in', () => {
        const shown = ruleBodies(/\.queue-up/).join('\n')
        expect(shown).toMatch(/opacity:\s*1/)
        expect(shown).toMatch(/visibility:\s*visible/)
    })
})

describe('transport anchoring on tall screens', () => {
    it('does not center the face stack — that floats the transport mid-screen', () => {
        const face = ruleBodies(/^\s*\.play-face\s*$/m).join('\n')
        expect(face).not.toMatch(/justify-content:\s*center/)
    })

    it('brackets the art + title/artist group with auto margins', () => {
        const art = ruleBodies(/^\s*\.play-art\s*$/m).join('\n')
        expect(art).toMatch(/margin-top:\s*auto/)
        const meta = ruleBodies(/^\s*\.play-meta\s*$/m).join('\n')
        expect(meta).toMatch(/margin-bottom:\s*auto/)
    })
})
