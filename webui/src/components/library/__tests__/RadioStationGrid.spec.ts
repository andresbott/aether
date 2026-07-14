import { describe, it, expect, vi, afterEach } from 'vitest'
import { h, ref } from 'vue'
import { mount } from '@vue/test-utils'

const state = {
    total: ref(2),
    items: ref<Array<{ id: string; name: string }>>([
        { id: 's1', name: 'ABBA FM' },
        { id: 's2', name: 'Blues FM' }
    ]),
    isLoading: ref(false),
    error: ref<unknown>(null)
}
vi.mock('@/composables/useRadioTable', () => ({ useRadioTable: () => ({ ...state }) }))

const VirtualCardGridStub = {
    name: 'VirtualCardGrid',
    props: { items: null, letters: null, total: null, showRail: null },
    setup(props: any, { slots }: any) {
        return () =>
            h('div', { class: 'vcg' }, (props.items ?? []).map((item: unknown) => slots.card?.({ item })))
    }
}

import RadioStationGrid from '@/components/library/RadioStationGrid.vue'

const mountGrid = () =>
    mount(RadioStationGrid, {
        global: { stubs: { VirtualCardGrid: VirtualCardGridStub, RadioStationCard: true } }
    })

describe('RadioStationGrid', () => {
    afterEach(() => {
        state.total.value = 2
        state.items.value = [
            { id: 's1', name: 'ABBA FM' },
            { id: 's2', name: 'Blues FM' }
        ]
        state.isLoading.value = false
        state.error.value = null
    })

    it('renders a RadioStationCard per item through VirtualCardGrid', () => {
        const w = mountGrid()
        expect(w.findAll('radio-station-card-stub')).toHaveLength(2)
    })

    it('hides the alphabet rail (passes showRail false and empty letters)', () => {
        const grid = mountGrid().findComponent(VirtualCardGridStub)
        expect(grid.props('showRail')).toBe(false)
        expect(grid.props('letters')).toEqual([])
    })

    it('shows an empty state when there are no stations', () => {
        state.total.value = 0
        state.items.value = []
        const w = mountGrid()
        expect(w.text()).toContain('No radio stations')
    })
})
