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
        expect(getAlbumList).toHaveBeenCalledWith('alphabeticalByName', 100, 0, 1, undefined)
        expect(c.api!.items.value[0]).toEqual({ id: 'al0', name: 'A0' })

        // Re-requesting the same page does not refetch.
        await c.api!.ensureRange(10, 40)
        expect(getAlbumList).toHaveBeenCalledTimes(1)

        // Jumping to offset 200 loads only page 2.
        await c.api!.ensureRange(200, 240)
        expect(getAlbumList).toHaveBeenCalledTimes(2)
        expect(getAlbumList).toHaveBeenLastCalledWith('alphabeticalByName', 100, 200, 1, undefined)
        expect(c.api!.items.value[200]).toEqual({ id: 'al200', name: 'A200' })
    })

    it('resets and refetches when the release type changes', async () => {
        getAlbumIndex.mockResolvedValue({ total: 100, index: [] })
        getAlbumList.mockImplementation((_t: string, size: number, offset: number) =>
            Promise.resolve(Array.from({ length: size }, (_, i) => ({ id: `al${offset + i}` })))
        )
        const releaseType = ref<string | undefined>(undefined)
        const captured: { api?: ReturnType<typeof useAlbumTable> } = {}
        const Host = defineComponent({
            setup() {
                captured.api = useAlbumTable(ref(1), undefined, releaseType)
                return () => h('div')
            }
        })
        const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
        mount(Host, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })

        await vi.waitFor(() => expect(captured.api!.total.value).toBe(100))
        await captured.api!.ensureRange(0, 50)
        expect(getAlbumList).toHaveBeenLastCalledWith('alphabeticalByName', 100, 0, 1, undefined)

        releaseType.value = 'Single'
        await vi.waitFor(() => expect(getAlbumIndex).toHaveBeenLastCalledWith(1, 'Single'))
        await captured.api!.ensureRange(0, 50)
        expect(getAlbumList).toHaveBeenLastCalledWith('alphabeticalByName', 100, 0, 1, 'Single')
    })

    // Regression: a page fetch started before a releaseType change must not land its
    // rows in items.value after the reset for the new filter has already happened —
    // otherwise the stale write sits there until the next reset.
    it('drops a stale pre-filter-change page fetch instead of writing it into the new window', async () => {
        getAlbumIndex.mockResolvedValue({ total: 100, index: [] })

        // The FIRST getAlbumList call (filter undefined) hangs on a promise we
        // control and resolve manually, later, after the filter has switched.
        let resolveStale!: (albums: Array<{ id: string }>) => void
        const stalePromise = new Promise<Array<{ id: string }>>((resolve) => {
            resolveStale = resolve
        })
        getAlbumList.mockImplementation(
            (_type: string, size: number, _offset: number, _fid: number, rt: string | undefined) => {
                if (rt === undefined) return stalePromise
                return Promise.resolve(Array.from({ length: size }, (_, i) => ({ id: `single${i}` })))
            }
        )

        const releaseType = ref<string | undefined>(undefined)
        const captured: { api?: ReturnType<typeof useAlbumTable> } = {}
        const Host = defineComponent({
            setup() {
                captured.api = useAlbumTable(ref(1), undefined, releaseType)
                return () => h('div')
            }
        })
        const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
        mount(Host, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })

        await vi.waitFor(() => expect(captured.api!.total.value).toBe(100))

        // Kick off the pre-change (filter undefined) fetch for offset 0, but don't
        // wait for it to finish — it is stuck on `stalePromise`.
        const staleCall = captured.api!.ensureRange(0, 50)

        // Switch filters before that slow page resolves. The reset watcher fires
        // (new items array, cleared loadedPages, bumped generation) before the
        // slow fetch above ever gets a chance to write.
        releaseType.value = 'Single'
        await vi.waitFor(() => expect(getAlbumIndex).toHaveBeenLastCalledWith(1, 'Single'))

        // The post-change window loads and resolves normally.
        await captured.api!.ensureRange(0, 50)
        expect(captured.api!.items.value[0]).toEqual({ id: 'single0' })

        // Only now does the slow pre-change fetch resolve, long after the switch.
        resolveStale(Array.from({ length: 50 }, (_, i) => ({ id: `stale${i}` })))
        await staleCall

        // Its rows must not have overwritten the current (post-change) window.
        expect(captured.api!.items.value[0]).toEqual({ id: 'single0' })
        expect(captured.api!.items.value.slice(0, 50)).not.toContainEqual({ id: 'stale0' })
    })
})
