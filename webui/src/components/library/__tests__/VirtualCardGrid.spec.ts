import { describe, it, expect, vi, afterEach } from 'vitest'
import { h, ref } from 'vue'
import { mount } from '@vue/test-utils'

const scrollToIndex = vi.fn()
vi.mock('@/composables/useScrollbarWidth', () => ({ useScrollbarWidth: () => ref(10) }))

// Stub VirtualScroller: renders each item via the #item slot (so card slots appear),
// exposes scrollToIndex, and can emit lazy-load with a row range.
const VirtualScrollerStub = {
    name: 'VirtualScroller',
    props: { items: null, itemSize: null, lazy: Boolean, numToleratedItems: null },
    emits: ['lazy-load'],
    setup(props: any, { slots, expose }: any) {
        expose({ scrollToIndex })
        return () =>
            h(
                'div',
                { class: 'vs' },
                (props.items ?? []).map((item: unknown, index: number) => slots.item?.({ item, index, options: { index } }))
            )
    }
}

import VirtualCardGrid from '@/components/library/VirtualCardGrid.vue'
import AlphabetRail from '@/components/library/AlphabetRail.vue'

type Row = { id: string }

const mountGrid = (props: {
    items: (Row | undefined)[]
    letters: { name: string; offset: number; count: number }[]
    total: number
}) =>
    mount(VirtualCardGrid, {
        props: { pageSize: 100, ...props },
        global: { stubs: { VirtualScroller: VirtualScrollerStub } },
        slots: {
            card: (slotProps: { item: Row | undefined }) =>
                slotProps.item
                    ? h('div', { class: 'card', 'data-id': slotProps.item.id })
                    : h('div', { class: 'card placeholder' })
        }
    })

const items3 = (): (Row | undefined)[] => [{ id: 'a' }, undefined, undefined]

describe('VirtualCardGrid', () => {
    afterEach(() => scrollToIndex.mockClear())

    it('passes the index letters to the alphabet rail', () => {
        const letters = [{ name: 'A', offset: 0, count: 3 }]
        const w = mountGrid({ items: items3(), letters, total: 3 })
        expect(w.findComponent(AlphabetRail).props('letters')).toEqual(letters)
    })

    it('runs the virtual scroller in lazy mode', () => {
        const w = mountGrid({ items: items3(), letters: [], total: 3 })
        expect(w.findComponent(VirtualScrollerStub).props('lazy')).toBe(true)
    })

    it('renders a card slot per item, passing undefined through for unloaded slots', () => {
        const w = mountGrid({ items: items3(), letters: [], total: 3 })
        expect(w.findAll('.card')).toHaveLength(3)
        expect(w.find('.card[data-id="a"]').exists()).toBe(true)
        expect(w.findAll('.card.placeholder')).toHaveLength(2)
    })

    it('emits an initial lazyLoad for page 0 on mount', () => {
        const w = mountGrid({ items: items3(), letters: [], total: 3 })
        const first = w.emitted('lazyLoad')?.[0]
        expect(first).toEqual([0, 2]) // min(total-1, pageSize-1) = 2
    })

    it('maps the scroller row range to an item range (1 column in jsdom)', () => {
        const w = mountGrid({ items: items3(), letters: [], total: 3 })
        w.findComponent(VirtualScrollerStub).vm.$emit('lazy-load', { first: 0, last: 2 })
        const calls = w.emitted('lazyLoad') as [number, number][]
        // columns=1 in jsdom: rows 0..2 -> items 0..2
        expect(calls.at(-1)).toEqual([0, 2])
    })

    it('scrolls to the row of a selected letter and pre-loads its window', async () => {
        const w = mountGrid({ items: items3(), letters: [], total: 3 })
        w.findComponent(AlphabetRail).vm.$emit('select', 2)
        await w.vm.$nextTick()
        // columns=1 -> offsetToRow(2,1) = 2
        expect(scrollToIndex).toHaveBeenCalledWith(2)
        const calls = w.emitted('lazyLoad') as [number, number][]
        // pre-load window starts at the offset
        expect(calls.at(-1)![0]).toBe(2)
    })

    it('re-kicks page 0 when total changes (folder switch / reset)', async () => {
        const w = mountGrid({ items: items3(), letters: [], total: 3 })
        const before = (w.emitted('lazyLoad') ?? []).length
        await w.setProps({ items: [{ id: 'x' }], total: 1 })
        const calls = w.emitted('lazyLoad') as [number, number][]
        expect(calls.length).toBeGreaterThan(before)
        expect(calls.at(-1)).toEqual([0, 0]) // min(total-1, pageSize-1) = 0
    })
})
