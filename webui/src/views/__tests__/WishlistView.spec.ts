import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { WishlistItem } from '@/types/wishlist'

const route = { query: {} as Record<string, string> }
const replace = vi.fn()
const push = vi.fn()
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ replace, push })
}))

const items = ref<WishlistItem[] | undefined>([])
const isLoading = ref(false)
const error = ref<Error | null>(null)
vi.mock('@/composables/useWishlist', () => ({
    useWishlist: () => ({ data: items, isLoading, error })
}))

const ScaffoldStub = {
    name: 'ContentScaffold',
    props: ['title', 'summary'],
    template: '<div><slot name="actions" /><slot /></div>'
}
const RowStub = { name: 'WishlistRow', props: ['item'], template: '<div class="row-stub">{{ item.title }}</div>' }
// The real VirtualScroller measures a viewport jsdom does not have; render every item.
const ScrollerStub = {
    name: 'VirtualScroller',
    props: ['items'],
    template: '<div><template v-for="item in items" :key="item.id"><slot name="item" :item="item" /></template></div>'
}
import WishlistView from '@/views/WishlistView.vue'
import SelectButton from 'primevue/selectbutton'

const entry = (id: number, kind: WishlistItem['kind'], inLibrary = false): WishlistItem => ({
    id,
    kind,
    mbid: `m-${id}`,
    title: `T${id}`,
    artist: 'A',
    album: '',
    date: '',
    type: '',
    disambiguation: '',
    note: '',
    in_library: inLibrary,
    found_at: null,
    created_at: '2026-01-01T00:00:00Z'
})

const mountView = () =>
    mount(WishlistView, {
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: {
                ContentScaffold: ScaffoldStub,
                WishlistRow: RowStub,
                VirtualScroller: ScrollerStub,
                Button: {
                    props: ['label'],
                    inheritAttrs: false,
                    template: '<button :class="$attrs.class" @click="$emit(\'click\')">{{ label }}</button>'
                }
            }
        }
    })

beforeEach(() => {
    replace.mockReset()
    push.mockReset()
    route.query = {}
    items.value = []
    isLoading.value = false
    error.value = null
})

describe('WishlistView', () => {
    it('passes the title and a pluralized count with the found share', () => {
        items.value = [entry(1, 'album', true), entry(2, 'song'), entry(3, 'artist')]
        const scaffold = mountView().findComponent(ScaffoldStub)
        expect(scaffold.props('title')).toBe('Wishlist')
        expect(scaffold.props('summary')).toBe('3 items · 1 in library')
    })

    it('uses the singular unit and leaves out a zero found share', () => {
        items.value = [entry(1, 'album')]
        expect(mountView().findComponent(ScaffoldStub).props('summary')).toBe('1 item')
    })

    it('passes an empty summary and shows the empty state when there is nothing', () => {
        const w = mountView()
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe('')
        expect(w.text()).toContain('Your wishlist is empty')
    })

    it('renders one row per entry', () => {
        items.value = [entry(1, 'album'), entry(2, 'song')]
        expect(mountView().findAll('.row-stub')).toHaveLength(2)
    })

    it('narrows to the kind in the query, summary included', () => {
        route.query = { kind: 'song' }
        items.value = [entry(1, 'album'), entry(2, 'song')]
        const w = mountView()
        expect(w.findAll('.row-stub').map((r) => r.text())).toEqual(['T2'])
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe('1 item')
    })

    it('says which kind is missing when a filter matches nothing', () => {
        route.query = { kind: 'artist' }
        items.value = [entry(1, 'album')]
        expect(mountView().text()).toContain('No artists on your wishlist')
    })

    it('writes the kind filter to the query, dropping it for All', async () => {
        const w = mountView()
        w.findComponent(SelectButton).vm.$emit('update:modelValue', 'album')
        expect(replace).toHaveBeenCalledWith({ query: { kind: 'album' } })
        route.query = { kind: 'album' }
        w.findComponent(SelectButton).vm.$emit('update:modelValue', 'all')
        expect(replace).toHaveBeenLastCalledWith({ query: {} })
    })

    it('the add button opens the search on the kind being shown', async () => {
        route.query = { kind: 'song' }
        await mountView().find('.add-wishlist-item').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'wishlist-new', query: { kind: 'song' } })
    })

    it('shows loading and a distinct error state', () => {
        isLoading.value = true
        expect(mountView().find('.loading').exists()).toBe(true)
        isLoading.value = false
        error.value = new Error('boom')
        const w = mountView()
        expect(w.text()).toContain('Could not load the wishlist')
        expect(w.text()).not.toContain('Your wishlist is empty')
    })
})
