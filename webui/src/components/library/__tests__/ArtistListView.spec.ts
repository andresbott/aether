import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'

const scrollToIndex = vi.fn()
const state = {
    total: ref(2),
    letters: ref([{ name: 'A', offset: 0, count: 2 }]),
    items: ref([{ id: 'ar1', name: 'ABBA' }, { id: 'ar2', name: 'Air' }]),
    isLoading: ref(false),
    error: ref(null)
}
const favoritesFlags: boolean[] = []
vi.mock('@/composables/useLibrarySource', () => ({
    useArtistSource: (_folderId: unknown, favoritesOnly: { value: boolean }) => {
        favoritesFlags.push(favoritesOnly.value)
        return state
    }
}))
vi.mock('@/composables/useScrollbarWidth', () => ({ useScrollbarWidth: () => ref(10) }))

const VirtualScrollerStub = {
    name: 'VirtualScroller',
    props: { items: null, itemSize: null, lazy: Boolean },
    setup(_: unknown, { expose }: { expose: (o: object) => void }) {
        expose({ scrollToIndex })
        return () => null
    }
}

import ArtistListView from '@/components/library/ArtistListView.vue'
import AlphabetRail from '@/components/library/AlphabetRail.vue'
import { LIST_CARD_METRICS } from '@/composables/useListCardMetrics'

const mountView = (props: { folderId?: number; favoritesOnly?: boolean } = { folderId: 1 }) =>
    mount(ArtistListView, {
        props,
        global: { stubs: { VirtualScroller: VirtualScrollerStub } }
    })

describe('ArtistListView', () => {
    beforeEach(() => {
        scrollToIndex.mockClear()
        favoritesFlags.length = 0
    })

    it('passes the index letters to the alphabet rail', () => {
        const w = mountView()
        expect(w.findComponent(AlphabetRail).props('letters')).toEqual(state.letters.value)
    })

    it('scrolls to the offset when a letter is selected', async () => {
        const w = mountView()
        w.findComponent(AlphabetRail).vm.$emit('select', 1)
        await w.vm.$nextTick()
        expect(scrollToIndex).toHaveBeenCalledWith(1)
    })

    it('renders the virtual scroller non-lazy with all items', () => {
        const w = mountView()
        const vs = w.findComponent(VirtualScrollerStub)
        expect(vs.props('lazy')).toBeFalsy()
        expect(vs.props('items')).toHaveLength(2)
    })

    // One slot per card plus the gap below it, or the cards drift off their slots.
    // jsdom has no matchMedia, so the viewport reads as desktop.
    it('sizes each slot to a list card', () => {
        const { cover, pad, gap } = LIST_CARD_METRICS.wide
        const vs = mountView().findComponent(VirtualScrollerStub)
        expect(vs.props('itemSize')).toBe(cover + 2 * pad + gap)
    })

    it('shows an empty state when there are no artists', () => {
        state.total.value = 0
        const w = mountView()
        expect(w.text()).toContain('No artists found')
        state.total.value = 2
    })

    it('names the favorites filter in the empty state rather than claiming no artists exist', () => {
        state.total.value = 0
        const w = mountView({ folderId: 1, favoritesOnly: true })
        expect(w.text()).toContain('No favorite artists yet')
        expect(w.text()).not.toContain('No artists found')
        state.total.value = 2
    })

    it('passes favoritesOnly through to the source (defaulting to false)', () => {
        mountView({ folderId: 1 })
        expect(favoritesFlags.at(-1)).toBe(false)
        mountView({ folderId: 1, favoritesOnly: true })
        expect(favoritesFlags.at(-1)).toBe(true)
    })
})
