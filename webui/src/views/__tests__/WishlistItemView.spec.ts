import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { WishlistItem } from '@/types/wishlist'

const push = vi.fn()
const back = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ push, back }),
    onBeforeRouteLeave: vi.fn()
}))

const item = ref<WishlistItem | undefined>(undefined)
const isLoading = ref(false)
const isError = ref(false)
const updateMutate = vi.fn()
const deleteMutate = vi.fn()
vi.mock('@/composables/useWishlist', () => ({
    useWishlistItem: () => ({ data: item, isLoading, isError }),
    useUpdateWishlistItem: () => ({ mutate: updateMutate, isPending: ref(false) }),
    useDeleteWishlistItem: () => ({ mutate: deleteMutate })
}))
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => true, getCoverArtUrl: (id: string) => `cover:${id}` }
}))

const ScaffoldStub = { name: 'ContentScaffold', template: '<div><slot name="actions" /><slot /></div>' }
const HeroStub = {
    name: 'HeroHeader',
    props: ['eyebrow', 'coverUrl', 'editing', 'roundCover'],
    template:
        '<div><slot name="edit-actions" /><slot v-if="!editing" name="read" /><slot v-else name="edit" /><slot v-if="!editing" name="actions" /></div>'
}
const EditBarStub = { name: 'EditActionBar', props: ['editing', 'dirty'], template: '<div class="edit-bar" />' }
const RouterLinkStub = {
    name: 'RouterLink',
    props: ['to', 'custom'],
    template: '<div class="router-link-stub"><slot :navigate="() => {}" /></div>'
}
import WishlistItemView from '@/views/WishlistItemView.vue'

const entry = (over: Partial<WishlistItem>): WishlistItem => ({
    id: 5,
    kind: 'album',
    mbid: '1b022e01-4da6-387b-8658-8678046e4cef',
    title: 'Nevermind',
    artist: 'Nirvana',
    album: '',
    date: '1991-09-24',
    type: 'Album',
    disambiguation: '',
    note: '',
    in_library: false,
    found_at: null,
    created_at: '2026-01-01T00:00:00Z',
    ...over
})

const mountView = () =>
    mount(WishlistItemView, {
        props: { id: '5' },
        global: {
            plugins: [PrimeVue],
            stubs: {
                ContentScaffold: ScaffoldStub,
                HeroHeader: HeroStub,
                EditActionBar: EditBarStub,
                ConfirmDialog: true,
                RouterLink: RouterLinkStub,
                Button: { props: ['label'], template: '<button class="btn">{{ label }}</button>' }
            }
        }
    })

beforeEach(() => {
    item.value = undefined
    isLoading.value = false
    isError.value = false
    push.mockReset()
    updateMutate.mockReset()
    deleteMutate.mockReset()
})

