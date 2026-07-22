import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref, unref, type Ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { Artist, Album, Song } from '@/types/subsonic'
import type { SearchParams } from '@/types/subsonic'

const route = { query: {} as Record<string, string> }
const replace = vi.fn()
const push = vi.fn()
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace, push })
}))

const searchResult = ref<{ artist?: Artist[]; album?: Album[]; song?: Song[] }>({})
const isLoading = ref(false)
const searchError = ref<Error | null>(null)
const playAlbum = vi.fn()
let lastSearchParams: Ref<SearchParams> | null = null

vi.mock('@/composables/useSubsonicQueries', () => ({
    useSearch: (params: Ref<SearchParams>) => {
        lastSearchParams = params
        return { data: searchResult, isLoading, error: searchError }
    }
}))

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ playAlbum })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => true, getCoverArtUrl: (id: string) => `/cover/${id}` }
}))

// Stub the scaffold and the cards/rows so this spec asserts what SearchView passes
// to them (its own responsibility) rather than their internal markup — ContentScaffold
// and AlbumCard already have their own specs; the card/row components need
// router-link stubs too. Slots are rendered so header actions stay clickable.
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
const ArtistRowStub = {
    name: 'ArtistRow',
    props: ['artist'],
    template: '<div class="artist-row-stub">{{ artist.name }}</div>'
}
const AlbumRowStub = {
    name: 'AlbumRow',
    props: ['album'],
    template: '<div class="album-row-stub">{{ album.name }}</div>'
}

import SearchView from '@/views/SearchView.vue'

const mountView = () =>
    mount(SearchView, {
        global: {
            plugins: [PrimeVue],
            stubs: {
                ContentScaffold: ScaffoldStub,
                ArtistCard: ArtistCardStub,
                AlbumCard: AlbumCardStub,
                ArtistRow: ArtistRowStub,
                AlbumRow: AlbumRowStub,
                // vue-router is mocked, so RouterLink (used by GenreTrackRow's
                // album link) isn't registered — stub it to a plain anchor.
                RouterLink: { template: '<a><slot /></a>' }
            }
        }
    })

async function typeQuery(w: ReturnType<typeof mountView>, text: string) {
    await w.find('input[placeholder]').setValue(text)
}

async function setFilter(
    w: ReturnType<typeof mountView>,
    inputId: string,
    checked: boolean
) {
    const box = w.find(`#${inputId}`)
    await box.setValue(checked)
}

const sampleResults = () => ({
    artist: [{ id: 'ar1', name: 'Pink Floyd' }],
    album: [
        { id: 'al1', name: 'The Wall' },
        { id: 'al2', name: 'Animals' }
    ],
    song: [{ id: 's1', title: 'Time', artist: 'Pink Floyd' }]
})

beforeEach(() => {
    route.query = {}
    replace.mockClear()
    push.mockClear()
    searchResult.value = {}
    isLoading.value = false
    searchError.value = null
    playAlbum.mockClear()
    lastSearchParams = null
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
        searchResult.value = sampleResults()
        const w = mountView()
        await typeQuery(w, 'floyd')
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe('1 artist • 2 albums • 1 song')
        expect(w.findAll('.artist-card-stub')).toHaveLength(1)
        expect(w.findAll('.album-card-stub')).toHaveLength(2)
        const rows = w.findAll('.genre-track-row')
        expect(rows).toHaveLength(1)
        expect(rows[0].find('.col-cover').exists()).toBe(true)
        expect(w.text()).toContain('Time')
    })

    it('double-clicking a song row plays the results from that track', async () => {
        const songs = [
            { id: 's1', title: 'Time', artist: 'Pink Floyd' },
            { id: 's2', title: 'Money', artist: 'Pink Floyd' }
        ]
        searchResult.value = { song: songs }
        const w = mountView()
        await typeQuery(w, 'floyd')
        await w.findAll('.genre-track-row')[1].trigger('dblclick')
        expect(playAlbum).toHaveBeenCalledWith(songs, 1)
    })

    it('renders rows instead of cards when the list layout is selected', async () => {
        route.query = { view: 'list' }
        searchResult.value = sampleResults()
        const w = mountView()
        await typeQuery(w, 'floyd')
        expect(w.findAll('.artist-row-stub')).toHaveLength(1)
        expect(w.findAll('.album-row-stub')).toHaveLength(2)
        expect(w.findAll('.artist-card-stub')).toHaveLength(0)
        expect(w.findAll('.album-card-stub')).toHaveLength(0)
        // Songs stay a table in both layouts.
        expect(w.findAll('.genre-track-row')).toHaveLength(1)
    })

    it('zeroes the count param and hides the section when a type is unchecked', async () => {
        searchResult.value = sampleResults()
        const w = mountView()
        await typeQuery(w, 'floyd')
        await setFilter(w, 'search-albums', false)
        expect(unref(lastSearchParams!).albumCount).toBe(0)
        expect(unref(lastSearchParams!).artistCount).toBeGreaterThan(0)
        expect(w.findAll('.album-card-stub')).toHaveLength(0)
        expect(w.findAll('.artist-card-stub')).toHaveLength(1)
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe('1 artist • 1 song')
    })

    it('shows a prompt and disables the query when every type is unchecked', async () => {
        searchResult.value = sampleResults()
        const w = mountView()
        await typeQuery(w, 'floyd')
        await setFilter(w, 'search-artists', false)
        await setFilter(w, 'search-albums', false)
        await setFilter(w, 'search-songs', false)
        expect(unref(lastSearchParams!).query).toBe('')
        expect(w.text()).toContain('Select at least one type to search')
        expect(w.findAll('.artist-card-stub')).toHaveLength(0)
        expect(w.findAll('.genre-track-row')).toHaveLength(0)
    })
})
