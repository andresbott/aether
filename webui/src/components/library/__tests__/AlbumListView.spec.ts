import { describe, it, expect, vi, afterEach } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'

const ensureRange = vi.fn()
const scrollToIndex = vi.fn()
const state = {
    total: ref(3),
    letters: ref([{ name: 'A', offset: 0, count: 3 }]),
    items: ref([{ id: 'al0', name: 'A0' }, undefined, undefined]),
    isLoading: ref(false),
    error: ref(null)
}
vi.mock('@/composables/useAlbumTable', () => ({
    ALBUM_PAGE_SIZE: 100,
    useAlbumTable: () => ({ ...state, ensureRange })
}))
vi.mock('@/composables/useScrollbarWidth', () => ({ useScrollbarWidth: () => ref(10) }))

// Stub VirtualScroller: renders nothing but exposes scrollToIndex and can emit lazy-load.
const VirtualScrollerStub = {
    name: 'VirtualScroller',
    props: { items: null, itemSize: null, lazy: Boolean },
    emits: ['lazy-load'],
    setup(_: unknown, { expose }: { expose: (o: object) => void }) {
        expose({ scrollToIndex })
        return () => null
    }
}

import AlbumListView from '@/components/library/AlbumListView.vue'
import AlphabetRail from '@/components/library/AlphabetRail.vue'

const mountView = () =>
    mount(AlbumListView, {
        props: { folderId: 1 },
        global: { stubs: { VirtualScroller: VirtualScrollerStub } }
    })

describe('AlbumListView', () => {
    afterEach(() => {
        state.total.value = 3
        ensureRange.mockClear()
        scrollToIndex.mockClear()
    })

    it('renders the alphabet rail with the index letters', () => {
        const w = mountView()
        expect(w.findComponent(AlphabetRail).props('letters')).toEqual(state.letters.value)
    })

    it('scrolls the virtual scroller to the offset when a letter is selected', async () => {
        const w = mountView()
        w.findComponent(AlphabetRail).vm.$emit('select', 2)
        await w.vm.$nextTick()
        expect(scrollToIndex).toHaveBeenCalledWith(2)
    })

    it('loads the target window when a letter is selected', async () => {
        const w = mountView()
        w.findComponent(AlphabetRail).vm.$emit('select', 100)
        await w.vm.$nextTick()
        expect(ensureRange).toHaveBeenCalled()
        expect(ensureRange.mock.calls.at(-1)![0]).toBe(100)
    })

    it('shows an empty state when there are no albums', () => {
        state.total.value = 0
        const w = mountView()
        expect(w.text()).toContain('No albums found')
    })

    it('runs the virtual scroller in lazy mode', () => {
        const w = mountView()
        expect(w.findComponent(VirtualScrollerStub).props('lazy')).toBe(true)
    })
})
