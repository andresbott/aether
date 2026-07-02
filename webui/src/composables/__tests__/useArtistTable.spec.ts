import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'

const getArtistIndex = vi.fn()
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { getArtistIndex: (...a: unknown[]) => getArtistIndex(...a) }
}))

import { useArtistTable } from '@/composables/useArtistTable'

function withComposable(enabled = ref(true)) {
    const captured: { api?: ReturnType<typeof useArtistTable> } = {}
    const Host = defineComponent({
        setup() {
            captured.api = useArtistTable(ref<number | undefined>(1), { enabled })
            return () => h('div')
        }
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    mount(Host, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })
    return captured
}

beforeEach(() => getArtistIndex.mockReset())

describe('useArtistTable', () => {
    it('exposes total, letters and flattened items from the index', async () => {
        getArtistIndex.mockResolvedValue({
            total: 2,
            letters: [{ name: 'A', offset: 0, count: 2 }],
            items: [{ id: 'ar1', name: 'ABBA' }, { id: 'ar2', name: 'Air' }]
        })
        const c = withComposable()
        await vi.waitFor(() => expect(c.api!.total.value).toBe(2))
        expect(c.api!.letters.value[0].name).toBe('A')
        expect(c.api!.items.value.map((a) => a.id)).toEqual(['ar1', 'ar2'])
    })

    it('does not fetch when disabled', async () => {
        getArtistIndex.mockResolvedValue({ total: 0, letters: [], items: [] })
        withComposable(ref(false))
        await new Promise((r) => setTimeout(r, 20))
        expect(getArtistIndex).not.toHaveBeenCalled()
    })
})
