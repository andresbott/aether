import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ref } from 'vue'
import ArtistImageSection from '@/views/settings/metadata-editor/ArtistImageSection.vue'
import type { EditSession } from '@/composables/useEditSession'

const resolveArtistFolderSpy = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api/Metadata', () => ({
    resolveArtistFolder: (...a: unknown[]) => resolveArtistFolderSpy(...a),
    getArtistImageUrl: () => 'http://img/artist.jpg'
}))

function mkSession(over: Partial<Record<string, unknown>> = {}) {
    return {
        picturesSavedAt: ref(0),
        getArtistImageOp: vi.fn(() => undefined),
        stageArtistImageSet: vi.fn(),
        stageArtistImageRemoval: vi.fn(),
        discardArtistImageOp: vi.fn(),
        ...over
    }
}

const stubs = {
    CollapsibleSection: { template: '<div data-test="artist-image-block"><slot /></div>' },
    ArtistImageSearchDialog: {
        name: 'ArtistImageSearchDialog',
        props: ['visible', 'artistName', 'allowUpload'],
        emits: ['update:visible', 'select', 'upload'],
        template: '<div class="dialog-stub" />'
    },
    Button: {
        props: ['label', 'icon', 'disabled'],
        emits: ['click'],
        template: '<button :disabled="disabled" @click="$emit(\'click\')">{{ label }}</button>'
    }
}

function mountSection(props: Record<string, unknown>, session = mkSession()) {
    const wrapper = mount(ArtistImageSection, {
        props: {
            libraryId: 1,
            session: session as unknown as EditSession,
            folderPath: 'Radiohead',
            ...props
        },
        global: { stubs }
    })
    return { wrapper, session }
}

describe('ArtistImageSection', () => {
    beforeEach(() => {
        resolveArtistFolderSpy.mockReset()
        resolveArtistFolderSpy.mockResolvedValue({
            eligible: true,
            artist: 'Radiohead',
            path: 'Radiohead',
            current_image: ''
        })
    })

    it('hides when the folder is not an artist folder', async () => {
        resolveArtistFolderSpy.mockResolvedValue({
            eligible: false,
            artist: '',
            path: '',
            current_image: ''
        })
        const { wrapper } = mountSection({})
        await flushPromises()
        expect(wrapper.find('[data-test="artist-image-block"]').exists()).toBe(false)
    })

    it('does not resolve without a selected folder', async () => {
        const { wrapper } = mountSection({ folderPath: null })
        await flushPromises()
        expect(resolveArtistFolderSpy).not.toHaveBeenCalled()
        expect(wrapper.find('[data-test="artist-image-block"]').exists()).toBe(false)
    })

    it('stages an online pick for the resolved folder', async () => {
        const { wrapper, session } = mountSection({})
        await flushPromises()
        await wrapper.find('[data-test="artist-image-change"]').trigger('click')
        wrapper.findComponent({ name: 'ArtistImageSearchDialog' }).vm.$emit('select', {
            mbid: 'mb-1',
            name: 'Radiohead',
            url: 'http://p/x.jpg',
            previewUrl: 'http://p/t.jpg'
        })
        expect(session.stageArtistImageSet).toHaveBeenCalledWith('Radiohead', {
            file: null,
            mbid: 'mb-1',
            url: 'http://p/x.jpg'
        })
    })

    it('keys staging off the resolved artist folder when a disc is selected', async () => {
        resolveArtistFolderSpy.mockResolvedValue({
            eligible: true,
            artist: 'Radiohead',
            path: 'Radiohead',
            current_image: ''
        })
        const { wrapper, session } = mountSection({ folderPath: 'Radiohead/OK Computer/CD 1' })
        await flushPromises()
        expect(resolveArtistFolderSpy).toHaveBeenCalledWith(1, 'Radiohead/OK Computer/CD 1')
        await wrapper.find('[data-test="artist-image-change"]').trigger('click')
        wrapper.findComponent({ name: 'ArtistImageSearchDialog' }).vm.$emit('select', {
            mbid: 'mb',
            name: 'Radiohead',
            url: 'http://p/x.jpg',
            previewUrl: 'http://p/t.jpg'
        })
        expect(session.stageArtistImageSet).toHaveBeenCalledWith('Radiohead', {
            file: null,
            mbid: 'mb',
            url: 'http://p/x.jpg'
        })
    })

    it('stages an uploaded file', async () => {
        const { wrapper, session } = mountSection({})
        await flushPromises()
        const file = new File(['x'], 'a.png', { type: 'image/png' })
        wrapper.findComponent({ name: 'ArtistImageSearchDialog' }).vm.$emit('upload', file)
        expect(session.stageArtistImageSet).toHaveBeenCalledWith('Radiohead', {
            file,
            mbid: null,
            url: null
        })
    })

    it('offers Remove for an existing image and stages a removal', async () => {
        resolveArtistFolderSpy.mockResolvedValue({
            eligible: true,
            artist: 'Radiohead',
            path: 'Radiohead',
            current_image: 'artist.jpg'
        })
        const { wrapper, session } = mountSection({})
        await flushPromises()
        await wrapper.find('[data-test="artist-image-remove"]').trigger('click')
        expect(session.stageArtistImageRemoval).toHaveBeenCalledWith('Radiohead')
    })

    it('shows Undo for a staged op and discards it', async () => {
        const session = mkSession({
            getArtistImageOp: vi.fn(() => ({
                kind: 'set',
                file: null,
                mbid: 'mb',
                url: 'http://p/x.jpg',
                preview: 'http://p/x.jpg'
            }))
        })
        const { wrapper } = mountSection({}, session)
        await flushPromises()
        expect(wrapper.find('[data-test="artist-image-change"]').exists()).toBe(false)
        await wrapper.find('[data-test="artist-image-undo"]').trigger('click')
        expect(session.discardArtistImageOp).toHaveBeenCalledWith('Radiohead')
    })
})
