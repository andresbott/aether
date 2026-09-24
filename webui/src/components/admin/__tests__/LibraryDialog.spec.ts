import { describe, it, expect, vi } from 'vitest'
import { nextTick, toRaw } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import InputText from 'primevue/inputtext'
import Checkbox from 'primevue/checkbox'
import Select from 'primevue/select'
import Dialog from 'primevue/dialog'
import Popover from 'primevue/popover'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

import LibraryDialog from '@/components/admin/LibraryDialog.vue'
import type { Library, LibraryFilter, LibraryInput } from '@/types/libraries'

const baseLibrary: Library = {
    id: 1,
    name: 'Main',
    views: ['discover', 'artists', 'albums'],
    default_view: 'discover',
    hide_from_artist_index: false,
    split_views: false,
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
    emits: ['update:modelValue', 'update:browsing'],
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

// The Escape harness: real IconSelect, real PrimeVue Dialog + Popover, attached
// to the document, and real <transition>s — the @enter hook is where PrimeVue
// binds the document-level Escape listener this case is about.
const mountEscapeDialog = () =>
    mount(LibraryDialog, {
        props: { visible: true, library: null, submitting: false },
        attachTo: document.body,
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { teleport: true, transition: false, LibraryFilterBuilder: FilterBuilderStub }
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

// The checkbox whose input carries `inputId` — one per view, plus the
// main-Artists-page one.
function checkbox(w: ReturnType<typeof mountDialog>, inputId: string) {
    return w.findAllComponents(Checkbox).find((c) => c.props('inputId') === inputId)!
}

async function toggleView(w: ReturnType<typeof mountDialog>, view: string) {
    await w.get(`#library-view-${view}`).trigger('change')
    await flushPromises()
}

async function submitted(w: ReturnType<typeof mountDialog>, label: string): Promise<LibraryInput> {
    await findButton(w, label).trigger('click')
    await flushPromises()
    return w.emitted('submit')![0][0] as LibraryInput
}

// PrimeVue's Popover keeps its open state in a private `visible` data property
// its public type does not expose, and in jsdom the leave transition never
// finishes, so the overlay element outlives the close. Its `show`/`hide` events
// are the public, observable signal.
function popoverEvents(w: ReturnType<typeof mountDialog>) {
    const p = w.findComponent(Popover)
    return { shown: p.emitted('show')?.length ?? 0, hidden: p.emitted('hide')?.length ?? 0 }
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
            views: ['discover', 'artists', 'albums'],
            default_view: 'discover',
            hide_from_artist_index: false,
            split_views: false,
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

describe('LibraryDialog views', () => {
    it('opens on the first view left when the one it opened on is unticked', async () => {
        const w = mountDialog(null)
        await flushPromises()

        await toggleView(w, 'discover')

        const input = await submitted(w, 'Create')
        expect(input.views).toEqual(['artists', 'albums'])
        expect(input.default_view).toBe('artists')
    })

    it('offers only the ticked views to open on, and sends them in display order', async () => {
        const w = mountDialog({ ...baseLibrary, views: ['albums'], default_view: 'albums' })
        await flushPromises()
        const opensOn = () => w.findComponent(Select)
        const offered = () => (opensOn().props('options') as { value: string }[]).map((o) => o.value)

        // One view leaves nothing to pick.
        expect(offered()).toEqual(['albums'])
        expect(opensOn().props('disabled')).toBe(true)

        // Ticked after Albums, but listed and sent before it.
        await toggleView(w, 'artists')
        expect(offered()).toEqual(['artists', 'albums'])
        expect(opensOn().props('disabled')).toBe(false)

        opensOn().vm.$emit('update:modelValue', 'artists')
        await flushPromises()

        const input = await submitted(w, 'Save')
        expect(input.views).toEqual(['artists', 'albums'])
        expect(input.default_view).toBe('artists')
    })

    it('keeps the last ticked view from being unticked', async () => {
        const w = mountDialog({ ...baseLibrary, views: ['artists'], default_view: 'artists' })
        await flushPromises()
        expect(checkbox(w, 'library-view-artists').props('disabled')).toBe(true)
        expect(checkbox(w, 'library-view-discover').props('disabled')).toBe(false)
    })

    // Hiding the artists from the main Artists page is its own setting: it
    // neither needs nor touches the Artists view.
    it("round-trips the main Artists page setting independently of the views", async () => {
        const w = mountDialog({
            ...baseLibrary,
            views: ['albums'],
            default_view: 'albums',
            hide_from_artist_index: true
        })
        await flushPromises()
        expect(checkbox(w, 'library-hide-artists').props('modelValue')).toBe(true)

        const input = await submitted(w, 'Save')
        expect(input.views).toEqual(['albums'])
        expect(input.hide_from_artist_index).toBe(true)
    })

    it('round-trips the sidebar layout', async () => {
        const w = mountDialog({ ...baseLibrary, split_views: true })
        await flushPromises()
        const radios = w.findAll<HTMLInputElement>('input[name="library-sidebar"]')
        expect(radios.map((r) => r.element.checked)).toEqual([false, true])

        await radios[0].setValue(true)
        await flushPromises()
        const input = await submitted(w, 'Save')
        expect(input.split_views).toBe(false)
    })

    it("copies the library's views instead of aliasing the array handed in", async () => {
        const lib: Library = { ...baseLibrary, views: ['artists', 'albums'], default_view: 'artists' }
        const w = mountDialog(lib)
        await flushPromises()

        await toggleView(w, 'albums')
        expect(lib.views).toEqual(['artists', 'albums'])
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

    it('routes a 422 to where each pointer belongs: the builder, the main Artists page, name, and the general list for the rest', async () => {
        const err = {
            response: {
                status: 422,
                data: {
                    title: 'Unprocessable Entity',
                    status: 422,
                    errors: [
                        { pointer: '/filters/0/values/0', detail: 'not a real genre' },
                        { pointer: '/hide_from_artist_index', detail: 'needs at least one filter' },
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

        // /name under the Name field, /hide_from_artist_index under its checkbox.
        expect(w.findComponent(InputText).props('invalid')).toBe(true)
        expect(checkbox(w, 'library-hide-artists').props('invalid')).toBe(true)
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

    it('shows every /views error on the views row and /default_view under Opens on', async () => {
        const err = {
            response: {
                status: 422,
                data: {
                    title: 'Unprocessable Entity',
                    status: 422,
                    errors: [
                        { pointer: '/views/1', detail: 'unknown view "songs"' },
                        { pointer: '/default_view', detail: 'not one of the views' }
                    ]
                }
            }
        }
        const w = mountWithError(err)
        await flushPromises()

        expect(checkbox(w, 'library-view-discover').props('invalid')).toBe(true)
        expect(w.findComponent(Select).props('invalid')).toBe(true)
        expect(w.findAll('.field-error').map((m) => m.text())).toEqual([
            'unknown view "songs"',
            'not one of the views'
        ])
        expect(w.find('.form-error').exists()).toBe(false)
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

    // PrimeVue binds one document-level Escape listener per visible Dialog and
    // none checks which one is on top, so while the builder's folder picker is
    // open this Dialog has to stop closing on Escape — see the builder's
    // update:browsing emit.
    it('ignores Escape while the builder reports the picker open and listens again after it closes', async () => {
        const w = mountDialog(null)
        await flushPromises()
        expect(w.findComponent(Dialog).props('closeOnEscape')).toBe(true)

        filterBuilder(w).vm.$emit('update:browsing', true)
        await flushPromises()
        expect(w.findComponent(Dialog).props('closeOnEscape')).toBe(false)

        filterBuilder(w).vm.$emit('update:browsing', false)
        await flushPromises()
        expect(w.findComponent(Dialog).props('closeOnEscape')).toBe(true)
    })

    // Same defect class, second host: PrimeVue's Popover hides on Escape
    // WITHOUT stopPropagation and binds its own document listener, so one
    // Escape in the icon search reaches this Dialog's document listener too.
    // The harness has to use real transitions and attach to the document: Vue
    // Test Utils' default <transition> stub never fires the @enter hook in
    // which PrimeVue binds that listener, so the default harness cannot see it.
    // Built in three explicit steps because ONE synthetic `dispatchEvent` can
    // never fail for this bug, and a real browser showed that it does fail: the
    // browser runs a microtask checkpoint after EACH listener of a trusted
    // event (the JS stack is empty between them), while a script's
    // `dispatchEvent` keeps the dispatching script on the stack throughout. In
    // between, Vue flushes and PrimeVue's Popover emits `hide` from the
    // <transition>'s @leave — at the START of the leave — so the host's
    // "picker open" flag would drop and hand `closeOnEscape` back to the Dialog
    // before the SAME Escape reaches its document listener. The three steps
    // replay exactly that ordering.
    it('keeps the dialog open when Escape closes the icon picker, and closes it on the next one', async () => {
        const w = mountEscapeDialog()
        await flushPromises()

        await w.get('.icon-select-trigger').trigger('click')
        await flushPromises()
        expect(popoverEvents(w)).toEqual({ shown: 1, hidden: 0 })

        const escape = () =>
            new KeyboardEvent('keydown', { code: 'Escape', key: 'Escape', bubbles: false })

        // 1. the popover's OWN element-level handler runs first (bubbles: false
        //    so nothing else sees this one).
        document.querySelector('.p-popover-content')!.dispatchEvent(escape())

        // 2. the microtask checkpoint a trusted dispatch performs here: Vue
        //    flushes and `hide` is emitted. nextTick, never flushPromises —
        //    flushPromises awaits a macrotask, which would let the fix's
        //    setTimeout(0) run and release the guard before step 3: the case
        //    would then go red although the fix is right.
        await nextTick()
        await nextTick()

        // 3. the same event arriving at the document-level listeners.
        document.dispatchEvent(escape())
        await nextTick()

        // Asserted first, so a failure names the defect rather than the picker.
        expect(w.emitted('update:visible')).toBeUndefined()
        expect(popoverEvents(w)).toEqual({ shown: 1, hidden: 1 })

        // and the NEXT Escape still closes the dialog: the fix must not leave it
        // deaf. Timers run first, so the guard has been released by then.
        await flushPromises()
        document.dispatchEvent(escape())
        await nextTick()
        expect(w.emitted('update:visible')![0]).toEqual([false])
        w.unmount()
    })

    // The builder unmounts with the dialog's content, so it cannot report
    // "closed" on the way out: a dialog re-opened after being closed with the
    // picker open must start listening to Escape again on its own.
    it('starts a re-opened dialog listening to Escape again', async () => {
        const w = mountDialog(null)
        await flushPromises()
        filterBuilder(w).vm.$emit('update:browsing', true)
        await flushPromises()
        expect(w.findComponent(Dialog).props('closeOnEscape')).toBe(false)

        await w.setProps({ visible: false })
        await flushPromises()
        await w.setProps({ visible: true })
        await flushPromises()
        expect(w.findComponent(Dialog).props('closeOnEscape')).toBe(true)
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
