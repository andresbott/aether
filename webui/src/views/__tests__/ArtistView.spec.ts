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

const imageSource = ref<any>(null)
const imageSourceRefetch = vi.fn()
vi.mock('@/composables/useArtistImageSource', () => ({
    useArtistImageSource: () => ({ data: imageSource, refetch: imageSourceRefetch })
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
import HeroHeader from '@/components/layout/HeroHeader.vue'
import { resetCoverVersions } from '@/composables/useCoverVersion'

// Records what v-tooltip was bound with, so specs can assert on the tooltip
// text the way the real directive would render it.
const recordingTooltip = {
    mounted(el: HTMLElement, binding: { value: unknown }) {
        const text = typeof binding.value === 'string' ? binding.value : undefined
        if (text) el.setAttribute('data-tooltip', text)
    }
}

const mountView = () =>
    mount(ArtistView, {
        props: { id: 'ar-1' },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: recordingTooltip },
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
    imageSource.value = null
    imageSourceRefetch.mockClear()
    // Cover versions are module-level (they must outlive a component), so they
    // leak between tests unless cleared.
    resetCoverVersions()
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
        imageSource.value = { source: 'upload', path: '', filename: 'cover.png' }
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
        imageSource.value = { source: 'upload', path: '', filename: 'cover.png' }
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

describe('ArtistView image-source note', () => {
    it('shows a note in edit mode when the image is read from the artist folder', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = {
            source: 'folder',
            path: '/music/Pink Floyd/artist.jpg',
            filename: 'artist.jpg'
        }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        const note = w.find('.image-source-note')
        expect(note.exists()).toBe(true)
        expect(note.text()).toContain('from music folder')
    })

    it('leads the note with an image icon and ends it with a help marker', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'folder', path: '/music/Pink Floyd/artist.jpg' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        const icons = w.findAll('.image-source-note i')
        expect(icons[0].classes()).toContain('pi-image')
        expect(icons[icons.length - 1].classes()).toContain('pi-question-circle')
    })

    // Only the "?" carries the tooltip, so hovering the label itself stays quiet.
    // It must go through PrimeVue's v-tooltip: a bare `title` attribute needs a
    // long hover on a small glyph and is easy to miss entirely.
    it('puts a served-from tooltip with the path on the help marker only', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'folder', path: '/music/Pink Floyd/artist.jpg' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        const note = w.find('.image-source-note')
        expect(note.text()).not.toContain('/music/Pink Floyd/artist.jpg')

        const hint = w.find('.image-source-help').attributes('data-tooltip') ?? ''
        expect(hint).toContain('served from /music/Pink Floyd/artist.jpg')
        // It also stands in for the hidden Remove button.
        expect(hint).toContain('will not delete it')
    })

    // The "?" is the folder case's affordance for the path; other sources have no
    // path to show, so they get no marker.
    it('shows no help marker for an image held in aether\'s store', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'upload', path: '', filename: 'cover.png' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()
        expect(w.find('.image-source-note').exists()).toBe(true)
        expect(w.find('.image-source-help').exists()).toBe(false)
    })

    it('hides the note outside edit mode', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = {
            source: 'folder',
            path: '/music/Pink Floyd/artist.jpg',
            filename: 'artist.jpg'
        }
        const w = mountView()
        await flushPromises()
        expect(w.find('.image-source-note').exists()).toBe(false)
    })

    // Staging a replacement swaps the note over to the picked file rather than
    // leaving the folder image described.
    it('stops describing the folder image once a replacement is staged', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = {
            source: 'folder',
            path: '/music/Pink Floyd/artist.jpg',
            filename: 'artist.jpg'
        }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()
        expect(w.find('.image-source-note').text()).toContain('from music folder')

        const file = new File(['x'], 'a.png', { type: 'image/png' })
        w.findComponent(FileUpload).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()
        const note = w.find('.image-source-note')
        expect(note.text()).not.toContain('from music folder')
        expect(note.text()).toContain('a.png')
    })
})

describe('ArtistView image-source refresh', () => {
    it('refetches the image source after saving a new image', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'folder', path: '/music/Pink Floyd/artist.jpg' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        const file = new File(['x'], 'a.png', { type: 'image/png' })
        w.findComponent(FileUpload).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()
        await w.find('.edit-action-save').trigger('click')
        await flushPromises()

        expect(imageSourceRefetch).toHaveBeenCalled()
    })
})

// Remove clears aether's own stored image. When the served image is a file from
// the music folder there is nothing of aether's to remove — and aether must not
// touch the user's file — so the control must not be usable.
describe('ArtistView remove with a folder image', () => {
    // Remove is hidden in this state (see "editor honesty"), so a clear can never
    // be staged — but the guard in onRemoveCover must hold even if it is reached.
    it('never stages a clear for an image aether does not own', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = {
            source: 'folder',
            path: '/music/Pink Floyd/artist.jpg',
            filename: 'artist.jpg'
        }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        w.findComponent(HeroHeader).vm.$emit('cover-remove')
        await w.vm.$nextTick()

        // The cover still shows the served image rather than going blank...
        expect(w.find('.flip-front img').attributes('src')).toBe('/cover/ar-1?size=250')
        // ...and Save has nothing to send.
        await w.find('.edit-action-save').trigger('click')
        expect(coverMutate).not.toHaveBeenCalled()
    })
})

