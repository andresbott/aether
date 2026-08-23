import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import MetadataEditorView from '@/views/settings/MetadataEditorView.vue'

// Same composable scaffold the other MetadataEditorView specs use.
const tracksRef = vi.hoisted(() => ({ value: [] }))
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
// Two libraries, so the picker shows the library list (no auto-select) and the
// library can be changed to exercise the clear-on-change behaviour.
vi.mock('@/composables/useLibraries', () => ({
    useLibraries: () => ({
        data: { value: [{ id: 1, name: 'Music' }, { id: 2, name: 'Podcasts' }] }
    })
}))
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({ tier: ref('desktop'), shell: ref('desktop'), isTouch: ref(false) })
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
    Message: { name: 'Message', template: '<div><slot /></div>' },
    Listbox: {
        name: 'Listbox',
        props: ['modelValue'],
        emits: ['update:modelValue'],
        template: '<div />'
    },
    InputText: {
        name: 'InputText',
        props: ['modelValue'],
        emits: ['update:modelValue'],
        template:
            '<input data-test="folder-filter" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
    },
    Splitter: { name: 'Splitter', template: '<div><slot /></div>' },
    SplitterPanel: { name: 'SplitterPanel', template: '<div><slot /></div>' },
    Button: {
        name: 'Button',
        props: ['label'],
        inheritAttrs: false,
        template: '<button @click="$emit(\'click\')">{{ label }}</button>'
    },
    FolderTree: {
        name: 'FolderTree',
        props: ['libraryId', 'filter', 'expandTo'],
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
}

const folderTree = (w: ReturnType<typeof mountView>) => w.findComponent({ name: 'FolderTree' })
const listbox = (w: ReturnType<typeof mountView>) => w.findComponent({ name: 'Listbox' })

describe('MetadataEditorView folder search', () => {
    beforeEach(() => vi.useFakeTimers())
    afterEach(() => vi.useRealTimers())

    it('passes the debounced search text to FolderTree as its filter', async () => {
        const w = mountView()
        await openPicker(w)
        listbox(w).vm.$emit('update:modelValue', 1)
        await w.vm.$nextTick()
        await w.find('[data-test="folder-filter"]').setValue('up')

        // Nothing until the debounce elapses.
        expect(folderTree(w).props('filter')).toBe('')

        await vi.advanceTimersByTimeAsync(400)
        expect(folderTree(w).props('filter')).toBe('up')
    })

    it('clears the search when the library changes', async () => {
        const w = mountView()
        await openPicker(w)
        listbox(w).vm.$emit('update:modelValue', 1)
        await w.vm.$nextTick()
        await w.find('[data-test="folder-filter"]').setValue('up')
        await vi.advanceTimersByTimeAsync(400)
        expect(folderTree(w).props('filter')).toBe('up')

        // Switching library resets the picker, filter included.
        listbox(w).vm.$emit('update:modelValue', 2)
        await w.vm.$nextTick()
        expect(folderTree(w).props('filter')).toBe('')
    })
})
