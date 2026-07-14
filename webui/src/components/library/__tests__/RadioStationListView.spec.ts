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

const mountView = () =>
    mount(RadioStationListView, {
        global: { stubs: { VirtualScroller: VirtualScrollerStub } }
    })

describe('RadioStationListView', () => {
    it('renders a Station/Homepage header and the virtual scroller with all items', () => {
        const w = mountView()
        expect(w.text()).toContain('Station')
        expect(w.text()).toContain('Homepage')
        expect(w.findComponent(VirtualScrollerStub).props('items')).toHaveLength(2)
    })

    it('shows an empty state when there are no stations', () => {
        state.total.value = 0
        const w = mountView()
        expect(w.text()).toContain('No radio stations')
        state.total.value = 2
    })
})
