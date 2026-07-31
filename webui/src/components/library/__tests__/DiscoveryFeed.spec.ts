import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import { ref, computed, nextTick } from 'vue'
import type { DiscoveryFeedEntry } from '@/types/subsonic'

// Ported from the deleted DiscoveryView.spec.ts: the feed BODY behaviour moved into
// this component when the standalone /discover view was folded into LibraryView's
// Discover tab. The header concerns that lived alongside it (title, count summary,
// layout toggle) are now covered by LibraryView.spec.ts.

const feed = {
    items: ref<DiscoveryFeedEntry[]>([]),
    isLoading: ref(false),
    isError: ref(false),
    hasNextPage: ref(false),
    isFetchingNextPage: ref(false),
    fetchNextPage: vi.fn()
}
vi.mock('@/composables/useDiscovery', () => ({
    DISCOVERY_PAGE_SIZE: 48,
    useDiscoveryFeed: () => ({
        items: computed(() => feed.items.value),
        isLoading: computed(() => feed.isLoading.value),
        isError: computed(() => feed.isError.value),
        hasNextPage: computed(() => feed.hasNextPage.value),
        isFetchingNextPage: computed(() => feed.isFetchingNextPage.value),
        fetchNextPage: feed.fetchNextPage
    })
}))

vi.mock('@/components/library/DiscoveryFeedItem.vue', () => ({
    default: {
        name: 'DiscoveryFeedItem',
        props: ['entry', 'layout'],
        template: '<div class="stub-feed-item" :data-layout="layout" :data-rank="entry.rank" />'
    }
}))

import DiscoveryFeed from '@/components/library/DiscoveryFeed.vue'

const albumEntry = (rank: number): DiscoveryFeedEntry => ({
    type: 'album',
    rank,
    reason: 'favorite',
    album: { id: `al-${rank}`, name: `Album ${rank}`, rank, reason: 'favorite' }
})

const stubs = { RouterLink: { template: '<a><slot /></a>' } }
const mountFeed = (layout: 'grid' | 'list' = 'grid') =>
    mount(DiscoveryFeed, {
        props: { layout },
        global: { plugins: [PrimeVue], directives: { tooltip: {} }, stubs }
    })

beforeEach(() => {
    feed.fetchNextPage.mockReset()
    feed.items.value = []
    feed.isLoading.value = false
    feed.isError.value = false
    feed.hasNextPage.value = false
    feed.isFetchingNextPage.value = false
})

describe('DiscoveryFeed', () => {
    it('renders one feed item per entry', () => {
        feed.items.value = [albumEntry(0), albumEntry(1), albumEntry(2)]
        expect(mountFeed().findAll('.stub-feed-item')).toHaveLength(3)
    })

    it('renders items in rank order', () => {
        feed.items.value = [albumEntry(0), albumEntry(1), albumEntry(2)]
        const ranks = mountFeed()
            .findAll('.stub-feed-item')
            .map((n) => n.attributes('data-rank'))
        expect(ranks).toEqual(['0', '1', '2'])
    })

    it('passes the layout prop through to each item', () => {
        feed.items.value = [albumEntry(0)]
        expect(mountFeed('grid').find('.stub-feed-item').attributes('data-layout')).toBe('grid')
        expect(mountFeed('list').find('.stub-feed-item').attributes('data-layout')).toBe('list')
    })

    it('shows a loading state', () => {
        feed.isLoading.value = true
        expect(mountFeed().find('.pi-spinner').exists()).toBe(true)
    })

    it('shows an error state', () => {
        feed.isError.value = true
        expect(mountFeed().text()).toContain('Could not load')
    })

    it('hides the error when loading', () => {
        feed.isError.value = true
        feed.isLoading.value = true
        expect(mountFeed().text()).not.toContain('Could not load')
    })

    it('shows an empty state when the feed is empty', () => {
        expect(mountFeed().text()).toContain('Nothing here yet')
    })

    it('does not show the empty state while loading', () => {
        feed.isLoading.value = true
        expect(mountFeed().text()).not.toContain('Nothing here yet')
    })

    it('hides the feed during the initial loading state', () => {
        feed.isLoading.value = true
        expect(mountFeed().find('.discovery-feed').exists()).toBe(false)
    })

    // There is no manual reshuffle: the feed's seed is the 12-hour window and
    // nothing else, so it rolls on its own and cannot be nudged by hand.
    it('offers no refresh action', () => {
        feed.items.value = [albumEntry(0)]
        const w = mountFeed()
        expect(w.find('.discovery-refresh').exists()).toBe(false)
        expect(w.find('.pi-refresh').exists()).toBe(false)
    })

    it('renders the sentinel only while more pages remain', () => {
        feed.items.value = [albumEntry(0)]
        expect(mountFeed().find('.discovery-sentinel').exists()).toBe(false)
        feed.hasNextPage.value = true
        expect(mountFeed().find('.discovery-sentinel').exists()).toBe(true)
    })

    it('keys feed items by entity id so a rank shift moves rather than recreates them', async () => {
        // Build entries with fixed ids and different ranks.
        const entryA: DiscoveryFeedEntry = {
            type: 'album',
            rank: 0,
            reason: 'favorite',
            album: { id: 'al-100', name: 'Album A', rank: 0, reason: 'favorite' }
        }
        const entryB: DiscoveryFeedEntry = {
            type: 'album',
            rank: 1,
            reason: 'favorite',
            album: { id: 'al-200', name: 'Album B', rank: 1, reason: 'favorite' }
        }
        feed.items.value = [entryA, entryB]
        const w = mountFeed()
        const firstBefore = w.findAll('.stub-feed-item')[0].element

        // The SAME entities, both SHIFTED to new ranks — not swapped. A swap would
        // leave the set of rank-derived keys unchanged ({0,1} either way), so Vue
        // would reuse the nodes even with a rank key and the test would prove
        // nothing. Shifting to ranks nothing previously held is what distinguishes
        // an identity key from a positional one: under a rank key every key is new,
        // so every node is destroyed and recreated.
        entryA.rank = 7
        entryB.rank = 8
        feed.items.value = [entryA, entryB]
        await nextTick()

        const nodes = w.findAll('.stub-feed-item').map((n) => n.element)
        expect(nodes).toContain(firstBefore)
    })
})

describe('DiscoveryFeed intersection observer', () => {
    let observes = 0
    let disconnects = 0

    beforeEach(() => {
        observes = 0
        disconnects = 0
        // jsdom has no IntersectionObserver, and the component guards on its absence —
        // so injecting a counting fake is what makes the lifecycle observable.
        ;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver = class {
            constructor(_cb: unknown) {}
            observe(): void {
                observes++
            }
            disconnect(): void {
                disconnects++
            }
            unobserve(): void {}
            takeRecords(): [] {
                return []
            }
            root = null
            rootMargin = ''
            thresholds = []
        }
        feed.items.value = [albumEntry(0)]
        feed.hasNextPage.value = true
    })

    it('observes the sentinel while more pages remain', async () => {
        mountFeed()
        await nextTick()
        expect(observes).toBeGreaterThan(0)
    })

    // A live observer after teardown keeps a reference to the unmounted component.
    it('disconnects the observer on unmount', async () => {
        const w = mountFeed()
        await nextTick()
        const before = disconnects
        w.unmount()
        expect(disconnects).toBeGreaterThan(before)
    })
})