// PrimeVue's FileUpload only ever says "No file chosen" — it knows nothing about
// an image already on the server. The note has to carry that state instead.
describe('ArtistView current-image label', () => {
    it('names an uploaded image', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'upload', path: '', filename: 'cover.png' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        const note = w.find('.image-source-note')
        expect(note.exists()).toBe(true)
        expect(note.text()).toContain('cover.png')
        expect(note.text()).toContain('uploaded')
    })

    it('labels an auto-fetched image as fetched', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'fetched', path: '', filename: 'cover.auto.jpg' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        expect(w.find('.image-source-note').text()).toContain('fetched automatically')
    })

    it('names the on-disk file for a folder image', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = {
            source: 'folder',
            path: '/music/Pink Floyd/artist.jpg',
            filename: 'artist.jpg'
        }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        const note = w.find('.image-source-note')
        expect(note.text()).toContain('artist.jpg')
        expect(note.text()).toContain('music folder')
    })

    // Removing an upload must read as a pending change, not silently leave the
    // old filename on screen.
    it('marks the note as pending removal once Remove is staged', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'upload', path: '', filename: 'cover.png' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        await w.find('.cover-remove').trigger('click')
        const note = w.find('.image-source-note')
        expect(note.classes()).toContain('is-pending')
        expect(note.text()).toContain('will be removed')
        expect(note.text()).toContain('cover.png')
    })

    it('names the picked file once a replacement is staged', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'upload', path: '', filename: 'cover.png' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        const file = new File(['x'], 'new-art.png', { type: 'image/png' })
        w.findComponent(FileUpload).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()

        const note = w.find('.image-source-note')
        expect(note.classes()).toContain('is-pending')
        expect(note.text()).toContain('new-art.png')
    })
})

// After saving an upload the editor must not imply there is nothing there, and
// after a reload with only a folder image it must not show a dead Remove button.
describe('ArtistView editor honesty', () => {
    it('shows no note at all when nothing is on file', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'none', path: '', filename: '' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        expect(w.find('.image-source-note').exists()).toBe(false)
    })

    // Remove is only meaningful for an image aether holds. For a folder image or
    // no image there is nothing to remove, so hide it rather than showing a
    // greyed-out control the user has to hover to understand.
    it('hides Remove when the image is not aether\'s to delete', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = {
            source: 'folder',
            path: '/music/Pink Floyd/artist.jpg',
            filename: 'artist.jpg'
        }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        expect(w.find('.cover-remove').exists()).toBe(false)
    })

    it('hides Remove when there is no image on file', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'none', path: '', filename: '' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        expect(w.find('.cover-remove').exists()).toBe(false)
    })

    it('shows Remove for an uploaded image', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'upload', path: '', filename: 'cover.png' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        const remove = w.find('.cover-remove')
        expect(remove.exists()).toBe(true)
        expect(remove.attributes('disabled')).toBeUndefined()
    })

    // An auto-fetched image is aether's own doing, not the user's upload — like a
    // music-folder image it is not theirs to delete from here, so no Remove.
    it('hides Remove for an auto-fetched image', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'fetched', path: '', filename: 'cover.auto.jpg' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        expect(w.find('.cover-remove').exists()).toBe(false)
    })

    // A staged pick must stay cancellable even when nothing is on the server.
    it('shows Remove once a file is staged over no image', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'none', path: '', filename: '' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        const file = new File(['x'], 'a.png', { type: 'image/png' })
        w.findComponent(FileUpload).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()

        expect(w.find('.cover-remove').exists()).toBe(true)
    })
})

// The browser's in-memory image cache never revalidates within an SPA session,
// so a changed cover has to change its URL — and that has to survive navigating
// away (the component, and any ref in it, is destroyed).
describe('ArtistView cover cache busting', () => {
    it('busts the cover url after a save, and keeps it busted across a remount', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'upload', path: '', filename: 'cover.png' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        const before = w.find('.flip-front img').attributes('src')
        expect(before).toBe('/cover/ar-1?size=250')

        const file = new File(['x'], 'a.png', { type: 'image/png' })
        w.findComponent(FileUpload).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()
        await w.find('.edit-action-save').trigger('click')
        await flushPromises()

        const after = w.find('.flip-front img').attributes('src')
        expect(after).not.toBe(before)

        // Navigate away and back: a fresh mount must still use the busted url.
        w.unmount()
        const w2 = mountView()
        await flushPromises()
        expect(w2.find('.flip-front img').attributes('src')).toBe(after)
    })
})

// With no Remove button, the "?" hint is the only place that can explain why an
// auto-fetched image cannot be dropped from here.
describe('ArtistView fetched-image hint', () => {
    it('explains on hover that a fetched image is not removable here', async () => {
        artist.value = { id: 'ar-1', name: 'Pink Floyd', albumCount: 1, coverArt: 'ar-1' }
        imageSource.value = { source: 'fetched', path: '', filename: 'cover.auto.jpg' }
        const w = mountView()
        await enterEdit(w)
        await flushPromises()

        const hint = w.find('.image-source-help').attributes('data-tooltip') ?? ''
        expect(hint).toContain('Upload an image to override it')
    })
})
