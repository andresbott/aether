import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import MetadataEditorView from '@/views/settings/MetadataEditorView.vue'

// Same composable scaffold as the other MetadataEditorView specs, but the
// library set is mutable so each test can configure one or several libraries.
const tracksRef = vi.hoisted(() => ({ value: [] as unknown[] }))
const librariesRef = vi.hoisted(() => ({ value: [] as { id: number; name: string }[] }))

let viewportState: {
    tier: ReturnType<typeof ref>
    shell: ReturnType<typeof ref>
    isTouch: ReturnType<typeof ref>
}
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
        useTracks: () => ({ data: tracksRef, isLoading: { value: false }, refetch: vi.fn() }),
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
    useLibraries: () => ({ data: librariesRef })
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
    // Kept so the pre-change component (which still renders Select) mounts
    // cleanly and the tests fail on their own assertions, not a PrimeVue crash.
    Select: {
        name: 'Select',
        props: ['modelValue', 'options', 'optionLabel', 'optionValue', 'placeholder'],
        emits: ['update:modelValue'],
        template: '<select />'
    },
    Listbox: {
        name: 'Listbox',
        props: ['modelValue', 'options', 'optionLabel', 'optionValue'],
        emits: ['update:modelValue'],
        template:
            '<ul class="listbox-stub"><li v-for="o in options" :key="o.value" :data-test="`lib-${o.value}`" @click="$emit(\'update:modelValue\', o.value)">{{ o.label }}</li></ul>'
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

async function openPicker(w: ReturnType<typeof mountView>) {
    const btn = w.findAll('button').find((b) => b.text() === 'Select folder')
    await btn!.trigger('click')
    await flushPromises()
}

const treeLibraryId = (w: ReturnType<typeof mountView>) =>
    w.findComponent({ name: 'FolderTree' }).props('libraryId') ?? null

beforeEach(() => {
    viewportState = createMockViewportState()
    tracksRef.value = []
    librariesRef.value = []
})

describe('MetadataEditorView library list', () => {
    it('renders a library list when more than one library is configured', async () => {
        librariesRef.value = [
            { id: 1, name: 'Music' },
            { id: 2, name: 'Podcasts' }
        ]
        const w = mountView()
        await openPicker(w)
        expect(w.findComponent({ name: 'Listbox' }).exists()).toBe(true)
        expect(w.findAll('[data-test^="lib-"]').map((n) => n.text())).toEqual(['Music', 'Podcasts'])
    })

    it('omits the library list when only one library is configured', async () => {
        librariesRef.value = [{ id: 1, name: 'Music' }]
        const w = mountView()
        await openPicker(w)
        expect(w.findComponent({ name: 'Listbox' }).exists()).toBe(false)
    })

    it('auto-selects the library when only one is configured', async () => {
        librariesRef.value = [{ id: 1, name: 'Music' }]
        const w = mountView()
        await flushPromises()
        await openPicker(w)
        expect(treeLibraryId(w)).toBe(1)
    })

    it('does not auto-select when several libraries are configured', async () => {
        librariesRef.value = [
            { id: 1, name: 'Music' },
            { id: 2, name: 'Podcasts' }
        ]
        const w = mountView()
        await flushPromises()
        await openPicker(w)
        expect(treeLibraryId(w)).toBe(null)
    })

    it('switches the active library when one is chosen from the list', async () => {
        librariesRef.value = [
            { id: 1, name: 'Music' },
            { id: 2, name: 'Podcasts' }
        ]
        const w = mountView()
        await openPicker(w)
        await w.find('[data-test="lib-2"]').trigger('click')
        await flushPromises()
        expect(treeLibraryId(w)).toBe(2)
    })
})
