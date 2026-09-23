import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '' }
}))
vi.mock('@/composables/useSubsonicQueries', () => ({
    useTogglePlaylistStar: () => ({ mutate: vi.fn() })
}))

const currentUser = ref<{ login: string; role: string } | null>(null)
vi.mock('@/composables/useAuth', () => ({ useAuth: () => ({ currentUser }) }))

beforeEach(() => {
    currentUser.value = null
})

import PlaylistListView from '@/components/library/PlaylistListView.vue'

const stubs = { RouterLink: { template: '<a><slot /></a>' } }
const playlists = [
    { id: 'pl1', name: 'Mix One', songCount: 3, duration: 600, created: '2025-01-01T00:00:00Z' },
    { id: 'pl2', name: 'Mix Two', songCount: 5, duration: 1200, created: '2025-01-02T00:00:00Z' }
]

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
        expect(rows[0].find('.row-star i').classes()).toContain('ms-favorite')
        expect(rows[0].find('.row-star').attributes('aria-label')).toBe('Add to favorites')
        expect(rows[1].find('.row-star i').classes()).toContain('msf-favorite')
        expect(rows[1].find('.row-star').attributes('aria-label')).toBe('Remove from favorites')
    })

    // Removed deliberately: AlbumRow has no play button either, and the row itself
    // opens the playlist. Play lives on the cards and on the detail view's hero.
    it('has no per-row play button', () => {
        const w = mount(PlaylistListView, { props: { playlists }, global: { stubs } })
        expect(w.find('.row-play').exists()).toBe(false)
    })

    it('marks foreign playlists read-only, not your own', () => {
        currentUser.value = { login: 'alice', role: 'user' }
        const w = mount(PlaylistListView, {
            props: {
                playlists: [
                    { id: 'pl1', name: 'Mine', owner: 'alice', songCount: 1, duration: 60, created: '2025-01-01T00:00:00Z' },
                    { id: 'pl2', name: 'Theirs', owner: 'bob', public: true, songCount: 1, duration: 60, created: '2025-01-02T00:00:00Z' }
                ]
            },
            global: { stubs }
        })
        const rows = w.findAll('.playlist-row')
        expect(rows[0].find('.col-songs .not-mine-icon').exists()).toBe(false)
        expect(rows[1].find('.col-songs .not-mine-icon').exists()).toBe(true)
    })
})
