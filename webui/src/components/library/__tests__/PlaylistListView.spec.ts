import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const { getPlaylist, playAlbum } = vi.hoisted(() => ({
    getPlaylist: vi.fn(),
    playAlbum: vi.fn()
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '', getPlaylist }
}))
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum }) }))

import PlaylistListView from '@/components/library/PlaylistListView.vue'

const stubs = { RouterLink: { template: '<a><slot /></a>' } }
const playlists = [
    { id: 'pl1', name: 'Mix One', songCount: 3, duration: 600, created: '2025-01-01T00:00:00Z' },
    { id: 'pl2', name: 'Mix Two', songCount: 5, duration: 1200, created: '2025-01-02T00:00:00Z' }
]

beforeEach(() => {
    getPlaylist.mockReset()
    playAlbum.mockReset()
})

describe('PlaylistListView', () => {
    it('renders a row per playlist', () => {
        const w = mount(PlaylistListView, { props: { playlists }, global: { stubs } })
        expect(w.findAll('.playlist-row')).toHaveLength(2)
        expect(w.text()).toContain('Mix One')
        expect(w.text()).toContain('Mix Two')
    })

    it('the row play button fetches and plays that playlist', async () => {
        getPlaylist.mockResolvedValue({ id: 'pl2', entry: [{ id: 's9' }] })
        const w = mount(PlaylistListView, { props: { playlists }, global: { stubs } })
        await w.findAll('.playlist-row')[1].find('.row-play').trigger('click')
        await flushPromises()
        expect(getPlaylist).toHaveBeenCalledWith('pl2')
        expect(playAlbum).toHaveBeenCalledWith([{ id: 's9' }])
    })
})
