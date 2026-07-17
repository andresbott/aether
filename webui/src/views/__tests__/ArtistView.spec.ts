import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import FileUpload from 'primevue/fileupload'
import Message from 'primevue/message'

const artist = ref<any>(null)
const isLoading = ref(false)
const error = ref<any>(null)
const toggleStarMutate = vi.fn()
const coverIsPending = ref(false)
// Invokes onSuccess so the component bumps its cache-bust after a save.
const coverMutate = vi.fn((_payload: unknown, opts?: { onSuccess?: () => void }) =>
    opts?.onSuccess?.()
)

vi.mock('@/composables/useSubsonicQueries', () => ({
    useArtist: () => ({ data: artist, isLoading, error }),
    useToggleStar: () => ({ mutate: toggleStarMutate }),
    useUpdateArtistCover: () => ({ mutate: coverMutate, isPending: coverIsPending })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size?: number) => `/cover/${id}?size=${size}`
    }
}))

vi.mock('vue-router', () => ({
    useRouter: () => ({ back: vi.fn() }),
    onBeforeRouteLeave: vi.fn()
}))

vi.mock('primevue/useconfirm', () => ({
    useConfirm: () => ({ require: (opts: { accept: () => void }) => opts.accept() })
}))

const ScaffoldStub = {
    name: 'ContentScaffold',
    props: ['title', 'summary'],
    template: '<div class="scaffold"><slot name="actions" /><slot /></div>'
}

import ArtistView from '@/views/ArtistView.vue'

const mountView = () =>
    mount(ArtistView, {
        props: { id: 'ar-1' },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { ContentScaffold: ScaffoldStub, ConfirmDialog: true }
        }
    })

const enterEdit = async (w: ReturnType<typeof mountView>) => {
    await w.find('.edit-action-edit').trigger('click')
}

beforeEach(() => {
    artist.value = null
    isLoading.value = false
    error.value = null
    toggleStarMutate.mockClear()
    coverMutate.mockClear()
    coverIsPending.value = false
    global.URL.createObjectURL = vi.fn(() => 'blob:mock')
    global.URL.revokeObjectURL = vi.fn()
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

    it('shows no scaffold title text and the artist name in the hero', () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 3 }
        const w = mountView()
        expect(w.findComponent(ScaffoldStub).props('title')).toBe('')
        expect(w.find('.hero-name').text()).toBe('Nirvana')
    })

    it('shows album and song counts in the hero meta row', () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 3, songCount: 20 }
        const w = mountView()
        const meta = w.find('.meta-row').text()
        expect(meta).toContain('3 albums')
        expect(meta).toContain('20 songs')
    })

    it('toggles star on click', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, starred: undefined }
        const w = mountView()
        const starBtn = w.findAll('button').find((b) => b.attributes('title') === 'Toggle star')
        await starBtn!.trigger('click')
        expect(toggleStarMutate).toHaveBeenCalledWith({ id: 'ar-1', starred: false })
    })

    it('renders the hero image and the flip-back cover controls', () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, coverArt: 'ar-1' }
        const w = mountView()
        expect(w.find('.flip-front img').attributes('src')).toBe('/cover/ar-1?size=250')
        expect(w.findComponent(FileUpload).exists()).toBe(true)
        expect(w.find('.cover-remove').exists()).toBe(true)
    })

    it('the pencil toggles the hero into edit mode (flips the cover)', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, coverArt: 'ar-1' }
        const w = mountView()
        expect(w.find('.hero-cover').classes()).not.toContain('flipped')
        await enterEdit(w)
        expect(w.find('.hero-cover').classes()).toContain('flipped')
    })

    it('has no Delete button (artists cannot be deleted)', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, coverArt: 'ar-1' }
        const w = mountView()
        await enterEdit(w)
        expect(w.find('.edit-action-delete').exists()).toBe(false)
    })

    it('selecting a file stages a preview and Save uploads it via updateArtistCover', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, coverArt: 'ar-1' }
        const w = mountView()
        const file = new File(['x'], 'a.png', { type: 'image/png' })
        w.findComponent(FileUpload).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()
        expect(w.find('.flip-front img').attributes('src')).toContain('blob:')
        expect(coverMutate).not.toHaveBeenCalled()

        await enterEdit(w)
        await w.find('.edit-action-save').trigger('click')
        expect(coverMutate).toHaveBeenCalledWith(
            expect.objectContaining({ artistId: 'ar-1', coverFile: file }),
            expect.anything()
        )
    })

    it('rejects an oversize file: shows an error and does not upload', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, coverArt: 'ar-1' }
        const w = mountView()
        const file = new File(['x'], 'big.png', { type: 'image/png' })
        Object.defineProperty(file, 'size', { value: 6 * 1024 * 1024 })
        w.findComponent(FileUpload).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()
        expect(coverMutate).not.toHaveBeenCalled()
        expect(w.findComponent(Message).exists()).toBe(true)
    })

    it('Remove stages a cover clear that Save commits', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, coverArt: 'ar-1' }
        const w = mountView()
        await w.find('.cover-remove').trigger('click')
        expect(coverMutate).not.toHaveBeenCalled()

        await enterEdit(w)
        await w.find('.edit-action-save').trigger('click')
        expect(coverMutate).toHaveBeenCalledWith(
            expect.objectContaining({ artistId: 'ar-1', coverClear: true }),
            expect.anything()
        )
    })
})
