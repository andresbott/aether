import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import MetadataEditorView from '@/views/settings/MetadataEditorView.vue'

// Same composable scaffold as MetadataEditorView.responsive.spec.ts.
const refetchSpy = vi.hoisted(() => vi.fn())
const tracksRef = vi.hoisted(() => ({ value: [] }))

let viewportState: { tier: ReturnType<typeof ref>; shell: ReturnType<typeof ref>; isTouch: ReturnType<typeof ref> }
function createMockViewportState() {
    return {
        tier: ref<'phone' | 'tablet' | 'desktop'>('desktop'),
        shell: ref<'desktop' | 'mobile'>('desktop'),
        isTouch: ref(false)
    }
}

vi.mock('@/composables/useMetadataEditor', async (importActual) => {
    const actual = await importActual<typeof import('@/composables/useMetadataEditor')>()
    return {
        ...actual,
        useTracks: () => ({ data: tracksRef, isLoading: { value: false }, refetch: refetchSpy }),
        useMetadataCapabilities: () => ({
            data: { value: { identify: true } },
            isPending: { value: false }
        }),
        useIdentifyTracks: () => ({ mutateAsync: vi.fn(), isPending: { value: false } }),
        useIdentifyAlbum: () => ({ mutateAsync: vi.fn(), isPending: { value: false } }),
        useApplyPicture: () => ({ mutateAsync: vi.fn() }),
        useDeletePicture: () => ({ mutateAsync: vi.fn() })
    }
})
vi.mock('@/composables/useLibraries', () => ({
    useLibraries: () => ({ data: { value: [{ id: 1, name: 'Music' }] } })
}))
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => {
        if (!viewportState) viewportState = createMockViewportState()
        return viewportState
    },
    resetViewportForTests: vi.fn(() => {
        viewportState = createMockViewportState()
    })
}))
vi.mock('@tanstack/vue-query', async (importActual) => {
    const actual = await importActual<typeof import('@tanstack/vue-query')>()
    return { ...actual, useQueryClient: () => ({ invalidateQueries: vi.fn() }) }
})
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))
vi.mock('primevue/useconfirm', () => ({
    useConfirm: () => ({ require: (opts: { accept?: () => void }) => opts.accept?.() })
}))
vi.mock('vue-router', () => ({ onBeforeRouteLeave: () => {} }))

const stubs = {
    Dialog: {
        name: 'Dialog',
        props: ['visible'],
        template: '<div v-if="visible"><slot /><slot name="footer" /></div>'
    },
    ConfirmDialog: true,
    Message: { name: 'Message', props: ['severity', 'closable'], template: '<div><slot /></div>' },
    Select: {
        name: 'Select',
        props: ['modelValue', 'options', 'optionLabel', 'optionValue', 'placeholder'],
        emits: ['update:modelValue'],
        template: '<select />'
    },
    Splitter: { name: 'Splitter', props: ['layout'], template: '<div><slot /></div>' },
    SplitterPanel: { name: 'SplitterPanel', template: '<div><slot /></div>' },
    Button: {
        name: 'Button',
        props: ['label'],
        inheritAttrs: false,
        template:
            '<button :data-test="$attrs[\'data-test\']" :aria-label="$attrs[\'aria-label\']" @click="$emit(\'click\')">{{ label }}</button>'
    },
    FolderTree: {
        name: 'FolderTree',
        props: ['libraryId', 'expandTo'],
        emits: ['select'],
        template: '<div />'
    },
    TrackList: { name: 'TrackList', template: '<div />' },
    EditPanel: { name: 'EditPanel', props: ['selection'], template: '<div />' },
    IdentifyReviewDialog: { name: 'IdentifyReviewDialog', props: ['visible'], template: '<div />' },
    IdentifyAlbumDialog: { name: 'IdentifyAlbumDialog', props: ['visible'], template: '<div />' }
}

function mountView() {
    return mount(MetadataEditorView, { global: { stubs, directives: { tooltip: () => {} } } })
}

async function clickSelectFolder(w: ReturnType<typeof mountView>) {
    const btn = w.findAll('button').find((b) => b.text() === 'Select folder')
    await btn!.trigger('click')
    await flushPromises()
}

// Mount, open the picker (the sole library auto-selects — no list to click),
// and select Artist/Album as the folder, leaving the breadcrumb showing
// Music / Artist / Album (dialog closed).
async function selectArtistAlbum() {
    const w = mountView()
    await clickSelectFolder(w)
    w.findComponent({ name: 'FolderTree' }).vm.$emit('select', 'Artist/Album')
    await flushPromises()
    return w
}

const ftExpandTo = (w: ReturnType<typeof mountView>) =>
    w.findComponent({ name: 'FolderTree' }).props('expandTo') ?? null

beforeEach(() => {
    viewportState = createMockViewportState()
})

describe('MetadataEditorView breadcrumb', () => {
    it('renders one clickable crumb for the library and each path segment', async () => {
        const w = await selectArtistAlbum()
        const crumbs = w.findAll('[data-test^="crumb-"]')
        expect(crumbs.map((c) => c.text())).toEqual(['Music', 'Artist', 'Album'])
    })

    it('opens the picker expanded to a clicked path segment', async () => {
        const w = await selectArtistAlbum()
        await w.find('[data-test="crumb-1"]').trigger('click')
        await flushPromises()
        expect(w.findComponent({ name: 'FolderTree' }).exists()).toBe(true)
        expect(ftExpandTo(w)).toBe('Artist')
    })

    it('opens the picker at the library root when the library crumb is clicked', async () => {
        const w = await selectArtistAlbum()
        await w.find('[data-test="crumb-0"]').trigger('click')
        await flushPromises()
        expect(ftExpandTo(w)).toBe('')
    })

    it('the Select folder button opens at root even after a segment set a path', async () => {
        const w = await selectArtistAlbum()
        await w.find('[data-test="crumb-1"]').trigger('click')
        await flushPromises()
        await clickSelectFolder(w)
        expect(ftExpandTo(w)).toBe(null)
    })
})
