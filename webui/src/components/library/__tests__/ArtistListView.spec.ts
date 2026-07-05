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
vi.mock('@/composables/useArtistTable', () => ({ useArtistTable: () => state }))
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

const mountView = () =>
    mount(ArtistListView, {
        props: { folderId: 1 },
        global: { stubs: { VirtualScroller: VirtualScrollerStub } }
    })

describe('ArtistListView', () => {
    beforeEach(() => {
        scrollToIndex.mockClear()
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

    it('shows an empty state when there are no artists', () => {
        state.total.value = 0
        const w = mountView()
        expect(w.text()).toContain('No artists found')
        state.total.value = 2
    })
})
