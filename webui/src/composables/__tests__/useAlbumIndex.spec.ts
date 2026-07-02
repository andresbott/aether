import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'

const getAlbumIndex = vi.fn()
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { getAlbumIndex: (...a: unknown[]) => getAlbumIndex(...a) }
}))

import { useAlbumIndex } from '@/composables/useAlbumIndex'

function withComposable(enabled = ref(true)) {
    const captured: { api?: ReturnType<typeof useAlbumIndex> } = {}
    const Host = defineComponent({
        setup() {
            captured.api = useAlbumIndex(ref<number | undefined>(1), { enabled })
            return () => h('div')
        }
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    mount(Host, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })
    return captured
}

beforeEach(() => getAlbumIndex.mockReset())

describe('useAlbumIndex', () => {
    it('exposes total and letters from the index query', async () => {
        getAlbumIndex.mockResolvedValue({ total: 3, index: [{ name: 'A', offset: 0, count: 3 }] })
        const c = withComposable()
        await vi.waitFor(() => expect(c.api!.total.value).toBe(3))
        expect(c.api!.letters.value[0].name).toBe('A')
    })

    it('does not fetch when disabled', async () => {
        getAlbumIndex.mockResolvedValue({ total: 3, index: [] })
        const c = withComposable(ref(false))
        await new Promise((r) => setTimeout(r, 20))
        expect(getAlbumIndex).not.toHaveBeenCalled()
        expect(c.api!.total.value).toBe(0)
    })
})
