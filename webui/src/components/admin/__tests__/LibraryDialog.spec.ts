import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

import LibraryDialog from '@/components/admin/LibraryDialog.vue'
import type { Library, LibraryInput } from '@/types/libraries'

const baseLibrary: Library = {
    id: 1,
    name: 'Main',
    show_artists: true,
    default_view: 'albums',
    icon: 'folder',
    filters: [],
    created_at: '',
    updated_at: '',
    track_count: 0
}

const mountDialog = (library: Library | null) =>
    mount(LibraryDialog, {
        props: { visible: true, library, submitting: false },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { teleport: true }
        }
    })

const mountWithError = (error: unknown) =>
    mount(LibraryDialog, {
        props: { visible: true, library: null, submitting: false, error },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { teleport: true }
        }
    })

// The backend answers a bad library with a 422 that names the offending field in
// errors[] ({pointer, detail}); the dialog shows that message on the field and
// marks it invalid, rather than the parent only toasting the top-level sentence.
describe('LibraryDialog validation errors', () => {
    const problem = (pointer: string, detail: string) => ({
        response: { status: 422, data: { title: 'Unprocessable Entity', status: 422, detail, errors: [{ pointer, detail }] } }
    })

    it('surfaces a field error whose pointer maps to no field, so none is ever swallowed', async () => {
        const w = mountWithError(problem('/mystery', 'unknown field failed'))
        await flushPromises()
        expect(w.find('.form-error').exists()).toBe(true)
        expect(w.text()).toContain('unknown field failed')
    })

    it('shows no field error when the submit did not fail', async () => {
        const w = mountWithError(undefined)
        await flushPromises()
        expect(w.find('.field-error').exists()).toBe(false)
        expect(w.find('.form-error').exists()).toBe(false)
        expect(w.find('.p-invalid').exists()).toBe(false)
    })
})
