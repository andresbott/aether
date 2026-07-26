import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'

const updateArtistCover = vi.fn()
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        updateArtistCover: (...a: unknown[]) => updateArtistCover(...a)
    }
}))

import { useUpdateArtistCover } from '@/composables/useSubsonicQueries'

function withComposable() {
    const captured: { api?: ReturnType<typeof useUpdateArtistCover> } = {}
    const Host = defineComponent({
        setup() {
            captured.api = useUpdateArtistCover()
            return () => h('div')
        }
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const invalidate = vi.fn()
    queryClient.invalidateQueries = invalidate as unknown as QueryClient['invalidateQueries']
    mount(Host, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })
    return { captured, invalidate }
}

// Returns the flat list of queryKeys invalidateQueries was asked to drop.
const invalidatedKeys = (invalidate: ReturnType<typeof vi.fn>): unknown[][] =>
    invalidate.mock.calls.map((c) => (c[0] as { queryKey?: unknown[] })?.queryKey ?? [])

beforeEach(() => updateArtistCover.mockReset())

describe('useUpdateArtistCover cache invalidation', () => {
    it('drops the artist detail query so the new image is picked up', async () => {
        updateArtistCover.mockResolvedValue(undefined)
        const { captured, invalidate } = withComposable()

        await captured.api!.mutateAsync({ artistId: 'ar-1' })

        expect(invalidatedKeys(invalidate)).toContainEqual(['subsonic', 'artist', 'ar-1'])
    })

    // The artist index feeds the library list/grid, and search results carry
    // artist entries too — both render the cover, so both go stale on a change.
    it('drops the artist index and search results', async () => {
        updateArtistCover.mockResolvedValue(undefined)
        const { captured, invalidate } = withComposable()

        await captured.api!.mutateAsync({ artistId: 'ar-1' })

        const keys = invalidatedKeys(invalidate)
        expect(keys).toContainEqual(['subsonic', 'artistIndex'])
        expect(keys).toContainEqual(['subsonic', 'search'])
    })

    // An upload moves the image into aether's store and a clear can uncover the
    // music-folder file again, so the recorded source is stale either way.
    it('drops the artist image-source query', async () => {
        updateArtistCover.mockResolvedValue(undefined)
        const { captured, invalidate } = withComposable()

        await captured.api!.mutateAsync({ artistId: 'ar-1' })

        expect(invalidatedKeys(invalidate)).toContainEqual(['artistImageSource', 'ar-1'])
    })
})
