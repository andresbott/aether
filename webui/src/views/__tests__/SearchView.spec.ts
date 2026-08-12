import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref, unref, type Ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { Artist, Album, Genre, Song } from '@/types/subsonic'
import type { SearchParams } from '@/types/subsonic'

const route = { query: {} as Record<string, string> }
const replace = vi.fn()
const push = vi.fn()
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace, push })
}))

const searchResult = ref<{
    artist?: Artist[]
    album?: Album[]
    song?: Song[]
    genre?: Genre[]
}>({})
const isLoading = ref(false)
const searchError = ref<Error | null>(null)
const playAlbum = vi.fn()
const enqueueAndPlayIfIdle = vi.fn()
let lastSearchParams: Ref<SearchParams> | null = null

// Only useSearch is stubbed; the length threshold comes through from the real
// module, so the view can never drift from the value the query layer enforces.
vi.mock('@/composables/useSubsonicQueries', async (importOriginal) => ({
    ...(await importOriginal<typeof import('@/composables/useSubsonicQueries')>()),
    useSearch: (params: Ref<SearchParams>) => {
        lastSearchParams = params
        return { data: searchResult, isLoading, error: searchError }
    },
    // The song rows carry a favorite toggle, whose real mutation needs a
    // VueQueryPlugin-provided client this spec has no use for.
    useToggleStar: () => ({ mutate: vi.fn() })
}))

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ playAlbum, enqueueAndPlayIfIdle })
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
const GenreCardStub = {
    name: 'GenreCard',
    props: ['genre'],
    template: '<div class="genre-card-stub">{{ genre.value }}</div>'
}
const GenreRowStub = {
    name: 'GenreRow',
    props: ['genre'],
    template: '<div class="genre-row-stub">{{ genre.value }}</div>'
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
                GenreCard: GenreCardStub,
                GenreRow: GenreRowStub,
                // vue-router is mocked, so RouterLink (used by GenreTrackRow's
                // album link) isn't registered — stub it to a plain anchor.
                RouterLink: { template: '<a><slot /></a>' }
            }
        }
    })

async function typeQuery(w: ReturnType<typeof mountView>, text: string) {
    await w.find('input[placeholder]').setValue(text)
}

// The scope is a SelectButton, which renders a <button> per option. Scoped to
// .search-filters so the grid/list SelectButton in the header can't be hit.
const scopeButtons = (w: ReturnType<typeof mountView>) =>
    w.findAll('.search-filters button')

async function setScope(w: ReturnType<typeof mountView>, label: string) {
    const option = scopeButtons(w).find((el) => el.text().trim() === label)
    if (!option) {
        const seen = scopeButtons(w).map((el) => el.text().trim())
        throw new Error(`scope "${label}" not found; options: ${JSON.stringify(seen)}`)
    }
    await option.trigger('click')
}

const activeScope = (w: ReturnType<typeof mountView>) =>
    scopeButtons(w)
        .find((el) => el.attributes('aria-pressed') === 'true')
        ?.text()
        .trim()

const sampleResults = () => ({
    artist: [{ id: 'ar1', name: 'Pink Floyd' }],
    album: [
        { id: 'al1', name: 'The Wall' },
        { id: 'al2', name: 'Animals' }
    ],
    genre: [{ value: 'Prog Rock', songCount: 12, albumCount: 3 }],
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
    enqueueAndPlayIfIdle.mockClear()
    lastSearchParams = null
})

