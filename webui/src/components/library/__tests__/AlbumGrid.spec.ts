import { describe, it, expect, vi, afterEach } from 'vitest'
import { h, ref } from 'vue'
import { mount } from '@vue/test-utils'

const ensureRange = vi.fn()
const state = {
    total: ref(2),
    letters: ref([{ name: 'A', offset: 0, count: 2 }]),
    items: ref<Array<{ id: string; name: string } | undefined>>([
        { id: 'al1', name: 'A' },
        { id: 'al2', name: 'B' }
    ]),
    isLoading: ref(false),
    error: ref<unknown>(null)
}
vi.mock('@/composables/useAlbumTable', () => ({ ALBUM_PAGE_SIZE: 100 }))
const favoritesFlags: boolean[] = []
vi.mock('@/composables/useLibrarySource', () => ({
    useAlbumSource: (_folderId: unknown, favoritesOnly: { value: boolean }) => {
        favoritesFlags.push(favoritesOnly.value)
        return { ...state, ensureRange }
    }
}))

// Stub VirtualCardGrid: renders the #card slot per item and re-exposes lazyLoad.
let mountCount = 0
const VirtualCardGridStub = {
    name: 'VirtualCardGrid',
    props: { items: null, letters: null, total: null, pageSize: null },
    emits: ['lazyLoad'],
    setup(props: any, { slots }: any) {
        mountCount++
        return () =>
            h('div', { class: 'vcg' }, ((props.items as unknown[]) ?? []).map((item) => slots.card?.({ item })))
    }
}

import AlbumGrid from '@/components/library/AlbumGrid.vue'

const mountGrid = (props: { folderId?: number; favoritesOnly?: boolean } = { folderId: 1 }) =>
    mount(AlbumGrid, {
        props,
        global: { stubs: { VirtualCardGrid: VirtualCardGridStub, AlbumCard: true } }
    })

describe('AlbumGrid', () => {
    afterEach(() => {
        state.total.value = 2
        state.isLoading.value = false
        state.error.value = null
        ensureRange.mockClear()
        favoritesFlags.length = 0
    })

    it('renders an AlbumCard per item through VirtualCardGrid', () => {
        const w = mountGrid()
        expect(w.findAll('album-card-stub')).toHaveLength(2)
    })

    it('passes the index letters to the grid', () => {
        const w = mountGrid()
        expect(w.findComponent(VirtualCardGridStub).props('letters')).toEqual(state.letters.value)
    })

    it('calls ensureRange with the item range on lazyLoad', () => {
        const w = mountGrid()
        w.findComponent(VirtualCardGridStub).vm.$emit('lazyLoad', 0, 99)
        expect(ensureRange).toHaveBeenCalledWith(0, 99)
    })

    it('shows an empty state when there are no albums', () => {
        state.total.value = 0
        const w = mountGrid()
        expect(w.text()).toContain('No albums found')
    })

    it('names the favorites filter in the empty state rather than claiming no albums exist', () => {
        state.total.value = 0
        const w = mountGrid({ folderId: 1, favoritesOnly: true })
        expect(w.text()).toContain('No favorite albums yet')
        expect(w.text()).not.toContain('No albums found')
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

    it('shows an error state when the index fails to load', () => {
        state.error.value = new Error('boom')
        const w = mountGrid()
        expect(w.text()).toContain('Could not load albums')
    })

    it('re-mounts the grid when the folder changes (resets even at same total)', async () => {
        mountCount = 0
        const w = mount(AlbumGrid, {
            props: { folderId: 1 },
            global: { stubs: { VirtualCardGrid: VirtualCardGridStub, AlbumCard: true } }
        })
        expect(mountCount).toBe(1)
        await w.setProps({ folderId: 2 })
        expect(mountCount).toBe(2)
    })
})
