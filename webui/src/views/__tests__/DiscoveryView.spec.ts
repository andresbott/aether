import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import { ref, computed, nextTick } from 'vue'
import type { DiscoveryFeedEntry } from '@/types/subsonic'

const replaceSpy = vi.fn()
const route = { query: {} as Record<string, unknown> }
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace: replaceSpy, push: vi.fn() })
}))

const feed = {
    items: ref<DiscoveryFeedEntry[]>([]),
    isLoading: ref(false),
    isError: ref(false),
    hasNextPage: ref(false),
    isFetchingNextPage: ref(false),
    fetchNextPage: vi.fn(),
    refresh: vi.fn()
}
vi.mock('@/composables/useDiscovery', () => ({
    DISCOVERY_PAGE_SIZE: 48,
    useDiscoveryFeed: () => ({
        items: computed(() => feed.items.value),
        isLoading: computed(() => feed.isLoading.value),
        isError: computed(() => feed.isError.value),
        hasNextPage: computed(() => feed.hasNextPage.value),
        isFetchingNextPage: computed(() => feed.isFetchingNextPage.value),
        fetchNextPage: feed.fetchNextPage,
        refresh: feed.refresh
    })
}))

vi.mock('@/components/library/DiscoveryFeedItem.vue', () => ({
    default: {
        name: 'DiscoveryFeedItem',
        props: ['entry', 'layout'],
        template: '<div class="stub-feed-item" :data-layout="layout" :data-rank="entry.rank" />'
    }
}))

import DiscoveryView from '@/views/DiscoveryView.vue'

const albumEntry = (rank: number): DiscoveryFeedEntry => ({
    type: 'album',
    rank,
    reason: 'favorite',
    album: { id: `al-${rank}`, name: `Album ${rank}`, rank, reason: 'favorite' }
})

const stubs = { RouterLink: { template: '<a><slot /></a>' } }
const mountView = () =>
    mount(DiscoveryView, { global: { plugins: [PrimeVue], directives: { tooltip: {} }, stubs } })

beforeEach(() => {
    replaceSpy.mockReset()
    feed.fetchNextPage.mockReset()
    feed.refresh.mockReset()
    route.query = {}
    feed.items.value = []
    feed.isLoading.value = false
    feed.isError.value = false
    feed.hasNextPage.value = false
    feed.isFetchingNextPage.value = false
})

describe('DiscoveryView', () => {
    it('renders the Discovery title', () => {
        expect(mountView().text()).toContain('Discovery')
    })

    it('renders one feed item per entry', () => {
        feed.items.value = [albumEntry(0), albumEntry(1), albumEntry(2)]
        expect(mountView().findAll('.stub-feed-item')).toHaveLength(3)
    })

    it('renders items in rank order', () => {
        feed.items.value = [albumEntry(0), albumEntry(1), albumEntry(2)]
        const ranks = mountView()
            .findAll('.stub-feed-item')
            .map((n) => n.attributes('data-rank'))
        expect(ranks).toEqual(['0', '1', '2'])
    })

    it('passes the grid layout by default', () => {
        feed.items.value = [albumEntry(0)]
        expect(mountView().find('.stub-feed-item').attributes('data-layout')).toBe('grid')
    })

    it('passes the list layout when the query says list', () => {
        route.query = { view: 'list' }
        feed.items.value = [albumEntry(0)]
        expect(mountView().find('.stub-feed-item').attributes('data-layout')).toBe('list')
    })

    it('summarises the item count, pluralised', () => {
        feed.items.value = [albumEntry(0), albumEntry(1)]
        expect(mountView().text()).toContain('2 items')
    })

    it('uses the singular for one item', () => {
        feed.items.value = [albumEntry(0)]
        expect(mountView().text()).toContain('1 item')
        expect(mountView().text()).not.toContain('1 items')
    })

    it('omits the summary at zero', () => {
        expect(mountView().text()).not.toContain('0 item')
    })

    it('shows a loading state', () => {
        feed.isLoading.value = true
        expect(mountView().find('.pi-spinner').exists()).toBe(true)
    })

    it('shows an error state', () => {
        feed.isError.value = true
        expect(mountView().text()).toContain('Could not load')
    })

    it('hides the error when loading', () => {
        feed.isError.value = true
        feed.isLoading.value = true
        expect(mountView().text()).not.toContain('Could not load')
    })

    it('shows an empty state when the feed is empty', () => {
        expect(mountView().text()).toContain('Nothing here yet')
    })

    it('does not show the empty state while loading', () => {
        feed.isLoading.value = true
        expect(mountView().text()).not.toContain('Nothing here yet')
    })

    it('hides the feed during the initial loading state', () => {
        feed.isLoading.value = true
        expect(mountView().find('.discovery-feed').exists()).toBe(false)
    })

    it('refreshes the feed when the refresh action is clicked', async () => {
        feed.items.value = [albumEntry(0)]
        const w = mountView()
        await w.find('.discovery-refresh').trigger('click')
        expect(feed.refresh).toHaveBeenCalledOnce()
    })

    it('renders the sentinel only while more pages remain', () => {
        feed.items.value = [albumEntry(0)]
        expect(mountView().find('.discovery-sentinel').exists()).toBe(false)
        feed.hasNextPage.value = true
        expect(mountView().find('.discovery-sentinel').exists()).toBe(true)
    })
})

describe('DiscoveryView intersection observer', () => {
    let observes = 0
    let disconnects = 0

    beforeEach(() => {
        observes = 0
        disconnects = 0
        // jsdom has no IntersectionObserver, and the view guards on its absence —
        // so injecting a counting fake is what makes the lifecycle observable.
        ;(globalThis as unknown as { IntersectionObserver: unknown }).IntersectionObserver =
            class {
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
        mountView()
        await nextTick()
        expect(observes).toBeGreaterThan(0)
    })

    // A live observer after teardown keeps a reference to the unmounted component.
    it('disconnects the observer on unmount', async () => {
        const w = mountView()
        await nextTick()
        const before = disconnects
        w.unmount()
        expect(disconnects).toBeGreaterThan(before)
    })
})
