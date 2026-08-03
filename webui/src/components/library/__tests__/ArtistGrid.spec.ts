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
const favoritesFlags: boolean[] = []
vi.mock('@/composables/useLibrarySource', () => ({
    useArtistSource: (_folderId: unknown, favoritesOnly: { value: boolean }) => {
        favoritesFlags.push(favoritesOnly.value)
        return { ...state }
    }
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

const mountGrid = (props: { folderId?: number; favoritesOnly?: boolean } = { folderId: 1 }) =>
    mount(ArtistGrid, {
        props,
        global: { stubs: { VirtualCardGrid: VirtualCardGridStub, ArtistCard: true } }
    })

describe('ArtistGrid', () => {
    afterEach(() => {
        state.total.value = 2
        state.items.value = [
            { id: 'ar1', name: 'A' },
            { id: 'ar2', name: 'B' }
        ]
        state.isLoading.value = false
        state.error.value = null
        mountCount = 0
        favoritesFlags.length = 0
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
    })

    it('re-mounts the grid when the folder changes', async () => {
        mountCount = 0
        const w = mountGrid({ folderId: 1 })
        expect(mountCount).toBe(1)
        await w.setProps({ folderId: 2 })
        expect(mountCount).toBe(2)
    })

    it('names the favorites filter in the empty state rather than claiming no artists exist', () => {
        state.total.value = 0
        state.items.value = []
        const w = mountGrid({ folderId: 1, favoritesOnly: true })
        expect(w.text()).toContain('No favorite artists yet')
        expect(w.text()).not.toContain('No artists found')
    })

    it('passes favoritesOnly through to the source (defaulting to false)', () => {
        mountGrid({ folderId: 1 })
        expect(favoritesFlags.at(-1)).toBe(false)
        mountGrid({ folderId: 1, favoritesOnly: true })
        expect(favoritesFlags.at(-1)).toBe(true)
    })

    it('re-mounts the grid when the favorites filter flips', async () => {
        mountCount = 0
        const w = mountGrid({ folderId: 1 })
        expect(mountCount).toBe(1)
        await w.setProps({ favoritesOnly: true })
        expect(mountCount).toBe(2)
    })
})
