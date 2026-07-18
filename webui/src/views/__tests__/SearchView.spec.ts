import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { Artist, Album, Song } from '@/types/subsonic'

const searchResult = ref<{ artist?: Artist[]; album?: Album[]; song?: Song[] }>({})
const isLoading = ref(false)
const searchError = ref<Error | null>(null)
const playNow = vi.fn()

vi.mock('@/composables/useSubsonicQueries', () => ({
    useSearch: () => ({ data: searchResult, isLoading, error: searchError })
}))

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ playNow })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { getCoverArtUrl: (id: string) => `/cover/${id}` }
}))

// Stub the scaffold and the cards so this spec asserts what SearchView passes to
// them (its own responsibility) rather than their internal markup — ContentScaffold
// and AlbumCard already have their own specs; ArtistCard needs a router-link stub too.
const ScaffoldStub = {
    name: 'ContentScaffold',
    props: ['title', 'summary'],
    template: '<div><slot name="actions" /><slot /></div>'
}
const ArtistCardStub = {
    name: 'ArtistCard',
    props: ['artist'],
    template: '<div class="artist-card-stub">{{ artist.name }}</div>'
}
const AlbumCardStub = {
    name: 'AlbumCard',
    props: ['album'],
    template: '<div class="album-card-stub">{{ album.name }}</div>'
}

import SearchView from '@/views/SearchView.vue'

const mountView = () =>
    mount(SearchView, {
        global: {
            plugins: [PrimeVue],
            stubs: {
                ContentScaffold: ScaffoldStub,
                ArtistCard: ArtistCardStub,
                AlbumCard: AlbumCardStub
            }
        }
    })

async function typeQuery(w: ReturnType<typeof mountView>, text: string) {
    await w.find('input').setValue(text)
}

beforeEach(() => {
    searchResult.value = {}
    isLoading.value = false
    searchError.value = null
    playNow.mockClear()
})

describe('SearchView', () => {
    it('shows the empty-query prompt and no summary before typing', () => {
        const w = mountView()
        expect(w.findComponent(ScaffoldStub).props('title')).toBe('Search')
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe('')
        expect(w.text()).toContain('Search your library')
    })

    it('shows a loading state while a query is in flight', async () => {
        isLoading.value = true
        const w = mountView()
        await typeQuery(w, 'floyd')
        expect(w.find('.pi-spinner').exists()).toBe(true)
    })

    it('shows an error state distinct from the empty-results state', async () => {
        searchError.value = new Error('boom')
        const w = mountView()
        await typeQuery(w, 'floyd')
        expect(w.text()).toContain('Could not search')
        expect(w.text()).not.toContain('No results found')
    })

    it('shows "No results found" when the query returns nothing', async () => {
        const w = mountView()
        await typeQuery(w, 'floyd')
        expect(w.text()).toContain('No results found')
    })

    it('renders grouped results and a pluralized summary', async () => {
        searchResult.value = {
            artist: [{ id: 'ar1', name: 'Pink Floyd' }],
            album: [
                { id: 'al1', name: 'The Wall' },
                { id: 'al2', name: 'Animals' }
            ],
            song: [{ id: 's1', title: 'Time', artist: 'Pink Floyd' }]
        }
        const w = mountView()
        await typeQuery(w, 'floyd')
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe('1 artist • 2 albums • 1 song')
        expect(w.findAll('.artist-card-stub')).toHaveLength(1)
        expect(w.findAll('.album-card-stub')).toHaveLength(2)
        expect(w.findAll('.song-row')).toHaveLength(1)
        expect(w.text()).toContain('Time')
    })

    it('plays a song when its row is clicked', async () => {
        const song = { id: 's1', title: 'Time', artist: 'Pink Floyd' }
        searchResult.value = { song: [song] }
        const w = mountView()
        await typeQuery(w, 'floyd')
        await w.find('.song-row').trigger('click')
        expect(playNow).toHaveBeenCalledWith(song)
    })
})
