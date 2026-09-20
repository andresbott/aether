import { describe, it, expect, vi } from 'vitest'
import { toRaw } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import InputText from 'primevue/inputtext'
import ToggleSwitch from 'primevue/toggleswitch'
import Dialog from 'primevue/dialog'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

import LibraryDialog from '@/components/admin/LibraryDialog.vue'
import type { Library, LibraryFilter, LibraryInput } from '@/types/libraries'

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

// LibraryFilterBuilder is its own fully tested component
// (LibraryFilterBuilder.spec.ts). Here it is stubbed to a minimal stand-in that
// exposes its modelValue/errors props and can emit update:modelValue, per the
// task brief, so this file tests only what the dialog does with it.
const FilterBuilderStub = {
    name: 'LibraryFilterBuilder',
    props: ['modelValue', 'errors'],
    emits: ['update:modelValue'],
    template: '<div class="filter-builder-stub" />'
}

const mountDialog = (library: Library | null) =>
    mount(LibraryDialog, {
        props: { visible: true, library, submitting: false },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { teleport: true, LibraryFilterBuilder: FilterBuilderStub }
        }
    })

const mountWithError = (error: unknown) =>
    mount(LibraryDialog, {
        props: { visible: true, library: null, submitting: false, error },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { teleport: true, LibraryFilterBuilder: FilterBuilderStub }
        }
    })

function findButton(w: ReturnType<typeof mountDialog>, label: string) {
    return w.findAll('button').find((b) => b.text().includes(label))!
}

function filterBuilder(w: ReturnType<typeof mountDialog>) {
    return w.findComponent(FilterBuilderStub)
}

describe('LibraryDialog create mode', () => {
    it('always sends filters, defaulting to [] alongside the other defaults', async () => {
        const w = mountDialog(null)
        await flushPromises()
        expect(filterBuilder(w).props('modelValue')).toEqual([])

        await findButton(w, 'Create').trigger('click')
        await flushPromises()

        const input = w.emitted('submit')![0][0] as LibraryInput
        expect(input).toEqual({
            name: '',
            show_artists: true,
            default_view: 'albums',
            icon: 'folder',
            filters: []
        })
    })

    it('trims the name but leaves filter values untouched', async () => {
        const w = mountDialog(null)
        await flushPromises()

        await w.find('#library-name').setValue('  Rock Albums  ')
        filterBuilder(w).vm.$emit('update:modelValue', [
            { field: 'genre', values: ['Rock '] }
        ] as LibraryFilter[])
        await flushPromises()

        await findButton(w, 'Create').trigger('click')
        await flushPromises()

        const input = w.emitted('submit')![0][0] as LibraryInput
        expect(input.name).toBe('Rock Albums')
        expect(input.filters).toEqual([{ field: 'genre', values: ['Rock '] }])
    })
})

describe('LibraryDialog edit mode', () => {
    it("loads the library's filters into the builder and emits them back unchanged when only the name is edited", async () => {
        const libWithFilters: Library = {
            ...baseLibrary,
            filters: [
                // 'Gone' models a stored value the server no longer offers (e.g. a
                // removed scan folder) — the builder is the one that shows it and
                // lets the admin remove it; the dialog must round-trip it either way.
                { field: 'scan_folder', values: ['Music', 'Gone'] },
                { field: 'genre', values: ['Rock'] }
            ]
        }
        const w = mountDialog(libWithFilters)
        await flushPromises()
        expect(filterBuilder(w).props('modelValue')).toEqual(libWithFilters.filters)

        await w.find('#library-name').setValue('Main Library')
        await findButton(w, 'Save').trigger('click')
        await flushPromises()

        const input = w.emitted('submit')![0][0] as LibraryInput
        expect(input.name).toBe('Main Library')
        expect(input.filters).toEqual(libWithFilters.filters)
    })

    // Global constraint: the form must never hold the vue-query cache's own
    // filter arrays, or editing the form (even abandoned, uncommitted edits)
    // could mutate objects other views are reading.
    it("copies the library's filters instead of aliasing the object handed in", async () => {
        const libWithFilters: Library = {
            ...baseLibrary,
            filters: [{ field: 'scan_folder', values: ['Music'] }]
        }
        const w = mountDialog(libWithFilters)
        await flushPromises()

        const modelValue = filterBuilder(w).props('modelValue') as LibraryFilter[]
        expect(modelValue).toEqual(libWithFilters.filters)
        // The form holds these in a ref, so what the builder receives is a
        // reactive PROXY of them: comparing the proxy to the library's own
        // object can never be equal, whether it aliases it or not. toRaw()
        // reaches the objects the form really holds — including the innermost
        // `values` array, the one an edit replaces — so aliasing fails here.
        expect(toRaw(modelValue)).not.toBe(libWithFilters.filters)
        expect(toRaw(modelValue[0])).not.toBe(libWithFilters.filters[0])
        expect(toRaw(modelValue[0].values)).not.toBe(libWithFilters.filters[0].values)
    })
})

