import { describe, it, expect, vi, beforeEach } from 'vitest'
import { computed, reactive, ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { WishlistItem, WishlistKind, WishlistSearchResult } from '@/types/wishlist'

const route = reactive({ query: {} as Record<string, string> })
const replace = vi.fn((to: { query: Record<string, string> }) => {
    route.query = to.query
})
const push = vi.fn()
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace, push })
}))

const term = ref('')
const results = ref<Partial<Record<WishlistKind, WishlistSearchResult[]>>>({})
const pending = ref<WishlistKind[]>([])
const error = ref<string | null>(null)
const rateLimited = ref(false)
const search = vi.fn((text: string) => {
    term.value = text.trim()
})
const clear = vi.fn(() => {
    term.value = ''
    results.value = {}
})
const mutate = vi.fn()
vi.mock('@/composables/useWishlist', () => ({
    MIN_WISHLIST_QUERY_LENGTH: 2,
    useWishlistSearch: () => ({
        term,
        results,
        pending,
        loading: computed(() => pending.value.length > 0),
        error,
        rateLimited,
        search,
        clear
    }),
    useAddWishlistItems: () => ({ mutate, isPending: ref(false) })
}))

const ScaffoldStub = {
    name: 'ContentScaffold',
    props: ['title', 'summary'],
    template: '<div><slot name="actions" /><slot /></div>'
}
import WishlistAddView from '@/views/WishlistAddView.vue'
import SelectButton from 'primevue/selectbutton'

const result = (over: Partial<WishlistSearchResult>): WishlistSearchResult => ({
    kind: 'album',
    mbid: 'm1',
    title: 'Nevermind',
    artist: 'Nirvana',
    album: '',
    date: '1991',
    type: 'Album',
    disambiguation: '',
    score: 100,
    in_library: false,
    ...over
})

const mountView = () =>
    mount(WishlistAddView, {
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: {
                ContentScaffold: ScaffoldStub,
                RouterLink: { props: ['to'], template: '<a class="router-link-stub"><slot /></a>' }
            }
        }
    })

const typeAndSubmit = async (w: ReturnType<typeof mountView>, text: string) => {
    await w.find('input.search-input').setValue(text)
    await w.find('form.search-row').trigger('submit')
}

beforeEach(() => {
    route.query = {}
    term.value = ''
    results.value = {}
    pending.value = []
    error.value = null
    rateLimited.value = false
    for (const f of [replace, push, search, clear, mutate]) f.mockClear()
})