describe('WishlistItemView', () => {
    it('shows a missing entry as not in the library, with only the MusicBrainz link', () => {
        item.value = entry({})
        const w = mountView()
        expect(w.findComponent(HeroStub).props('eyebrow')).toBe('Wishlist · Album')
        expect(w.findComponent(HeroStub).props('coverUrl')).toBeNull()
        expect(w.text()).toContain('Not in your library yet')
        expect(w.find('.router-link-stub').exists()).toBe(false)
        expect(w.find('.open-musicbrainz').attributes('href')).toBe(
            'https://musicbrainz.org/release-group/1b022e01-4da6-387b-8658-8678046e4cef'
        )
    })

    it('links a found album to the album and wears its cover', () => {
        item.value = entry({ in_library: true, match_id: 12, found_at: '2026-02-03T10:00:00Z' })
        const w = mountView()
        expect(w.text()).toContain('In your library since')
        const link = w.findComponent(RouterLinkStub)
        expect(link.props('to')).toEqual({ name: 'album', params: { id: 'al-12' } })
        expect(link.text()).toBe('Open album')
        expect(w.findComponent(HeroStub).props('coverUrl')).toBe('cover:al-12')
    })

    it('sends a found song to its album', () => {
        item.value = entry({ kind: 'song', title: 'Lithium', album: 'Nevermind', in_library: true, match_id: 40, match_album_id: 12 })
        const link = mountView().findComponent(RouterLinkStub)
        expect(link.props('to')).toEqual({ name: 'album', params: { id: 'al-12' } })
        expect(link.text()).toBe('Open on its album')
    })

    it('explains an entry added by hand can not be found, with no MusicBrainz link', () => {
        item.value = entry({ mbid: '', type: '', date: '', note: 'radio rip' })
        const w = mountView()
        expect(w.text()).toContain('Added by hand')
        expect(w.text()).toContain('radio rip')
        expect(w.find('.open-musicbrainz').exists()).toBe(false)
    })

    it('shows an artist with a round cover', () => {
        item.value = entry({ kind: 'artist', title: 'Nirvana', artist: '' })
        expect(mountView().findComponent(HeroStub).props('roundCover')).toBe(true)
    })

    it('saves the edited note', async () => {
        item.value = entry({ note: 'old' })
        const w = mountView()
        w.findComponent(EditBarStub).vm.$emit('update:editing', true)
        await w.vm.$nextTick()
        await w.find('.wishlist-note').setValue('the 2011 remaster')
        expect(w.findComponent(EditBarStub).props('dirty')).toBe(true)
        w.findComponent(EditBarStub).vm.$emit('save')
        expect(updateMutate).toHaveBeenCalledWith({ id: 5, patch: { note: 'the 2011 remaster' } }, expect.anything())
    })

    it('edits only the description of a MusicBrainz pick', async () => {
        item.value = entry({})
        const w = mountView()
        w.findComponent(EditBarStub).vm.$emit('update:editing', true)
        await w.vm.$nextTick()
        expect(w.find('.wishlist-title').exists()).toBe(false)
        expect(w.find('.wishlist-kind').exists()).toBe(false)
        expect(w.find('.wishlist-note').exists()).toBe(true)
    })

    it('edits every field of an entry added by hand', async () => {
        item.value = entry({ mbid: '', kind: 'song', title: 'Radio sesion', type: '', date: '', note: '1993' })
        const w = mountView()
        w.findComponent(EditBarStub).vm.$emit('update:editing', true)
        await w.vm.$nextTick()

        await w.find('.wishlist-title').setValue('  Radio session ')
        w.findComponent({ name: 'SelectButton' }).vm.$emit('update:modelValue', 'album')
        await w.find('.wishlist-note').setValue('BBC, 1993')
        await w.vm.$nextTick()
        expect(w.findComponent(EditBarStub).props('dirty')).toBe(true)

        w.findComponent(EditBarStub).vm.$emit('save')
        expect(updateMutate).toHaveBeenCalledWith(
            { id: 5, patch: { kind: 'album', title: 'Radio session', note: 'BBC, 1993' } },
            expect.anything()
        )
    })

    it('will not save an entry added by hand without a title', async () => {
        item.value = entry({ mbid: '', type: '', date: '' })
        const w = mountView()
        w.findComponent(EditBarStub).vm.$emit('update:editing', true)
        await w.vm.$nextTick()
        await w.find('.wishlist-title').setValue('   ')
        w.findComponent(EditBarStub).vm.$emit('save')
        expect(updateMutate).not.toHaveBeenCalled()
    })

    it('removes the entry and returns to the wishlist', () => {
        item.value = entry({})
        const w = mountView()
        w.findComponent(EditBarStub).vm.$emit('delete')
        const [id, opts] = deleteMutate.mock.calls[0]
        expect(id).toBe(5)
        opts.onSuccess()
        expect(push).toHaveBeenCalledWith({ name: 'wishlist' })
    })

    it('reports a missing entry', () => {
        expect(mountView().text()).toContain('Wishlist entry not found')
    })
})
