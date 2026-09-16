import { describe, it, expect, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

import LibraryDialog from '@/components/admin/LibraryDialog.vue'
import type { Library, LibraryInput } from '@/types/libraries'

const baseLibrary: Library = {
    id: 1,
    name: 'Main',
    path: '/srv/music',
    exclude_patterns: [],
    follow_symlinks: true,
    show_artists: true,
    default_view: 'albums',
    icon: 'folder',
    cover_style: 'bauhaus',
    source: 'db',
    last_scan_started_at: null,
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

describe('LibraryDialog cover style', () => {
    it('defaults cover_style to auto in create mode', async () => {
        const w = mountDialog(null)
        await flushPromises()
        const createBtn = w.findAll('button').find((b) => b.text().includes('Create'))!
        await createBtn.trigger('click')
        await flushPromises()
        const input = w.emitted('submit')![0][0] as LibraryInput
        expect(input.cover_style).toBe('auto')
    })

    it('submits the library cover_style unchanged in edit mode', async () => {
        const w = mountDialog(baseLibrary)
        await flushPromises()
        const saveBtn = w.findAll('button').find((b) => b.text().includes('Save'))!
        await saveBtn.trigger('click')
        await flushPromises()
        const input = w.emitted('submit')![0][0] as LibraryInput
        expect(input.cover_style).toBe('bauhaus')
    })

    it('offers auto plus all six styles in the dropdown', async () => {
        const w = mountDialog(null)
        await flushPromises()
        const selects = w.findAllComponents({ name: 'Select' })
        const styleSelect = selects.find((d) =>
            (d.props('options') as { value: string }[]).some((o) => o.value === 'auto')
        )!
        expect(styleSelect).toBeTruthy()
        const values = (styleSelect.props('options') as { value: string }[]).map((o) => o.value)
        expect(values).toEqual(['auto', 'classic', 'bauhaus', 'rings', 'waves', 'poster', 'remix'])
    })
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

    it('shows the server message on the field its pointer names and marks it invalid', async () => {
        const w = mountWithError(problem('/path', 'path is not a usable directory'))
        await flushPromises()
        expect(w.find('.field-error').exists()).toBe(true)
        expect(w.find('.field-error').text()).toContain('path is not a usable directory')
        expect(w.find('.p-invalid').exists()).toBe(true)
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
