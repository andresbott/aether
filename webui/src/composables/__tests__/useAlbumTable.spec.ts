import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'

const getAlbumIndex = vi.fn()
const getAlbumList = vi.fn()
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        getAlbumIndex: (...a: unknown[]) => getAlbumIndex(...a),
        getAlbumList: (...a: unknown[]) => getAlbumList(...a)
    }
}))

import { useAlbumTable } from '@/composables/useAlbumTable'

function withComposable(folderId = ref<number | undefined>(1)) {
    const captured: { api?: ReturnType<typeof useAlbumTable> } = {}
    const Host = defineComponent({
        setup() {
            captured.api = useAlbumTable(folderId)
            return () => h('div')
        }
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    mount(Host, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })
    return captured
}

beforeEach(() => {
    getAlbumIndex.mockReset()
    getAlbumList.mockReset()
})

describe('useAlbumTable', () => {
    it('exposes total and letters from the index query', async () => {
        getAlbumIndex.mockResolvedValue({ total: 3, index: [{ name: 'A', offset: 0, count: 3 }] })
        const c = withComposable()
        await vi.waitFor(() => expect(c.api!.total.value).toBe(3))
        expect(c.api!.letters.value[0].name).toBe('A')
        expect(c.api!.items.value.length).toBe(3)
    })

    it('ensureRange loads only the covering pages and fills items', async () => {
        getAlbumIndex.mockResolvedValue({ total: 250, index: [] })
        getAlbumList.mockImplementation((_type: string, size: number, offset: number) =>
            Promise.resolve(
                Array.from({ length: size }, (_, i) => ({ id: `al${offset + i}`, name: `A${offset + i}` }))
            )
        )
        const c = withComposable()
        await vi.waitFor(() => expect(c.api!.total.value).toBe(250))
        expect(c.api!.items.value.length).toBe(250)

        await c.api!.ensureRange(0, 50)
        expect(getAlbumList).toHaveBeenCalledTimes(1)
        expect(getAlbumList).toHaveBeenCalledWith('alphabeticalByName', 100, 0, 1)
        expect(c.api!.items.value[0]).toEqual({ id: 'al0', name: 'A0' })

        // Re-requesting the same page does not refetch.
        await c.api!.ensureRange(10, 40)
        expect(getAlbumList).toHaveBeenCalledTimes(1)

        // Jumping to offset 200 loads only page 2.
        await c.api!.ensureRange(200, 240)
        expect(getAlbumList).toHaveBeenCalledTimes(2)
        expect(getAlbumList).toHaveBeenLastCalledWith('alphabeticalByName', 100, 200, 1)
        expect(c.api!.items.value[200]).toEqual({ id: 'al200', name: 'A200' })
    })
})
