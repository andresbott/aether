import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'

const playlists = ref<any[]>([])
vi.mock('@/composables/useSubsonicQueries', () => ({
    usePlaylists: () => ({ data: playlists, isLoading: ref(false) }),
    useCreatePlaylist: () => ({ mutate: vi.fn(), isPending: ref(false) }),
    useTogglePlaylistStar: () => ({ mutate: vi.fn() })
}))

const replaceSpy = vi.fn()
const route = { query: {} as Record<string, unknown> }
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace: replaceSpy })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '', getPlaylist: vi.fn(), scrobble: vi.fn() }
}))
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum: vi.fn() }) }))

import PlaylistsView from '@/views/PlaylistsView.vue'

const stubs = { RouterLink: { template: '<a><slot /></a>' } }

const mountView = () =>
    mount(PlaylistsView, { global: { plugins: [PrimeVue], directives: { tooltip: {} }, stubs } })

// PrimeVue's SelectButton renders a ToggleButton per option, so finding by
// component type would match the layout toggle too — target the stable hook class.
const favoritesToggle = (w: ReturnType<typeof mountView>) =>
    w.find('.playlists-favorites-filter')
const favoriteIcon = (w: ReturnType<typeof mountView>) =>
    favoritesToggle(w).find('.p-togglebutton-icon')

beforeEach(() => {
    playlists.value = [
        { id: 'pl1', name: 'Mix One', songCount: 3 },
        { id: 'pl2', name: 'Mix Two', songCount: 5 }
    ]
    replaceSpy.mockReset()
    route.query = {}
})

describe('PlaylistsView', () => {
    it('defaults to the grid layout with a card per playlist', () => {
        const w = mountView()
        expect(w.findAll('.playlist-card')).toHaveLength(2)
        expect(w.find('.playlist-list').exists()).toBe(false)
    })

    it('renders the list layout when the route query says so', () => {
        route.query = { view: 'list' }
        const w = mountView()
        expect(w.find('.playlist-list').exists()).toBe(true)
        expect(w.find('.playlist-card').exists()).toBe(false)
    })

    it('shows the count summary', () => {
        const w = mountView()
        expect(w.text()).toContain('2 playlists')
    })
})

// Mirrors LibraryView's favorites filter: URL state (?favorites=1), heart pair,
// and a count that says "favorites" rather than the unfiltered total. Unlike the
// library's it is a client-side predicate — getPlaylists already carries `starred`.
describe('PlaylistsView favorites filter', () => {
    const starred = () => [
        { id: 'pl1', name: 'Mix One', songCount: 3 },
        { id: 'pl2', name: 'Mix Two', songCount: 5, starred: '2026-02-01T00:00:00Z' }
    ]

    it('is off by default and lists everything with an outline heart', () => {
        const w = mountView()
        expect(favoriteIcon(w).classes()).toContain('pi-heart')
        expect(favoriteIcon(w).classes()).not.toContain('pi-heart-fill')
        expect(w.findAll('.playlist-card')).toHaveLength(2)
    })

    it('lists only starred playlists when ?favorites=1, in both layouts', () => {
        playlists.value = starred()
        route.query = { favorites: '1' }
        const grid = mountView()
        expect(favoriteIcon(grid).classes()).toContain('pi-heart-fill')
        expect(grid.findAll('.playlist-card')).toHaveLength(1)
        expect(grid.text()).toContain('Mix Two')
        expect(grid.text()).not.toContain('Mix One')

        route.query = { favorites: '1', view: 'list' }
        const list = mountView()
        expect(list.findAll('.playlist-row')).toHaveLength(1)
        expect(list.text()).toContain('Mix Two')
        expect(list.text()).not.toContain('Mix One')
    })

    it('writes ?favorites=1 on enable and drops the key on disable', async () => {
        const w = mountView()
        await favoritesToggle(w).trigger('click')
        expect(replaceSpy).toHaveBeenCalledWith({ query: { favorites: '1' } })

        replaceSpy.mockReset()
        route.query = { favorites: '1' }
        const on = mountView()
        await favoritesToggle(on).trigger('click')
        expect(replaceSpy).toHaveBeenCalledWith({ query: {} })
    })

    it('preserves the layout when the filter is toggled', async () => {
        route.query = { view: 'list' }
        const w = mountView()
        await favoritesToggle(w).trigger('click')
        expect(replaceSpy).toHaveBeenCalledWith({ query: { view: 'list', favorites: '1' } })
    })

    it('labels the toggle by what clicking it will do', () => {
        expect(favoritesToggle(mountView()).attributes('aria-label')).toBe('Show favorites only')
        route.query = { favorites: '1' }
        expect(favoritesToggle(mountView()).attributes('aria-label')).toBe('Show all')
    })

    it('summarises the favorites count, pluralised, never the unfiltered total', () => {
        playlists.value = starred()
        route.query = { favorites: '1' }
        const single = mountView().text()
        expect(single).toContain('1 favorite')
        expect(single).not.toContain('1 favorites')
        expect(single).not.toContain('2 playlists')

        playlists.value = [
            { id: 'pl1', name: 'Mix One', songCount: 3, starred: '2026-02-01T00:00:00Z' },
            { id: 'pl2', name: 'Mix Two', songCount: 5, starred: '2026-02-02T00:00:00Z' }
        ]
        expect(mountView().text()).toContain('2 favorites')
    })

    it('says no FAVORITES yet — not "No playlists" — when none are starred', () => {
        route.query = { favorites: '1' }
        const w = mountView()
        expect(w.text()).toContain('No favorite playlists yet')
        expect(w.text()).not.toContain('No playlists')
        expect(w.text()).not.toContain('2 playlists')
    })
})