// The backend answers a bad library with a 422 that names the offending field in
// errors[] ({pointer, detail}); the dialog shows that message on the field its
// pointer names, rather than the parent only toasting the top-level sentence.
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

    it('routes a 422 to where each pointer belongs: the builder, show artists, name, and the general list for the rest', async () => {
        const err = {
            response: {
                status: 422,
                data: {
                    title: 'Unprocessable Entity',
                    status: 422,
                    errors: [
                        { pointer: '/filters/0/values/0', detail: 'not a real genre' },
                        { pointer: '/show_artists', detail: 'needs at least one filter' },
                        { pointer: '/name', detail: 'name is required' },
                        { pointer: '/mystery', detail: 'unknown field failed' }
                    ]
                }
            }
        }
        const w = mountWithError(err)
        await flushPromises()

        // /filters… reaches the builder through its errors prop; nothing inline
        // in the dialog itself renders it.
        expect(filterBuilder(w).props('errors')).toMatchObject({
            '/filters/0/values/0': 'not a real genre'
        })

        // /name under the Name field, /show_artists under the toggle.
        expect(w.findComponent(InputText).props('invalid')).toBe(true)
        expect(w.findComponent(ToggleSwitch).props('invalid')).toBe(true)
        const fieldErrorTexts = w.findAll('.field-error').map((m) => m.text())
        expect(fieldErrorTexts).toEqual(['name is required', 'needs at least one filter'])

        // An unknown pointer is never swallowed: it lands in the general list —
        // and only it; every pointer shown inline is not duplicated there.
        const generalText = w.get('.form-error').text()
        expect(generalText).toContain('unknown field failed')
        expect(generalText).not.toContain('not a real genre')
        expect(generalText).not.toContain('name is required')
        expect(generalText).not.toContain('needs at least one filter')
    })
})

// CROSS-TASK ruling (phase3b-admin-ui progress.md, decided after Task 3's
// review): the server's /filters… pointers are POSITIONAL, indexing the
// filters as they were last sent. Once the admin edits the filters after a
// failed submit, a leftover /filters… error would attach to the wrong row, so
// the dialog hides it from the builder until the next failed submit supplies a
// fresh, correctly-positioned one.
describe('LibraryDialog stale filter errors', () => {
    const errorWith = (errors: { pointer: string; detail: string }[]) => ({
        response: { status: 422, data: { title: 'Unprocessable Entity', status: 422, errors } }
    })

    it('hides /filters… errors from the builder once the filters are edited, and restores them on the next failed submit', async () => {
        const firstError = errorWith([
            { pointer: '/filters/0/values/0', detail: 'not a real genre' },
            { pointer: '/name', detail: 'name is required' }
        ])
        const w = mountWithError(firstError)
        await flushPromises()

        const builder = filterBuilder(w)
        expect(builder.props('errors')['/filters/0/values/0']).toBe('not a real genre')
        expect(w.get('.field-error').text()).toBe('name is required')

        // The admin edits the filters (e.g. removes the offending row) — every
        // emit is an edit, since the builder is a controlled component that
        // never rewrites a value on its own.
        builder.vm.$emit('update:modelValue', [])
        await flushPromises()

        const errorsAfterEdit = builder.props('errors') as Record<string, string>
        expect(
            Object.keys(errorsAfterEdit).some((p) => p === '/filters' || p.startsWith('/filters/'))
        ).toBe(false)
        // A non-filter error is untouched by editing the filters.
        expect(w.get('.field-error').text()).toBe('name is required')

        // A NEW failed submit (a new `error` prop) brings the filter errors back.
        const secondError = errorWith([
            { pointer: '/filters/0/values/0', detail: 'still not a real genre' },
            { pointer: '/name', detail: 'name is required' }
        ])
        await w.setProps({ error: secondError })
        await flushPromises()

        expect(builder.props('errors')['/filters/0/values/0']).toBe('still not a real genre')
    })
})

describe('LibraryDialog chrome', () => {
    it('reads "Add Library" in create mode and is wide enough for the filter builder', async () => {
        const w = mountDialog(null)
        await flushPromises()
        expect(w.findComponent(Dialog).props('header')).toBe('Add Library')
        // Dialog's `style` isn't a declared prop — PrimeVue merges it onto the
        // rendered `.p-dialog` panel's own inline style, so assert it there.
        expect(w.get('.p-dialog').attributes('style')).toContain('width: min(92vw, 44rem)')
    })

    it('reads "Edit Library" in edit mode', async () => {
        const w = mountDialog(baseLibrary)
        await flushPromises()
        expect(w.findComponent(Dialog).props('header')).toBe('Edit Library')
    })

    it('shows the filters help text directly under the builder', async () => {
        const w = mountDialog(null)
        await flushPromises()
        expect(w.text()).toContain(
            'Filters narrow the library: every filter must match; inside one filter any value may.'
        )
        const html = w.html()
        expect(html.indexOf('filter-builder-stub')).toBeLessThan(
            html.indexOf('Filters narrow the library')
        )
    })
})
