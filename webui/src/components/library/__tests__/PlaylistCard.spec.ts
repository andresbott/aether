import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import type { Playlist } from '@/types/subsonic'

const { starMutate, scrobbleMock, getPlaylistMock, playAlbumMock } = vi.hoisted(() => ({
    starMutate: vi.fn(),
    scrobbleMock: vi.fn(() => Promise.resolve()),
    getPlaylistMock: vi.fn(() => Promise.resolve({ entry: [{ id: 'tr-1' }] })),
    playAlbumMock: vi.fn()
}))

vi.mock('@/composables/useSubsonicQueries', () => ({
    useTogglePlaylistStar: () => ({ mutate: starMutate })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => false,
        getCoverArtUrl: () => '',
        getPlaylist: getPlaylistMock,
        scrobble: scrobbleMock
    }
}))

vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum: playAlbumMock }) }))

import PlaylistCard from '@/components/library/PlaylistCard.vue'

const playlist = (over: Partial<Playlist> = {}): Playlist => ({
    id: 'pl-1',
    name: 'Mix',
    songCount: 3,
    duration: 300,
    created: '2026-01-01T00:00:00Z',
    ...over
})

const stubs = { RouterLink: { template: '<a><slot /></a>' } }
const mountCard = (pl: Playlist) =>
    mount(PlaylistCard, { props: { playlist: pl }, global: { stubs } })

beforeEach(() => {
    starMutate.mockReset()
    scrobbleMock.mockClear()
    playAlbumMock.mockReset()
})

describe('PlaylistCard star toggle', () => {
    it('shows an outline star when unstarred and a filled one when starred', () => {
        expect(mountCard(playlist()).find('.card-star i').classes()).toContain('pi-star')
        const starred = mountCard(playlist({ starred: '2026-02-01T00:00:00Z' }))
        expect(starred.find('.card-star i').classes()).toContain('pi-star-fill')
    })

    it('keeps a starred playlist star visible without hover', () => {
        expect(mountCard(playlist({ starred: '2026-02-01T00:00:00Z' })).find('.card-star').classes())
            .toContain('is-starred')
        expect(mountCard(playlist()).find('.card-star').classes()).not.toContain('is-starred')
    })

    it('toggles the star with the playlist id and its current state', async () => {
        const w = mountCard(playlist({ starred: '2026-02-01T00:00:00Z' }))
        await w.find('.card-star').trigger('click')
        expect(starMutate).toHaveBeenCalledWith({ id: 'pl-1', starred: true })
    })

    it('scrobbles the playlist when it is played', async () => {
        const w = mountCard(playlist())
        await w.find('.card-play').trigger('click')
        await Promise.resolve()
        await Promise.resolve()
        expect(scrobbleMock).toHaveBeenCalledWith('pl-1')
        expect(playAlbumMock).toHaveBeenCalled()
    })
})
