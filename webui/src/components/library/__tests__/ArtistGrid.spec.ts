import { describe, it, expect, vi, afterEach } from 'vitest'
import { h, ref } from 'vue'
import { mount } from '@vue/test-utils'

const state = {
    total: ref(2),
    letters: ref([{ name: 'A', offset: 0, count: 2 }]),
    items: ref<Array<{ id: string; name: string }>>([
        { id: 'ar1', name: 'A' },
        { id: 'ar2', name: 'B' }
    ]),
    isLoading: ref(false),
    error: ref<unknown>(null)
}
vi.mock('@/composables/useArtistTable', () => ({
    useArtistTable: () => ({ ...state })
}))

let mountCount = 0

const VirtualCardGridStub = {
    name: 'VirtualCardGrid',
    props: { items: null, letters: null, total: null, pageSize: null },
    emits: ['lazyLoad'],
    setup(props: any, { slots }: any) {
        mountCount++
        return () =>
            h('div', { class: 'vcg' }, (props.items ?? []).map((item: unknown) => slots.card?.({ item })))
    }
}

import ArtistGrid from '@/components/library/ArtistGrid.vue'

const mountGrid = () =>
    mount(ArtistGrid, {
        props: { folderId: 1 },
        global: { stubs: { VirtualCardGrid: VirtualCardGridStub, ArtistCard: true } }
    })

describe('ArtistGrid', () => {
    afterEach(() => {
        state.total.value = 2
        state.isLoading.value = false
        state.error.value = null
        mountCount = 0
    })

    it('renders an ArtistCard per item through VirtualCardGrid', () => {
        const w = mountGrid()
        expect(w.findAll('artist-card-stub')).toHaveLength(2)
    })

    it('passes the index letters to the grid', () => {
        const w = mountGrid()
        expect(w.findComponent(VirtualCardGridStub).props('letters')).toEqual(state.letters.value)
    })

    it('shows an empty state when there are no artists', () => {
        state.total.value = 0
        state.items.value = []
        const w = mountGrid()
        expect(w.text()).toContain('No artists found')
        state.items.value = [
            { id: 'ar1', name: 'A' },
            { id: 'ar2', name: 'B' }
        ]
    })

    it('re-mounts the grid when the folder changes', async () => {
        mountCount = 0
        const w = mount(ArtistGrid, {
            props: { folderId: 1 },
            global: { stubs: { VirtualCardGrid: VirtualCardGridStub, ArtistCard: true } }
        })
        expect(mountCount).toBe(1)
        await w.setProps({ folderId: 2 })
        expect(mountCount).toBe(2)
    })
})
