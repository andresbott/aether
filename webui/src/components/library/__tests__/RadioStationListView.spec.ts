import { describe, it, expect, vi } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'

const state = {
    total: ref(2),
    items: ref([
        { id: 's1', name: 'ABBA FM' },
        { id: 's2', name: 'Blues FM' }
    ]),
    isLoading: ref(false),
    error: ref(null)
}
vi.mock('@/composables/useRadioTable', () => ({ useRadioTable: () => state }))

const VirtualScrollerStub = {
    name: 'VirtualScroller',
    props: { items: null, itemSize: null },
    setup: () => () => null
}

import RadioStationListView from '@/components/library/RadioStationListView.vue'
import { LIST_CARD_METRICS } from '@/composables/useListCardMetrics'

const mountView = () =>
    mount(RadioStationListView, {
        global: { stubs: { VirtualScroller: VirtualScrollerStub } }
    })

describe('RadioStationListView', () => {
    it('renders the virtual scroller with all items, one list card per slot', () => {
        const w = mountView()
        const vs = w.findComponent(VirtualScrollerStub)
        expect(vs.props('items')).toHaveLength(2)
        // jsdom has no matchMedia, so the viewport reads as the desktop tier.
        const { cover, pad, gap } = LIST_CARD_METRICS.wide
        expect(vs.props('itemSize')).toBe(cover + 2 * pad + gap)
    })

    it('shows an empty state when there are no stations', () => {
        state.total.value = 0
        const w = mountView()
        expect(w.text()).toContain('No radio stations')
        state.total.value = 2
    })
})
