import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const { getPlaylist, playAlbum } = vi.hoisted(() => ({
    getPlaylist: vi.fn(),
    playAlbum: vi.fn(),
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '', getPlaylist }
}))
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum }) }))

import PlaylistCard from '@/components/library/PlaylistCard.vue'

const stubs = { RouterLink: { template: '<a><slot /></a>' } }

beforeEach(() => {
    getPlaylist.mockReset()
    playAlbum.mockReset()
})

describe('PlaylistCard', () => {
    const playlist = { id: 'pl1', name: 'My Mix', songCount: 12, duration: 3600, created: '2024-01-01' }

    it('renders the name and song count', () => {
        const w = mount(PlaylistCard, { props: { playlist }, global: { stubs } })
        expect(w.text()).toContain('My Mix')
        expect(w.text()).toContain('12 songs')
    })

    it('play fetches the playlist entries and plays them', async () => {
        getPlaylist.mockResolvedValue({ id: 'pl1', entry: [{ id: 's1' }, { id: 's2' }] })
        const w = mount(PlaylistCard, { props: { playlist }, global: { stubs } })
        await w.find('.card-play').trigger('click')
        await flushPromises()
        expect(getPlaylist).toHaveBeenCalledWith('pl1')
        expect(playAlbum).toHaveBeenCalledWith([{ id: 's1' }, { id: 's2' }])
    })
})
