import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

const { getPlaylist, playAlbum, scrobble } = vi.hoisted(() => ({
    getPlaylist: vi.fn(),
    playAlbum: vi.fn(),
    scrobble: vi.fn()
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '', getPlaylist, scrobble }
}))
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum }) }))
vi.mock('@/composables/useSubsonicQueries', () => ({
    useTogglePlaylistStar: () => ({ mutate: vi.fn() })
}))

import PlaylistListView from '@/components/library/PlaylistListView.vue'

const stubs = { RouterLink: { template: '<a><slot /></a>' } }
const playlists = [
    { id: 'pl1', name: 'Mix One', songCount: 3, duration: 600, created: '2025-01-01T00:00:00Z' },
    { id: 'pl2', name: 'Mix Two', songCount: 5, duration: 1200, created: '2025-01-02T00:00:00Z' }
]

beforeEach(() => {
    getPlaylist.mockReset()
    playAlbum.mockReset()
    scrobble.mockReset()
})

describe('PlaylistListView', () => {
    it('renders a row per playlist', () => {
        const w = mount(PlaylistListView, { props: { playlists }, global: { stubs } })
        expect(w.findAll('.playlist-row')).toHaveLength(2)
        expect(w.text()).toContain('Mix One')
        expect(w.text()).toContain('Mix Two')
    })

    it('uses the heart icon and favorite wording for the row toggle', () => {
        const w = mount(PlaylistListView, {
            props: {
                playlists: [
                    playlists[0],
                    { ...playlists[1], starred: '2026-02-01T00:00:00Z' }
                ]
            },
            global: { stubs }
        })
        const rows = w.findAll('.playlist-row')
        expect(rows[0].find('.row-star i').classes()).toContain('pi-heart')
        expect(rows[0].find('.row-star').attributes('aria-label')).toBe('Add to favorites')
        expect(rows[1].find('.row-star i').classes()).toContain('pi-heart-fill')
        expect(rows[1].find('.row-star').attributes('aria-label')).toBe('Remove from favorites')
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