describe('WishlistAddView', () => {
    it('does not search while typing, only on submit, across every kind for All', async () => {
        const w = mountView()
        await w.find('input.search-input').setValue('in utero nirvana')
        expect(search).not.toHaveBeenCalled()

        await w.find('form.search-row').trigger('submit')
        expect(search).toHaveBeenCalledWith('in utero nirvana', ['album', 'artist', 'song'])
    })

    it('searches on Enter in the box', async () => {
        const w = mountView()
        await w.find('input.search-input').setValue('nevermind')
        await w.find('input.search-input').trigger('keydown', { key: 'Enter' })
        expect(search).toHaveBeenCalledTimes(1)
    })

    it('keeps the Search button disabled below the minimum length', async () => {
        const w = mountView()
        await w.find('input.search-input').setValue('n')
        expect(w.find('.wishlist-search-submit').attributes('disabled')).toBeDefined()
        await w.find('form.search-row').trigger('submit')
        expect(search).not.toHaveBeenCalled()
    })

    it('searches one kind under a narrowed filter, and a filter click re-searches once something was searched', async () => {
        route.query = { kind: 'song' }
        const w = mountView()
        await typeAndSubmit(w, 'lithium')
        expect(search).toHaveBeenLastCalledWith('lithium', ['song'])

        w.findComponent(SelectButton).vm.$emit('update:modelValue', 'album')
        await flushPromises()
        expect(replace).toHaveBeenLastCalledWith({ query: { kind: 'album' } })
        expect(search).toHaveBeenLastCalledWith('lithium', ['album'])
    })

    it('a filter click before any search only sets the filter', async () => {
        const w = mountView()
        w.findComponent(SelectButton).vm.$emit('update:modelValue', 'artist')
        await flushPromises()
        expect(search).not.toHaveBeenCalled()
    })

    it('shows the results in labelled sections under All, with a count summary', async () => {
        const w = mountView()
        term.value = 'nirvana'
        results.value = {
            artist: [result({ kind: 'artist', mbid: 'a1', title: 'Nirvana' })],
            album: [result({ mbid: 'b1' }), result({ mbid: 'b2', title: 'In Utero' })],
            song: []
        }
        await flushPromises()
        expect(w.findAll('.section-label').map((h) => h.text())).toEqual(['Albums', 'Artists'])
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe('2 albums • 1 artist')
    })

    it('drops the section labels and other kinds under a narrowed filter', async () => {
        route.query = { kind: 'album' }
        const w = mountView()
        term.value = 'nirvana'
        results.value = { artist: [result({ kind: 'artist', mbid: 'a1' })], album: [result({ mbid: 'b1' })] }
        await flushPromises()
        expect(w.findAll('.section-label')).toHaveLength(0)
        expect(w.findAll('.result-row')).toHaveLength(1)
    })

    it('says nothing matched only once a search has answered empty', async () => {
        const w = mountView()
        expect(w.text()).not.toContain('No matches')
        term.value = 'zzzz'
        pending.value = ['album']
        await flushPromises()
        expect(w.text()).not.toContain('No matches')
        pending.value = []
        results.value = { album: [], artist: [], song: [] }
        await flushPromises()
        expect(w.text()).toContain('No matches on MusicBrainz')
    })

    it('collects picks in the selection pill and adds them, then returns to the wishlist', async () => {
        const w = mountView()
        term.value = 'nirvana'
        results.value = { album: [result({ mbid: 'a' }), result({ mbid: 'b', title: 'In Utero' })] }
        await flushPromises()
        expect(w.find('.search-selection-pill').exists()).toBe(false)

        const picks = w.findAll('.result-pick')
        await picks[0].trigger('click')
        await picks[1].trigger('click')
        await picks[1].trigger('click')
        expect(w.find('.selection-count').text()).toBe('1 selected')

        await w.find('.wishlist-add-selected').trigger('click')
        const [sent, opts] = mutate.mock.calls[0]
        expect(sent).toEqual([
            {
                kind: 'album',
                mbid: 'a',
                title: 'Nevermind',
                artist: 'Nirvana',
                album: '',
                date: '1991',
                type: 'Album',
                disambiguation: ''
            }
        ])
        opts.onSuccess([{ id: 1 }] as WishlistItem[])
        await flushPromises()
        expect(push).toHaveBeenCalledWith({ name: 'wishlist' })
    })

    it('keeps picks across a new search, and clears them on request', async () => {
        const w = mountView()
        term.value = 'nirvana'
        results.value = { album: [result({ mbid: 'a' })] }
        await flushPromises()
        await w.find('.result-pick').trigger('click')

        term.value = 'other'
        results.value = { album: [result({ mbid: 'z' })] }
        await flushPromises()
        expect(w.find('.selection-count').text()).toBe('1 selected')

        await w.find('.wishlist-clear-selection').trigger('click')
        expect(w.find('.search-selection-pill').exists()).toBe(false)
    })

    it('will not pick a result already on the wishlist, and links one in the library', async () => {
        const w = mountView()
        term.value = 'nirvana'
        results.value = {
            album: [result({ mbid: 'a', wishlist_id: 9 }), result({ mbid: 'b', in_library: true, match_id: 4 })]
        }
        await flushPromises()
        const rows = w.findAll('.result-row')
        expect(rows[0].text()).toContain('On wishlist')
        await rows[0].find('.result-pick').trigger('click')
        expect(w.find('.search-selection-pill').exists()).toBe(false)
        expect(rows[0].find('.result-pick').attributes('aria-describedby')).toBeTruthy()
        expect(rows[1].find('.library-badge').exists()).toBe(true)
    })

    it('resets the box, the results and the filter', async () => {
        route.query = { kind: 'song' }
        const w = mountView()
        await typeAndSubmit(w, 'lithium')
        await w.find('button.reset-search').trigger('click')
        expect(clear).toHaveBeenCalled()
        expect((w.find('input.search-input').element as HTMLInputElement).value).toBe('')
        expect(replace).toHaveBeenLastCalledWith({ query: {} })
    })

    it('offers the manual form before any search, and adds by hand back to the wishlist', async () => {
        const w = mountView()
        const form = w.findComponent({ name: 'WishlistManualForm' })
        expect(form.exists()).toBe(true)

        form.vm.$emit('add', { kind: 'song', title: 'Radio session', note: '1993' })
        const [sent, opts] = mutate.mock.calls[0]
        expect(sent).toEqual([
            {
                kind: 'song',
                title: 'Radio session',
                note: '1993',
                mbid: '',
                artist: '',
                album: '',
                date: '',
                type: '',
                disambiguation: ''
            }
        ])
        opts.onSuccess([{ id: 3 }] as WishlistItem[])
        expect(push).toHaveBeenCalledWith({ name: 'wishlist' })
    })

    it('keeps the manual form below the results', async () => {
        const w = mountView()
        term.value = 'nirvana'
        results.value = { album: [result({ mbid: 'a' })] }
        await flushPromises()
        const html = w.html()
        expect(html.indexOf('result-row')).toBeLessThan(html.indexOf('manual-form'))
    })

    it('shows a rate limit as a warning', async () => {
        const w = mountView()
        term.value = 'nirvana'
        error.value = 'MusicBrainz is busy'
        rateLimited.value = true
        await flushPromises()
        expect(w.findComponent({ name: 'Message' }).props('severity')).toBe('warn')
    })
})
