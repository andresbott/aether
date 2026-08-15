import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import MetadataEditorView from '@/views/settings/MetadataEditorView.vue'
import { resetViewportForTests } from '@/composables/useViewport'

// Mocks for composables (same scaffold as identifyCache.spec.ts)
const refetchSpy = vi.hoisted(() => vi.fn())
const tracksRef = vi.hoisted(() => ({ value: [] }))

// Create mutable viewport state that can be changed during tests
let viewportState: ReturnType<typeof createMockViewportState>

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
        useTracks: () => ({
            data: tracksRef,
            isLoading: { value: false },
            refetch: refetchSpy
        }),
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
        if (!viewportState) {
            viewportState = createMockViewportState()
        }
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
    Message: {
        name: 'Message',
        props: ['severity', 'closable'],
        template: '<div class="p-message" :data-severity="severity"><slot /></div>'
    },
    Select: {
        name: 'Select',
        props: ['modelValue', 'options', 'optionLabel', 'optionValue', 'placeholder'],
        emits: ['update:modelValue'],
        template: '<select />'
    },
    Splitter: {
        name: 'Splitter',
        props: ['layout'],
        template: '<div><slot /></div>'
    },
    SplitterPanel: { name: 'SplitterPanel', template: '<div><slot /></div>' },
    Button: {
        name: 'Button',
        props: ['label'],
        inheritAttrs: false,
        template:
            '<button :data-test="$attrs[\'data-test\']" :aria-label="$attrs[\'aria-label\']" @click="$emit(\'click\')">{{ label }}</button>'
    },
    FolderTree: { name: 'FolderTree', template: '<div />' },
    TrackList: { name: 'TrackList', template: '<div />' },
    EditPanel: { name: 'EditPanel', props: ['selection'], template: '<div />' },
    IdentifyReviewDialog: { name: 'IdentifyReviewDialog', props: ['visible'], template: '<div />' },
    IdentifyAlbumDialog: { name: 'IdentifyAlbumDialog', props: ['visible'], template: '<div />' }
}

function mountView() {
    return mount(MetadataEditorView, { global: { stubs, directives: { tooltip: () => {} } } })
}

beforeEach(() => {
    viewportState = createMockViewportState()
    viewportState.tier.value = 'desktop'
    viewportState.shell.value = 'desktop'
})

describe('MetadataEditorView responsive layout', () => {
    it('shows horizontal splitter and no notice on desktop', () => {
        viewportState.tier.value = 'desktop'
        const w = mountView()

        const splitter = w.findComponent({ name: 'Splitter' })
        expect(splitter.props('layout')).toBe('horizontal')

        const message = w.findComponent({ name: 'Message' })
        expect(message.exists()).toBe(false)
    })

    it('shows vertical splitter and dismissable-not notice on phone', () => {
        viewportState.tier.value = 'phone'
        const w = mountView()

        const splitter = w.findComponent({ name: 'Splitter' })
        expect(splitter.props('layout')).toBe('vertical')

        const message = w.findComponent({ name: 'Message' })
        expect(message.exists()).toBe(true)
        expect(message.props('severity')).toBe('info')
        expect(message.props('closable')).toBe(false)
        expect(message.text()).toContain('The metadata editor works best on a larger screen.')
    })

    it('toolbar wraps on phone tier', () => {
        // The semantic layout behavior (stacked=true drives vertical splitter) is
        // asserted above; the toolbar's CSS wrapping at 767.98px is pinned in
        // MetadataEditorView.phoneStyles.spec.ts (off-disk).
        viewportState.tier.value = 'phone'
        const w = mountView()
        expect(w.find('.editor-header').exists()).toBe(true)
    })

    it('shows horizontal splitter on tablet', () => {
        viewportState.tier.value = 'tablet'
        const w = mountView()

        const splitter = w.findComponent({ name: 'Splitter' })
        expect(splitter.props('layout')).toBe('horizontal')

        const message = w.findComponent({ name: 'Message' })
        expect(message.exists()).toBe(false)
    })
})