describe('SearchView', () => {
    it('shows the empty-query prompt and no summary before typing', () => {
        const w = mountView()
        expect(w.findComponent(ScaffoldStub).props('title')).toBe('Search')
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe('')
        expect(w.text()).toContain('Search your library')
    })

    it('keeps prompting until the term reaches the threshold', async () => {
        searchResult.value = sampleResults()
        const w = mountView()

        await typeQuery(w, 'ro')
        // Not "No results found" — nothing was searched yet.
        expect(w.text()).toContain('at least 3 characters')
        expect(w.text()).not.toContain('No results found')
        expect(w.findAll('.album-card-stub')).toHaveLength(0)
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe('')

        await typeQuery(w, 'roc')
        expect(w.text()).not.toContain('at least 3 characters')
        expect(w.findAll('.album-card-stub')).toHaveLength(2)
    })

    it('does not count surrounding whitespace toward the threshold', async () => {
        searchResult.value = sampleResults()
        const w = mountView()
        await typeQuery(w, '  a ')
        expect(w.text()).toContain('at least 3 characters')
        expect(w.findAll('.album-card-stub')).toHaveLength(0)
    })

    it('shows the generic prompt, not the threshold hint, on an empty term', async () => {
        const w = mountView()
        await typeQuery(w, 'roc')
        await typeQuery(w, '')
        expect(w.text()).toContain('Search your library')
        expect(w.text()).not.toContain('at least 3 characters')
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
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe(
            '1 artist • 2 albums • 1 genre • 1 song'
        )
        expect(w.findAll('.artist-card-stub')).toHaveLength(1)
        expect(w.findAll('.album-card-stub')).toHaveLength(2)
        expect(w.findAll('.genre-card-stub')).toHaveLength(1)
        const rows = w.findAll('.genre-track-row')
        expect(rows).toHaveLength(1)
        expect(rows[0].find('.col-cover').exists()).toBe(true)
        expect(w.text()).toContain('Time')
    })

    it('double-clicking a song row appends that track to the queue', async () => {
        const songs = [
            { id: 's1', title: 'Time', artist: 'Pink Floyd' },
            { id: 's2', title: 'Money', artist: 'Pink Floyd' }
        ]
        searchResult.value = { song: songs }
        const w = mountView()
        await typeQuery(w, 'floyd')
        await w.findAll('.genre-track-row')[1].trigger('dblclick')
        expect(enqueueAndPlayIfIdle).toHaveBeenCalledWith([songs[1]])
        // Double-click appends — it must never replace the queue.
        expect(playAlbum).not.toHaveBeenCalled()
    })

    it('renders rows instead of cards when the list layout is selected', async () => {
        route.query = { view: 'list' }
        searchResult.value = sampleResults()
        const w = mountView()
        await typeQuery(w, 'floyd')
        expect(w.findAll('.artist-row-stub')).toHaveLength(1)
        expect(w.findAll('.album-row-stub')).toHaveLength(2)
        expect(w.findAll('.genre-row-stub')).toHaveLength(1)
        expect(w.findAll('.artist-card-stub')).toHaveLength(0)
        expect(w.findAll('.album-card-stub')).toHaveLength(0)
        expect(w.findAll('.genre-card-stub')).toHaveLength(0)
        // Songs stay a table in both layouts.
        expect(w.findAll('.genre-track-row')).toHaveLength(1)
    })

    it('offers All plus one button per type, with All active by default', () => {
        const w = mountView()
        expect(scopeButtons(w).map((b) => b.text().trim())).toEqual([
            'All',
            'Artists',
            'Albums',
            'Genres',
            'Songs'
        ])
        expect(activeScope(w)).toBe('All')
    })

    it('narrowing the scope shows only that type and zeroes the other counts', async () => {
        searchResult.value = sampleResults()
        const w = mountView()
        await typeQuery(w, 'floyd')

        await setScope(w, 'Artists')
        expect(activeScope(w)).toBe('Artists')
        const params = unref(lastSearchParams!)
        expect(params.artistCount).toBeGreaterThan(0)
        expect(params.albumCount).toBe(0)
        expect(params.genreCount).toBe(0)
        expect(params.songCount).toBe(0)

        expect(w.findAll('.artist-card-stub')).toHaveLength(1)
        expect(w.findAll('.album-card-stub')).toHaveLength(0)
        expect(w.findAll('.genre-card-stub')).toHaveLength(0)
        expect(w.findAll('.genre-track-row')).toHaveLength(0)
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe('1 artist')
    })

    it.each([
        ['Albums', '.album-card-stub', '2 albums'],
        ['Genres', '.genre-card-stub', '1 genre']
    ])('scoping to %s shows only its own results', async (label, stub, summary) => {
        searchResult.value = sampleResults()
        const w = mountView()
        await typeQuery(w, 'floyd')
        await setScope(w, label)
        expect(w.findAll(stub).length).toBeGreaterThan(0)
        expect(w.findAll('.artist-card-stub')).toHaveLength(0)
        expect(w.findAll('.genre-track-row')).toHaveLength(0)
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe(summary)
    })

    it('scoping to Songs keeps the track table and drops the card sections', async () => {
        searchResult.value = sampleResults()
        const w = mountView()
        await typeQuery(w, 'floyd')
        await setScope(w, 'Songs')
        expect(w.findAll('.genre-track-row')).toHaveLength(1)
        expect(w.findAll('.artist-card-stub')).toHaveLength(0)
        expect(w.findAll('.album-card-stub')).toHaveLength(0)
        expect(w.findAll('.genre-card-stub')).toHaveLength(0)
        expect(unref(lastSearchParams!).songCount).toBeGreaterThan(0)
        expect(unref(lastSearchParams!).artistCount).toBe(0)
    })

    it('going back to All restores every section', async () => {
        searchResult.value = sampleResults()
        const w = mountView()
        await typeQuery(w, 'floyd')
        await setScope(w, 'Albums')
        expect(w.findAll('.artist-card-stub')).toHaveLength(0)

        await setScope(w, 'All')
        expect(activeScope(w)).toBe('All')
        expect(w.findAll('.artist-card-stub')).toHaveLength(1)
        expect(w.findAll('.album-card-stub')).toHaveLength(2)
        expect(w.findAll('.genre-card-stub')).toHaveLength(1)
        expect(w.findAll('.genre-track-row')).toHaveLength(1)
    })

    it('asks for more of a single type than it does under All', async () => {
        const w = mountView()
        await typeQuery(w, 'floyd')
        const underAll = unref(lastSearchParams!).albumCount!
        await setScope(w, 'Albums')
        expect(unref(lastSearchParams!).albumCount).toBeGreaterThan(underAll)
    })

    it('drops section headings when scoped, since the active button names it', async () => {
        searchResult.value = sampleResults()
        const w = mountView()
        await typeQuery(w, 'floyd')
        expect(w.findAll('.section-label').length).toBeGreaterThan(1)

        await setScope(w, 'Albums')
        expect(w.findAll('.section-label')).toHaveLength(0)
    })

    it('names the scope in the empty state so it reads as a filter, not an empty library', async () => {
        const w = mountView()
        await typeQuery(w, 'floyd')
        expect(w.text()).toContain('No results found')

        await setScope(w, 'Artists')
        expect(w.text()).toContain('No artists found')
    })

    it('resets the term and the scope, and hides itself when pristine', async () => {
        searchResult.value = sampleResults()
        const w = mountView()
        const resetBtn = () => w.find('button.reset-search')

        // Nothing to reset on a freshly loaded page.
        expect(resetBtn().exists()).toBe(false)

        await typeQuery(w, 'floyd')
        await setScope(w, 'Albums')
        expect(resetBtn().exists()).toBe(true)

        await resetBtn().trigger('click')

        const input = w.find('input[placeholder]').element as HTMLInputElement
        expect(input.value).toBe('')
        expect(activeScope(w)).toBe('All')
        const params = unref(lastSearchParams!)
        expect(params.query).toBe('')
        expect(params.artistCount).toBeGreaterThan(0)
        expect(params.albumCount).toBeGreaterThan(0)
        expect(params.songCount).toBeGreaterThan(0)
        expect(params.genreCount).toBeGreaterThan(0)
        expect(w.text()).toContain('Search your library')
        expect(resetBtn().exists()).toBe(false)
    })

    it('lives inside the search box, so it travels with the input', async () => {
        const w = mountView()
        await typeQuery(w, 'floyd')
        expect(w.find('.search-input-wrapper button.reset-search').exists()).toBe(true)
        // The term must not run underneath the button.
        expect(w.find('input[placeholder]').classes()).toContain('has-reset')
    })

    // The overlay positioning MUST live on a plain wrapper, not on the Button:
    // PrimeVue's ripple directive writes `position: relative` inline, which beats
    // any stylesheet rule, so positioning the Button itself leaves it in the flex
    // flow sticking out past the input's right edge. jsdom has no layout engine,
    // so this structural assertion is the only thing standing between that
    // regression and a visual review.
    it('positions via a wrapper span the ripple cannot override', async () => {
        const w = mountView()
        await typeQuery(w, 'floyd')
        const slot = w.find('.search-input-wrapper > .reset-search-slot')
        expect(slot.exists()).toBe(true)
        expect(slot.find('button.reset-search').exists()).toBe(true)
    })

    it('a term alone, or a narrowed scope alone, is enough to show reset', async () => {
        const w = mountView()
        const resetBtn = () => w.find('button.reset-search')

        await typeQuery(w, 'floyd')
        expect(resetBtn().exists()).toBe(true)

        await typeQuery(w, '')
        expect(resetBtn().exists()).toBe(false)

        await setScope(w, 'Songs')
        expect(resetBtn().exists()).toBe(true)
    })

    it('reset leaves the layout preference alone', async () => {
        route.query = { view: 'list' }
        searchResult.value = sampleResults()
        const w = mountView()
        await typeQuery(w, 'floyd')
        await w.find('button.reset-search').trigger('click')
        // Layout lives in the URL and is a display preference, not search state.
        expect(replace).not.toHaveBeenCalled()
        expect(w.findAll('.album-card-stub')).toHaveLength(0)
    })

    it('renders genre results even when they are the only match', async () => {
        searchResult.value = { genre: [{ value: 'Ambient', songCount: 4, albumCount: 1 }] }
        const w = mountView()
        await typeQuery(w, 'ambi')
        expect(w.text()).not.toContain('No results found')
        expect(w.findAll('.genre-card-stub')).toHaveLength(1)
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe('1 genre')
    })

    // Replaces the old "every type unchecked" case: single-select means a scope
    // is always active, so `allowEmpty=false` must keep a second click on the
    // active button from clearing it and searching for nothing.
    it('cannot be left with no scope, so the query always has something to search', async () => {
        searchResult.value = sampleResults()
        const w = mountView()
        await typeQuery(w, 'floyd')

        await setScope(w, 'Albums')
        await setScope(w, 'Albums')
        expect(activeScope(w)).toBe('Albums')
        expect(unref(lastSearchParams!).query).toBe('floyd')
        expect(unref(lastSearchParams!).albumCount).toBeGreaterThan(0)
        expect(w.findAll('.album-card-stub')).toHaveLength(2)
    })
})
