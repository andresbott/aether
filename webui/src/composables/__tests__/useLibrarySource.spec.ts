import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'

const getAlbumIndex = vi.fn()
const getAlbumList = vi.fn()
const getArtistIndex = vi.fn()
const getStarred = vi.fn()
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        getAlbumIndex: (...a: unknown[]) => getAlbumIndex(...a),
        getAlbumList: (...a: unknown[]) => getAlbumList(...a),
        getArtistIndex: (...a: unknown[]) => getArtistIndex(...a),
        getStarred: (...a: unknown[]) => getStarred(...a)
    }
}))

import { useAlbumSource, useArtistSource } from '@/composables/useLibrarySource'

function withSources(favoritesOnly = ref(false), folderId = ref<number | undefined>(1)) {
    const captured: {
        albums?: ReturnType<typeof useAlbumSource>
        artists?: ReturnType<typeof useArtistSource>
    } = {}
    const Host = defineComponent({
        setup() {
            captured.albums = useAlbumSource(folderId, favoritesOnly)
            captured.artists = useArtistSource(folderId, favoritesOnly)
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
    getArtistIndex.mockReset()
    getStarred.mockReset()

    getAlbumIndex.mockResolvedValue({ total: 2, index: [{ name: 'A', offset: 0, count: 2 }] })
    getAlbumList.mockResolvedValue([
        { id: 'al1', name: 'All 1' },
        { id: 'al2', name: 'All 2' }
    ])
    getArtistIndex.mockResolvedValue({
        total: 2,
        letters: [{ name: 'A', offset: 0, count: 2 }],
        items: [
            { id: 'ar1', name: 'All Artist 1' },
            { id: 'ar2', name: 'All Artist 2' }
        ]
    })
    getStarred.mockResolvedValue({
        album: [{ id: 'al2', name: 'Fav Album' }],
        artist: [{ id: 'ar2', name: 'Fav Artist' }],
        song: [],
        playlist: []
    })
})

describe('useAlbumSource', () => {
    it('serves the full library and never calls getStarred while the filter is off', async () => {
        const c = withSources(ref(false))
        await vi.waitFor(() => expect(c.albums!.total.value).toBe(2))
        expect(getStarred).not.toHaveBeenCalled()
    })

    it('serves favorites and never calls the library index while the filter is on', async () => {
        const c = withSources(ref(true))
        await vi.waitFor(() => expect(c.albums!.total.value).toBe(1))
        expect(c.albums!.items.value[0]!.name).toBe('Fav Album')
        expect(c.albums!.letters.value).toEqual([{ name: 'F', offset: 0, count: 1 }])
        expect(getAlbumIndex).not.toHaveBeenCalled()
    })

    it('swaps sources when the flag flips, fetching each only on first use', async () => {
        const favoritesOnly = ref(false)
        const c = withSources(favoritesOnly)
        await vi.waitFor(() => expect(c.albums!.total.value).toBe(2))

        favoritesOnly.value = true
        await vi.waitFor(() => expect(c.albums!.total.value).toBe(1))
        expect(getStarred).toHaveBeenCalledTimes(1)

        // Flipping back reads the cached library index rather than refetching.
        favoritesOnly.value = false
        await vi.waitFor(() => expect(c.albums!.total.value).toBe(2))
        expect(getAlbumIndex).toHaveBeenCalledTimes(1)
    })

    it('ensureRange pages the library but is a no-op on favorites (getStarred2 is unpaginated)', async () => {
        const favoritesOnly = ref(false)
        const c = withSources(favoritesOnly)
        await vi.waitFor(() => expect(c.albums!.total.value).toBe(2))

        await c.albums!.ensureRange(0, 1)
        expect(getAlbumList).toHaveBeenCalledTimes(1)

        favoritesOnly.value = true
        await vi.waitFor(() => expect(c.albums!.total.value).toBe(1))
        await c.albums!.ensureRange(0, 1)
        expect(getAlbumList).toHaveBeenCalledTimes(1)
    })
})

describe('useArtistSource', () => {
    it('serves the full artist index while the filter is off', async () => {
        const c = withSources(ref(false))
        await vi.waitFor(() => expect(c.artists!.total.value).toBe(2))
        expect(getStarred).not.toHaveBeenCalled()
    })

    it('serves starred artists with derived letters while the filter is on', async () => {
        const c = withSources(ref(true))
        await vi.waitFor(() => expect(c.artists!.total.value).toBe(1))
        expect(c.artists!.items.value[0].name).toBe('Fav Artist')
        expect(c.artists!.letters.value).toEqual([{ name: 'F', offset: 0, count: 1 }])
        expect(getArtistIndex).not.toHaveBeenCalled()
    })
})
