// @vitest-environment node
// The right-hand clearance of a main content view
// (docs/architecture/main-content-view-layout.md §4–5):
//
//   1. Only a view that draws the alphabet rail keeps the rail's slot clear.
//      The clearance is 0 everywhere else, and it is read only by the scaffold,
//      which turns it on for its header and body together, and by the bodies
//      that draw the rail.
//   2. A scrolling body reserves nothing for its own scrollbar: its stable
//      gutter already holds it. Only a frame outside the scroller (a header
//      above it) makes room for that scrollbar, and only once.
//
// Scoped styles never apply under vue-test-utils, so both are pinned off disk.
import { describe, it, expect } from 'vitest'
import { readdirSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

// webui/src, three levels up from here.
const srcDir = fileURLToPath(new URL('../../../', import.meta.url))

// Every stylesheet and component; specs and their fixtures are out.
const styleSources = (readdirSync(srcDir, { recursive: true }) as string[]).filter(
    (rel) => /\.(vue|scss)$/.test(rel) && !rel.split('/').includes('__tests__')
)

// The declarations of a source, comments dropped (they are free to name the
// tokens): a component's <style> blocks, or a whole stylesheet.
function styleOf(rel: string): string {
    const text = readFileSync(join(srcDir, rel), 'utf8')
    const css = rel.endsWith('.vue')
        ? [...text.matchAll(/<style[^>]*>([\s\S]*?)<\/style>/g)].map((m) => m[1]).join('\n')
        : text
    return css.replace(/\/\*[\s\S]*?\*\//g, ' ').replace(/^\s*\/\/.*$/gm, ' ')
}

// The innermost rules of a style text: those inside a media query come out on
// their own, with the query's text in front of their selector.
const rules = (css: string): Array<{ selector: string; body: string }> =>
    [...css.matchAll(/([^{}]+)\{([^{}]*)\}/g)].map((m) => ({ selector: m[1].trim(), body: m[2] }))

describe('the rail clearance', () => {
    it('reserves nothing by default', () => {
        const root = styleOf('assets/scss/_variables.scss').match(/:root\s*\{[\s\S]*?\n\}/)?.[0]
        expect(root).toMatch(/--app-rail-clearance:\s*0px;/)
    })

    // The content's own gutter spaces it from the rail, as from the left edge,
    // so the slot is the rail's width and nothing more.
    it('keeps a slot exactly as wide as the rail the bodies draw', () => {
        const slot = styleOf('assets/scss/_variables.scss').match(
            /--app-rail-slot:\s*([\d.]+rem);/
        )?.[1]
        expect(slot).toBeTruthy()
        for (const body of [
            'components/library/AlbumListView.vue',
            'components/library/ArtistListView.vue',
            'components/library/VirtualCardGrid.vue'
        ]) {
            const rail = rules(styleOf(body)).find(({ selector }) =>
                selector.includes('.alphabet-rail')
            )
            expect(rail?.body, body).toMatch(new RegExp(`width:\\s*${slot!.replace('.', '\\.')};`))
        }
    })

    it('is set to the slot by a scaffold whose body draws the rail', () => {
        expect(styleOf('components/layout/ContentScaffold.vue')).toMatch(
            /\.content-scaffold--rail\s*\{[^}]*--app-rail-clearance:\s*var\(--app-rail-slot\)/
        )
    })

    // An exact list, so a scan that finds nothing fails too. A new body that
    // draws the rail joins it; one that does not has no use for the clearance.
    it('is read only by the scaffold and the bodies that draw the rail', () => {
        const readers = styleSources.filter((rel) => styleOf(rel).includes('--app-rail-clearance'))
        expect(readers.sort()).toEqual(
            [
                'assets/scss/_variables.scss',
                'components/layout/ContentScaffold.vue',
                'components/library/AlbumListView.vue',
                'components/library/ArtistListView.vue',
                'components/library/VirtualCardGrid.vue'
            ].sort()
        )
    })
})

describe('the scrollbar clearance', () => {
    it('never counts a scrollbar twice', () => {
        const doubled = styleSources.filter((rel) => /\d\s*\*\s*var\(--sb-w/.test(styleOf(rel)))
        expect(doubled).toEqual([])
    })

    // The boxes whose width already leaves the scrollbar out: a scroller with a
    // stable gutter, and the content box of a VirtualScroller.
    const scrollingBodies = styleSources.flatMap((rel) =>
        rules(styleOf(rel))
            .filter(
                ({ selector, body }) =>
                    /scrollbar-gutter:\s*stable/.test(body) ||
                    selector.includes('.p-virtualscroller-content')
            )
            .map((rule) => ({ ...rule, at: `src/${rel} ${rule.selector}` }))
    )

    it('is not taken again inside a scrolling body', () => {
        const offenders = scrollingBodies
            .filter(({ body }) => body.includes('--sb-w'))
            .map((r) => r.at)
        expect(offenders).toEqual([])
    })

    it('finds the scrolling bodies, so a pass is not for lack of looking', () => {
        const at = scrollingBodies.map((r) => r.at)
        expect(at).toContain('src/components/library/DiscoveryFeed.vue .discovery-scroll')
        expect(at.some((a) => a.startsWith('src/components/library/AlbumListView.vue '))).toBe(true)
    })
})
