import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { Library } from '@/types/libraries'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

// A stable spy so a test can inspect what confirm.require() was called with —
// the mock factory below must keep returning the SAME spy across the panel's
// (single) useConfirm() call, not a fresh vi.fn() per mock invocation.
const confirmRequire = vi.hoisted(() => vi.fn())
vi.mock('primevue/useconfirm', () => ({ useConfirm: () => ({ require: confirmRequire }) }))

// The panel destructures these, so the mocks must hand back real refs —
// a plain { value } object does not unwrap in the template.
const libraries = vi.hoisted(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return { current: [] as any[] }
})

// The mutation doubles are shared so a test can drive .error and assert .reset,
// which the panel now reads (pass the error to the dialog) and calls (clear a
// stale error when the dialog opens).
const mutations = vi.hoisted(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return { create: null as any, update: null as any }
})

vi.mock('@/composables/useLibraries', async () => {
    const { ref: vueRef } = await import('vue')
    mutations.create = { mutate: vi.fn(), isPending: vueRef(false), error: vueRef(null), reset: vi.fn() }
    mutations.update = { mutate: vi.fn(), isPending: vueRef(false), error: vueRef(null), reset: vi.fn() }
    return {
        useLibraries: () => ({ data: vueRef(libraries.current), isLoading: vueRef(false) }),
        useCreateLibrary: () => mutations.create,
        useUpdateLibrary: () => mutations.update,
        useDeleteLibrary: () => ({ mutate: vi.fn(), isPending: vueRef(false), error: vueRef(null), reset: vi.fn() })
    }
})

import LibrariesPanel from '@/components/admin/LibrariesPanel.vue'
import LibraryDialog from '@/components/admin/LibraryDialog.vue'

// Records what each tooltip binding carried, as a plain attribute — the real
// PrimeVue Tooltip directive stores the value on a JS property, not the DOM,
// so content can't be asserted through it (only presence can, via
// data-pd-tooltip). A falsy value sets no attribute, mirroring the real
// directive. Mirrors FieldRow.spec.ts's recorder.
const tooltipRecorder = {
    mounted(el: HTMLElement, binding: { value: unknown }) {
        if (binding.value) el.setAttribute('data-tooltip', String(binding.value))
    },
    updated(el: HTMLElement, binding: { value: unknown }) {
        if (binding.value) el.setAttribute('data-tooltip', String(binding.value))
        else el.removeAttribute('data-tooltip')
    }
}

function library(over: Partial<Library> = {}): Library {
    return {
        id: 1,
        name: 'Main',
        show_artists: true,
        default_view: 'albums',
        icon: 'folder',
        filters: [],
        created_at: '',
        updated_at: '',
        track_count: 0,
        ...over
    }
}

beforeEach(() => {
    mutations.create.error.value = null
    mutations.update.error.value = null
    mutations.create.reset.mockClear()
    mutations.update.reset.mockClear()
    confirmRequire.mockClear()
})

const mountPanel = (libs: Library[]) => {
    libraries.current = libs
    return mount(LibrariesPanel, {
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: tooltipRecorder },
            stubs: { teleport: true, ConfirmDialog: true, LibraryDialog: true }
        }
    })
}

// A failed create/update leaves its error on the mutation; the panel forwards
// that to the dialog so the field-level message shows on the offending input,
// and clears any stale error when the dialog is (re)opened.
describe('LibrariesPanel forwards validation errors to the dialog', () => {
    const addBtn = (w: ReturnType<typeof mountPanel>) =>
        w.findAll('button').find((b) => b.text().includes('Add library'))!

    it('passes the create mutation error down to the dialog', async () => {
        const w = mountPanel([])
        await addBtn(w).trigger('click')
        mutations.create.error.value = {
            response: { status: 422, data: { errors: [{ pointer: '/name', detail: 'name is required' }] } }
        }
        await flushPromises()
        expect(w.findComponent(LibraryDialog).props('error')).toBe(mutations.create.error.value)
    })

    it('resets both mutations when opening the dialog so no stale error shows', async () => {
        const w = mountPanel([])
        await addBtn(w).trigger('click')
        expect(mutations.create.reset).toHaveBeenCalled()
        expect(mutations.update.reset).toHaveBeenCalled()
    })
})

