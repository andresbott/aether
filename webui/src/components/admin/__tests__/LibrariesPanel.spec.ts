import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { Library } from '@/types/libraries'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))
vi.mock('primevue/useconfirm', () => ({ useConfirm: () => ({ require: vi.fn() }) }))

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

beforeEach(() => {
    mutations.create.error.value = null
    mutations.update.error.value = null
    mutations.create.reset.mockClear()
    mutations.update.reset.mockClear()
})

const mountPanel = (libs: Library[]) => {
    libraries.current = libs
    return mount(LibrariesPanel, {
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
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
