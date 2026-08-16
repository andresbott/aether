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
 * The heading ("Queue" + track summary + actions) belongs to the QUEUE panel
 * and scrolls in with it — there is no fixed bar over the player face, so
 * nothing to fade and no height to hold constant across the switch. It is a
 * non-shrinking row above the list scroller, and the topmost surface on its
 * panel, so it reserves the top inset itself.
 */
describe('the queue heading rides in the queue panel', () => {
    const heading = ruleBodies(/^\s*\.queue-heading\s*$/m).join('\n')

    it('is a non-shrinking row above the list scroller', () => {
        expect(heading).toMatch(/display:\s*flex/)
        expect(heading).toMatch(/align-items:\s*center/)
        expect(heading).toMatch(/flex-shrink:\s*0/)
    })

    it('reserves the top safe-area inset', () => {
        expect(heading).toMatch(/env\(safe-area-inset-top\)/)
    })

    it('ellipsizes the summary instead of widening the row', () => {
        expect(ruleBodies(/\.queue-heading-text/).join('\n')).toMatch(/min-width:\s*0/)
        expect(ruleBodies(/\.queue-heading-summary/).join('\n')).toMatch(
            /text-overflow:\s*ellipsis/
        )
    })

    // The predecessor was a ContentScaffold header faded in by a `queue-up`
    // class; both are gone, and a leftover rule would style nothing.
    it('keeps no scaffold or fade machinery', () => {
        expect(styles).not.toMatch(/scaffold-/)
        expect(styles).not.toMatch(/queue-up/)
    })
})

/**
 * With no header above it and no mini player below (hidden on this route), the
 * face is the entire screen: it reserves both insets, or the status bar sits on
 * the nav chevron that replaced the hamburger.
 */
describe('the bare player face', () => {
    it('reserves both safe-area insets', () => {
        const face = ruleBodies(/^\s*\.play-face\s*$/m).join('\n')
        expect(face).toMatch(/env\(safe-area-inset-top\)/)
        expect(face).toMatch(/env\(safe-area-inset-bottom\)/)
    })

    it('never squashes the nav chevron — a short screen shrinks the artwork', () => {
        expect(ruleBodies(/^\s*\.play-nav-hint\s*$/m).join('\n')).toMatch(/flex-shrink:\s*0/)
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
