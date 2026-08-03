// @vitest-environment node
// Node env, not jsdom: this spec reads SFCs off disk rather than rendering.
// Scoped SFC styles never apply under vue-test-utils, so a mounted test cannot
// catch any of these regressions — same reason PlayerControls.railStyles.spec.ts
// and ShortcutHelpOverlay.badgeStyles.spec.ts exist.
import { describe, it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'

/**
 * The three Library list views (Albums, Artists, and Discover in list layout) share
 * one gap above their column header, `--app-list-header-top`.
 *
 * The load-bearing part is WHERE it is applied. Discover's header is
 * `position: sticky`, so the gap has to live on the header element itself — as
 * padding on the scrolling container it would scroll away and the gap would
 * collapse the moment the list moved. Getting that wrong looks correct on load and
 * only breaks once you scroll, which is exactly the kind of thing that regresses
 * silently.
 */
const read = (path: string) =>
    readFileSync(fileURLToPath(new URL(`../${path}`, import.meta.url)), 'utf8')

const albumList = read('AlbumListView.vue')
const artistList = read('ArtistListView.vue')
const discoveryFeed = read('DiscoveryFeed.vue')

// The `.list-header` rule body from an SFC's style block.
function listHeaderRule(sfc: string, selector = '.list-header'): string {
    const escaped = selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
    const match = sfc.match(new RegExp(`${escaped}\\s*\\{([^}]*)\\}`))
    if (!match) throw new Error(`no ${selector} rule found`)
    return match[1]
}

describe('list header top gap', () => {
    it('all three views take the gap from the shared token', () => {
        for (const [name, sfc, selector] of [
            ['AlbumListView', albumList, '.list-header'],
            ['ArtistListView', artistList, '.list-header'],
            ['DiscoveryFeed', discoveryFeed, '.discovery-feed-list .list-header']
        ] as const) {
            expect(listHeaderRule(sfc, selector), name).toContain(
                'padding-top: var(--app-list-header-top)'
            )
        }
    })

    // The whole point of putting it on the header: a sticky header keeps its own
    // padding while scrolling, whereas the container's is left behind.
    it("Discover's gap is on the sticky element itself, not the scroll container", () => {
        const header = listHeaderRule(discoveryFeed, '.discovery-feed-list .list-header')
        expect(header).toContain('position: sticky')
        expect(header).toContain('padding-top: var(--app-list-header-top)')
        // The container must NOT also pad, or the two would stack into a double gap
        // that then halves on scroll.
        expect(listHeaderRule(discoveryFeed, '.discovery-feed-list')).toContain('padding-top: 0')
    })

    // The padded box is what masks rows sliding under the header, so it has to be
    // opaque — a transparent strip above the labels would show them through.
    it("Discover's sticky header is opaque", () => {
        expect(listHeaderRule(discoveryFeed, '.discovery-feed-list .list-header')).toMatch(
            /background:\s*var\(--app-/
        )
    })
})

describe('--app-list-header-top', () => {
    it('is defined once, as a dimension in :root', () => {
        const variables = read('../../assets/scss/_variables.scss')
        expect(variables).toContain('--app-list-header-top:')
        // Dimensions live in :root only — the dark-mode block overrides colours.
        expect(variables.match(/--app-list-header-top:/g)).toHaveLength(1)
    })
})
