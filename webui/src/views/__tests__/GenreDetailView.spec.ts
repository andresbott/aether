import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'
import FileUpload from 'primevue/fileupload'

const genres = ref<any[]>([])
const items = ref<any[]>([])
const updateCoverMutate = vi.fn()
const updateCoverIsPending = ref(false)

vi.mock('@/composables/useSubsonicQueries', () => ({
    useGenres: () => ({ data: genres, isLoading: ref(false), error: ref(null) }),
    useUpdateGenreCover: () => ({ mutate: updateCoverMutate, isPending: updateCoverIsPending })
}))

vi.mock('@/composables/useGenreSongsTable', () => ({
    useGenreSongsTable: () => ({ items, ensureRange: vi.fn() }),
    GENRE_SONG_PAGE_SIZE: 100
}))

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        playAlbum: vi.fn(),
        addMultipleToQueue: vi.fn(),
        currentTrack: ref(null)
    })
}))

vi.mock('@/composables/useSongsDrag', () => ({
    useSongsDrag: () => ({ start: vi.fn(), end: vi.fn() })
}))

vi.mock('@/composables/useRowSelection', () => ({
    useRowSelection: () => ({
        isSelected: () => false,
        onRowClick: vi.fn(),
        selectionForDrag: () => [],
        clearSelection: vi.fn(),
        selectedCount: ref(0),
        selectedIndices: ref(new Set())
    })
}))

const isAdmin = ref(true)
vi.mock('@/composables/useAuth', () => ({
    useAuth: () => ({ isAdmin })
}))

const mockGetGeneratedCoverPreviewUrl = vi.fn()
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (art: string, size: number) => `cover:${art}:${size}`,
        getGeneratedCoverPreviewUrl: (...args: unknown[]) => mockGetGeneratedCoverPreviewUrl(...args)
    }
}))

vi.mock('vue-router', () => ({
    useRouter: () => ({ back: vi.fn() }),
    onBeforeRouteLeave: vi.fn()
}))

vi.mock('primevue/useconfirm', () => ({
    useConfirm: () => ({ require: (opts: { accept: () => void }) => opts.accept() })
}))

import GenreDetailView from '@/views/GenreDetailView.vue'
import { resetCoverVersions } from '@/composables/useCoverVersion'

const mountView = () =>
    mount(GenreDetailView, {
        props: { name: 'Jazz' },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: {
                VirtualScroller: { template: '<div><slot /></div>' },
                RouterLink: true,
                TrackActionSheet: {
                    name: 'TrackActionSheet',
                    props: ['song', 'visible'],
                    template: '<div />'
                },
                GenerateCoverDialog: {
                    name: 'GenerateCoverDialog',
                    props: ['visible', 'entityId', 'title'],
                    emits: ['update:visible', 'select'],
                    template: '<div class="generate-cover-dialog-stub" />'
                }
            }
        }
    })

const enterEdit = async (w: ReturnType<typeof mountView>) => {
    await w.find('.edit-action-edit').trigger('click')
}

beforeEach(() => {
    genres.value = [
        { value: 'Jazz', songCount: 100, albumCount: 20, coverArt: 'genre-jazz' }
    ]
    items.value = []
    updateCoverMutate.mockClear()
    updateCoverIsPending.value = false
    isAdmin.value = true
    resetCoverVersions()
    global.URL.createObjectURL = vi.fn(() => 'blob:mock')
    global.URL.revokeObjectURL = vi.fn()
    mockGetGeneratedCoverPreviewUrl.mockClear()
    mockGetGeneratedCoverPreviewUrl.mockReturnValue('generated:preview')
})

describe('GenreDetailView cover editing', () => {
    it('shows Generate button in edit mode and passes pick to mutation', async () => {
        const w = mountView()
        await enterEdit(w)

        const generateBtn = w.find('button[aria-label="Generate"]')
        expect(generateBtn.exists()).toBe(true)

        await generateBtn.trigger('click')
        await flushPromises()

        const dialog = w.findComponent({ name: 'GenerateCoverDialog' })
        expect(dialog.props('visible')).toBe(true)
        expect(dialog.props('entityId')).toBe('genre-jazz')

        dialog.vm.$emit('select', { style: 'classic', variation: 2 })
        await flushPromises()

        expect(mockGetGeneratedCoverPreviewUrl).toHaveBeenCalledWith({
            id: 'genre-jazz',
            style: 'classic',
            variation: 2,
            size: 512
        })

        await w.find('.edit-action-save').trigger('click')
        expect(updateCoverMutate).toHaveBeenCalledWith(
            {
                genreId: 'genre-jazz',
                coverFile: undefined,
                coverClear: undefined,
                generate: { style: 'classic', variation: 2 }
            },
            expect.anything()
        )
    })
})
