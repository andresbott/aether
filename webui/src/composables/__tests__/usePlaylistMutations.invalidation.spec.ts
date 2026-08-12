import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'

const createPlaylist = vi.fn()
const updatePlaylist = vi.fn()
const deletePlaylist = vi.fn()
const replacePlaylistTracks = vi.fn()
const updatePlaylistCover = vi.fn()
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        createPlaylist: (...a: unknown[]) => createPlaylist(...a),
        updatePlaylist: (...a: unknown[]) => updatePlaylist(...a),
        deletePlaylist: (...a: unknown[]) => deletePlaylist(...a),
        replacePlaylistTracks: (...a: unknown[]) => replacePlaylistTracks(...a),
        updatePlaylistCover: (...a: unknown[]) => updatePlaylistCover(...a)
    }
}))

import {
    useCreatePlaylist,
    useUpdatePlaylist,
    useDeletePlaylist,
    useReplacePlaylistTracks,
    useUpdatePlaylistCover
} from '@/composables/useSubsonicQueries'

// Mounts a host component so the composable sees a query client, with
// invalidateQueries stubbed to record what it was asked to drop.
function withComposable<T>(composable: () => T) {
    const captured: { api?: T } = {}
    const Host = defineComponent({
        setup() {
            captured.api = composable()
            return () => h('div')
        }
    })
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const invalidate = vi.fn()
    queryClient.invalidateQueries = invalidate as unknown as QueryClient['invalidateQueries']
    mount(Host, { global: { plugins: [[VueQueryPlugin, { queryClient }]] } })
    return { api: () => captured.api!, invalidate }
}

const invalidatedKeys = (invalidate: ReturnType<typeof vi.fn>): unknown[][] =>
    invalidate.mock.calls.map((c) => (c[0] as { queryKey?: unknown[] })?.queryKey ?? [])

beforeEach(() => {
    createPlaylist.mockReset()
    updatePlaylist.mockReset()
    deletePlaylist.mockReset()
    replacePlaylistTracks.mockReset()
    updatePlaylistCover.mockReset()
})

// The Discover feed on /library ranks playlists alongside albums and caches its
// pages for a whole seed window. Any playlist mutation that leaves it cached
// makes the change invisible there until a full page reload.
describe('playlist mutations invalidate the discovery feed', () => {
    it('drops the discovery feed and the playlist list on create', async () => {
        createPlaylist.mockResolvedValue({ id: 'pl-1' })
        const { api, invalidate } = withComposable(useCreatePlaylist)

        await api().mutateAsync({ name: 'New' })

        const keys = invalidatedKeys(invalidate)
        expect(keys).toContainEqual(['subsonic', 'discovery'])
        expect(keys).toContainEqual(['subsonic', 'playlists'])
    })

    it('drops the discovery feed on delete', async () => {
        deletePlaylist.mockResolvedValue(undefined)
        const { api, invalidate } = withComposable(useDeletePlaylist)

        await api().mutateAsync('pl-1')

        expect(invalidatedKeys(invalidate)).toContainEqual(['subsonic', 'discovery'])
    })

    // Rename/comment edits, track edits and cover changes all show up in the
    // feed's playlist cards, so each has to drop it too.
    it('drops the discovery feed on update, track replace and cover change', async () => {
        updatePlaylist.mockResolvedValue(undefined)
        replacePlaylistTracks.mockResolvedValue(undefined)
        updatePlaylistCover.mockResolvedValue(undefined)

        const update = withComposable(useUpdatePlaylist)
        await update.api().mutateAsync({ playlistId: 'pl-1', name: 'Renamed' })
        expect(invalidatedKeys(update.invalidate)).toContainEqual(['subsonic', 'discovery'])

        const replace = withComposable(useReplacePlaylistTracks)
        await replace.api().mutateAsync({ playlistId: 'pl-1', songIds: ['s-1'] })
        expect(invalidatedKeys(replace.invalidate)).toContainEqual(['subsonic', 'discovery'])

        const cover = withComposable(useUpdatePlaylistCover)
        await cover.api().mutateAsync({ playlistId: 'pl-1', coverClear: true })
        expect(invalidatedKeys(cover.invalidate)).toContainEqual(['subsonic', 'discovery'])
    })

    // A creation has no detail cache to drop yet; the others do.
    it('keeps the playlist detail cache out of the create invalidation', async () => {
        createPlaylist.mockResolvedValue({ id: 'pl-1' })
        const { api, invalidate } = withComposable(useCreatePlaylist)

        await api().mutateAsync({ name: 'New' })

        expect(invalidatedKeys(invalidate)).not.toContainEqual(['subsonic', 'playlist'])
    })
})
