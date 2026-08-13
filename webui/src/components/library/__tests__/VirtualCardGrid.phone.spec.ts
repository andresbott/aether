import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import VirtualCardGrid from '../VirtualCardGrid.vue'

const tier = ref<'phone' | 'tablet' | 'desktop'>('phone')
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({ tier, isTouch: ref(true), shell: ref('mobile') })
}))

const CardGridStub = {
    name: 'CardGrid',
    props: ['items', 'minColWidth', 'gap'],
    template: '<div />'
}

const mountGrid = () =>
    mount(VirtualCardGrid, {
        props: { items: [{ id: 'a' }], letters: [], total: 1, minColWidth: 200, gap: 32 },
        global: {
            stubs: {
                CardGrid: CardGridStub,
                VirtualScroller: {
                    props: { items: null, itemSize: null, lazy: Boolean, numToleratedItems: null },
                    template: '<div><slot name="item" :item="[{ id: \'a\' }]" /></div>'
                },
                AlphabetRail: true
            }
        }
    })

describe('VirtualCardGrid phone tuning', () => {
    it('caps column width and gap on the phone tier', () => {
        tier.value = 'phone'
        const grid = mountGrid()
        const row = grid.findComponent(CardGridStub)
        expect(row.props('minColWidth')).toBe(150)
        expect(row.props('gap')).toBe(16)
    })

    it('passes props through unchanged on desktop', () => {
        tier.value = 'desktop'
        const grid = mountGrid()
        const row = grid.findComponent(CardGridStub)
        expect(row.props('minColWidth')).toBe(200)
        expect(row.props('gap')).toBe(32)
    })
})
