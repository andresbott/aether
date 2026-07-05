import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

const artist = ref<any>(null)
const isLoading = ref(false)
const error = ref<any>(null)
const toggleStarMutate = vi.fn()

vi.mock('@/composables/useSubsonicQueries', () => ({
    useArtist: () => ({ data: artist, isLoading, error }),
    useToggleStar: () => ({ mutate: toggleStarMutate })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size?: number) => `/cover/${id}?size=${size}`
    }
}))

vi.mock('vue-router', () => ({
    useRouter: () => ({ back: vi.fn() })
}))

const ScaffoldStub = {
    name: 'ContentScaffold',
    props: ['title', 'summary'],
    template: '<div class="scaffold"><slot name="actions" /><slot /></div>'
}
const DialogStub = {
    name: 'ArtistEditDialog',
    props: ['visible', 'artistId', 'artistName'],
    emits: ['update:visible', 'saved'],
    template: '<div class="dialog-stub" />'
}

import ArtistView from '@/views/ArtistView.vue'

const mountView = () =>
    mount(ArtistView, {
        props: { id: 'ar-1' },
        global: {
            plugins: [PrimeVue],
            stubs: { ContentScaffold: ScaffoldStub, ArtistEditDialog: DialogStub }
        }
    })

beforeEach(() => {
    artist.value = null
    isLoading.value = false
    error.value = null
    toggleStarMutate.mockClear()
})

describe('ArtistView', () => {
    it('shows a loading state and no scaffold while loading', () => {
        isLoading.value = true
        const w = mountView()
        expect(w.find('.loading').exists()).toBe(true)
        expect(w.findComponent(ScaffoldStub).exists()).toBe(false)
    })

    it('shows an error state on failure', () => {
        error.value = { message: 'boom' }
        const w = mountView()
        expect(w.find('.error').text()).toContain('boom')
    })

    it('passes the artist name as title and pluralized album count as summary', () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 3 }
        const w = mountView()
        const scaffold = w.findComponent(ScaffoldStub)
        expect(scaffold.props('title')).toBe('Nirvana')
        expect(scaffold.props('summary')).toBe('3 albums')
    })

    it('shows an empty summary when the artist has no albums', () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0 }
        const w = mountView()
        expect(w.findComponent(ScaffoldStub).props('summary')).toBe('')
    })

    it('opens the edit dialog from the header pencil button', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0 }
        const w = mountView()
        const dialog = w.findComponent(DialogStub)
        expect(dialog.props('visible')).toBe(false)
        const editBtn = w.findAll('button').find((b) => b.attributes('title') === 'Edit MusicBrainz match')
        expect(editBtn).toBeTruthy()
        await editBtn!.trigger('click')
        expect(w.findComponent(DialogStub).props('visible')).toBe(true)
    })

    it('toggles star on click', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, starred: undefined }
        const w = mountView()
        const starBtn = w.findAll('button').find((b) => b.attributes('title') === 'Toggle star')
        await starBtn!.trigger('click')
        expect(toggleStarMutate).toHaveBeenCalledWith({ id: 'ar-1', starred: false })
    })
})
