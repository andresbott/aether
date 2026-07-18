import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
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
        getCoverArtUrl: (id: string, size?: number) => `/cover/${id}?size=${size}`,
        getAlbum: (id: string) =>
            Promise.resolve(
                (
                    {
                        a1: { id: 'a1', song: [{ id: 's1' }, { id: 's2' }] },
                        a2: { id: 'a2', song: [{ id: 's3' }] }
                    } as Record<string, unknown>
                )[id] ?? null
            )
    }
}))

const playAlbum = vi.fn()
const addMultipleToQueue = vi.fn()
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ playAlbum, addMultipleToQueue })
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
            stubs: { ContentScaffold: ScaffoldStub, ConfirmDialog: true, RouterLink: true }
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
    playAlbum.mockClear()
    addMultipleToQueue.mockClear()
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
        await w.find('.hero-action-star').trigger('click')
        expect(toggleStarMutate).toHaveBeenCalledWith({ id: 'ar-1', starred: false })
    })

    it('Play gathers each album\'s songs then plays them in order', async () => {
        artist.value = {
            id: 'ar-1',
            name: 'Nirvana',
            album: [
                { id: 'a1', name: 'A1' },
                { id: 'a2', name: 'A2' }
            ]
        }
        const w = mountView()
        await w.find('.hero-action-play').trigger('click')
        await flushPromises()
        expect(playAlbum).toHaveBeenCalledWith([{ id: 's1' }, { id: 's2' }, { id: 's3' }])
    })

    it('Add to queue gathers songs then enqueues them', async () => {
        artist.value = {
            id: 'ar-1',
            name: 'Nirvana',
            album: [
                { id: 'a1', name: 'A1' },
                { id: 'a2', name: 'A2' }
            ]
        }
        const w = mountView()
        await w.find('.hero-action-queue').trigger('click')
        await flushPromises()
        expect(addMultipleToQueue).toHaveBeenCalledWith([{ id: 's1' }, { id: 's2' }, { id: 's3' }])
    })

    it('hides the hero actions in edit mode', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, coverArt: 'ar-1' }
        const w = mountView()
        expect(w.find('.hero-action-star').exists()).toBe(true)
        await enterEdit(w)
        expect(w.find('.hero-action-star').exists()).toBe(false)
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

    it('Cancel discards a staged cover without uploading and exits edit mode', async () => {
        artist.value = { id: 'ar-1', name: 'Nirvana', albumCount: 0, coverArt: 'ar-1' }
        const w = mountView()
        const file = new File(['x'], 'a.png', { type: 'image/png' })
        w.findComponent(FileUpload).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()
        expect(w.find('.flip-front img').attributes('src')).toContain('blob:')

        await w.find('.edit-action-edit').trigger('click')
        await w.find('.edit-action-cancel').trigger('click')
        expect(coverMutate).not.toHaveBeenCalled()
        // Staged preview is discarded — the hero reverts to the persisted cover URL.
        expect(w.find('.flip-front img').attributes('src')).toBe('/cover/ar-1?size=250')
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