// A library owns no directory — it is a filtered view — so the table summarizes
// what the filters select instead of a path, one Tag per summarize() line.
describe('LibrariesPanel filter summary', () => {
    it('shows one tag per summarize(filters) line', async () => {
        const w = mountPanel([library({ filters: [{ field: 'genre', values: ['Jazz', 'Ambient'] }] })])
        await flushPromises()
        expect(w.text()).toContain('Genre: Jazz, Ambient')
    })

    it('renders a separate line per filter, in order', async () => {
        const w = mountPanel([
            library({
                filters: [
                    { field: 'release_type', values: [''] },
                    { field: 'compilation', values: ['false'] }
                ]
            })
        ])
        await flushPromises()
        expect(w.text()).toContain('Release type: (none)')
        expect(w.text()).toContain('Compilation: No')
    })

    it('shows "Whole catalog" when a library has no filters', async () => {
        const w = mountPanel([library({ filters: [] })])
        await flushPromises()
        expect(w.text()).toContain('Whole catalog')
    })
})

// `warnings` is the server's signal that a stored filter value no longer
// matches anything configured (e.g. a renamed/removed scan folder) — surfaced
// as a badge next to the name, never silently.
describe('LibrariesPanel warnings tag', () => {
    it('shows a "Needs attention" tag with the warnings\' details as its tooltip', async () => {
        const w = mountPanel([
            library({
                warnings: [{ pointer: '/filters/0/values/0', detail: 'scan folder "Old" is not configured' }]
            })
        ])
        await flushPromises()
        const tag = w.find('[data-test="warnings-tag"]')
        expect(tag.exists()).toBe(true)
        expect(tag.text()).toBe('Needs attention')
        expect(tag.attributes('data-tooltip')).toBe('scan folder "Old" is not configured')
    })

    it('joins multiple warning details into the tooltip', async () => {
        const w = mountPanel([
            library({
                warnings: [
                    { pointer: '/filters/0/values/0', detail: 'first problem' },
                    { pointer: '/filters/1/values/0', detail: 'second problem' }
                ]
            })
        ])
        await flushPromises()
        const tag = w.find('[data-test="warnings-tag"]')
        expect(tag.attributes('data-tooltip')).toBe('first problem\nsecond problem')
    })

    it('does not show the warnings tag when a library has an empty warnings list', async () => {
        const w = mountPanel([library({ warnings: [] })])
        await flushPromises()
        expect(w.find('[data-test="warnings-tag"]').exists()).toBe(false)
    })

    it('does not show the warnings tag when warnings is absent', async () => {
        // library() sets no `warnings` key at all unless told to — this is the
        // "never fetched/no issues" shape the API sends for a healthy library.
        const w = mountPanel([library({})])
        await flushPromises()
        expect(w.find('[data-test="warnings-tag"]').exists()).toBe(false)
    })
})

// Deleting a library only removes the saved filter — it owns no tracks — so the
// confirmation must not threaten track/star/play-history loss.
describe('LibrariesPanel delete confirmation', () => {
    it('asks to delete by name and says only the view is removed', async () => {
        const w = mountPanel([library({ id: 7, name: 'Jazz Picks' })])
        await flushPromises()
        const deleteBtn = w.findAll('button').find((b) => b.classes().includes('p-button-danger'))!
        await deleteBtn.trigger('click')
        expect(confirmRequire).toHaveBeenCalledWith(
            expect.objectContaining({
                message:
                    'Delete library "Jazz Picks"? Only this view is removed — no track, star or play history is touched.'
            })
        )
    })
})

// A library is a filtered view, not a directory: nothing here may still imply a
// path, a scan timestamp, or a config-provisioned badge (all removed pre-3b).
describe('LibrariesPanel no longer shows removed columns', () => {
    it('renders no Path, Last scan or config-badge copy', async () => {
        const w = mountPanel([library({ name: 'Main' })])
        await flushPromises()
        expect(w.text()).not.toContain('Path')
        expect(w.text()).not.toContain('Last scan')
        expect(w.text()).not.toContain('From config')
    })
})

describe('LibrariesPanel empty state', () => {
    it('shows the exact empty-state copy when there are no libraries', async () => {
        const w = mountPanel([])
        await flushPromises()
        expect(w.text()).toContain(
            'No libraries yet. A library is a filtered view over your music — add one to give clients a music folder to browse.'
        )
    })
})
