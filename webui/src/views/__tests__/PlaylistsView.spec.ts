import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'

const playlists = ref<any[]>([])
vi.mock('@/composables/useSubsonicQueries', () => ({
    usePlaylists: () => ({ data: playlists, isLoading: ref(false) }),
    useCreatePlaylist: () => ({ mutate: vi.fn(), isPending: ref(false) })
}))

const replaceSpy = vi.fn()
const route = { query: {} as Record<string, unknown> }
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace: replaceSpy })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '', getPlaylist: vi.fn() }
}))
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ playAlbum: vi.fn() }) }))

import PlaylistsView from '@/views/PlaylistsView.vue'

const stubs = { RouterLink: { template: '<a><slot /></a>' } }

const mountView = () =>
    mount(PlaylistsView, { global: { plugins: [PrimeVue], directives: { tooltip: {} }, stubs } })

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
