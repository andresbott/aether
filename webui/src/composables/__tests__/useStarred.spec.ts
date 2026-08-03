import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'

const getStarred = vi.fn()
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        getStarred: (...a: unknown[]) => getStarred(...a)
    }
}))

import { useStarredAlbums, useStarredArtists } from '@/composables/useStarred'

function withComposables(folderId = ref<number | undefined>(1)) {
    const captured: {
        albums?: ReturnType<typeof useStarredAlbums>
        artists?: ReturnType<typeof useStarredArtists>
    } = {}
    const Host = defineComponent({
        setup() {
            captured.albums = useStarredAlbums(folderId)
            captured.artists = useStarredArtists(folderId)
            return () => h('div')
        }
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    mount(Host, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })
    return captured
}

const response = {
    album: [
        { id: 'al1', name: 'Amnesiac' },
        { id: 'al2', name: 'Bends' }
    ],
    artist: [{ id: 'ar1', name: 'Radiohead' }],
    song: [{ id: 's1', title: 'Idioteque' }],
    playlist: []
}

beforeEach(() => {
    getStarred.mockReset()
})

describe('useStarredAlbums / useStarredArtists', () => {
    it('exposes items, total and derived letters per type', async () => {
        getStarred.mockResolvedValue(response)
        const c = withComposables()

        await vi.waitFor(() => expect(c.albums!.total.value).toBe(2))
        expect(c.albums!.items.value.map((a) => a.id)).toEqual(['al1', 'al2'])
        expect(c.albums!.letters.value).toEqual([
            { name: 'A', offset: 0, count: 1 },
            { name: 'B', offset: 1, count: 1 }
        ])

        expect(c.artists!.total.value).toBe(1)
        expect(c.artists!.items.value[0].name).toBe('Radiohead')
        expect(c.artists!.letters.value).toEqual([{ name: 'R', offset: 0, count: 1 }])
    })

    it('passes the folder id through and shares ONE request between both composables', async () => {
        getStarred.mockResolvedValue(response)
        const c = withComposables(ref(7))
        await vi.waitFor(() => expect(c.albums!.total.value).toBe(2))
        // Both composables key on the same query, so the albums list and the
        // artists list cost one call, not two.
        expect(getStarred).toHaveBeenCalledTimes(1)
        expect(getStarred).toHaveBeenCalledWith(7)
    })

    it('reports zero totals and no letters when nothing is starred', async () => {
        getStarred.mockResolvedValue({ album: [], artist: [], song: [], playlist: [] })
        const c = withComposables()
        await vi.waitFor(() => expect(c.albums!.isLoading.value).toBe(false))
        expect(c.albums!.total.value).toBe(0)
        expect(c.albums!.letters.value).toEqual([])
        expect(c.artists!.total.value).toBe(0)
    })

    it('surfaces the query error', async () => {
        getStarred.mockRejectedValue(new Error('boom'))
        const c = withComposables()
        await vi.waitFor(() => expect(c.albums!.error.value).toBeTruthy())
        expect(c.albums!.total.value).toBe(0)
    })
})
