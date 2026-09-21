import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import Select from 'primevue/select'
import MultiSelect from 'primevue/multiselect'
import type { Library, LibraryFilterOptions, LibraryInput } from '@/types/libraries'

// The seams every other admin spec cuts: the panel stubs the dialog, the dialog
// stubs the builder and the builder stubs every PrimeVue input — so a listener
// bound to a misspelled event (`@submit`, `@update:modelValue`, `@select`)
// passes all of them, and the project's tsconfig has no `strictTemplates` to
// catch it either. Here the three components, their composables and the real
// PrimeVue inputs are wired together for real; only the network and the two
// PrimeVue services that need an app-level provider are faked.
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))
vi.mock('primevue/useconfirm', () => ({ useConfirm: () => ({ require: vi.fn() }) }))

const api = vi.hoisted(() => ({
    listLibraries: vi.fn(),
    getLibrary: vi.fn(),
    createLibrary: vi.fn(),
    updateLibrary: vi.fn(),
    deleteLibrary: vi.fn(),
    previewLibrary: vi.fn(),
    getLibraryFilterOptions: vi.fn(),
    browseFolders: vi.fn()
}))
vi.mock('@/lib/api/Libraries', () => api)

const listScanFolders = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api/ScanFolders', () => ({ listScanFolders }))

import LibrariesPanel from '@/components/admin/LibrariesPanel.vue'
import LibraryDialog from '@/components/admin/LibraryDialog.vue'
import FolderPickerDialog from '@/components/admin/FolderPickerDialog.vue'

// --- Fixtures -----------------------------------------------------------------
// 'Gone' is a scan folder the server no longer offers, and 'Rock ' a genre whose
// trailing space is part of the value: both must survive the round trip byte for
// byte, since the server matches them against scanned data.
const stored: Library = {
    id: 7,
    name: 'Main',
    show_artists: true,
    default_view: 'albums',
    icon: 'folder',
    filters: [
        { field: 'scan_folder', values: ['Music', 'Gone'] },
        { field: 'genre', values: ['Rock '] }
    ],
    created_at: '',
    updated_at: '',
    track_count: 3
}

const filterOptions: LibraryFilterOptions = {
    scan_folders: ['Music'],
    formats: [],
    genres: ['Rock '],
    release_types: []
}

function mountAdmin(opts: { realTransitions?: boolean } = {}) {
    const queryClient = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } }
    })
    return mount(LibrariesPanel, {
        attachTo: opts.realTransitions ? document.body : undefined,
        global: {
            plugins: [PrimeVue, [VueQueryPlugin, { queryClient }]],
            directives: { tooltip: {} },
            // Vue Test Utils' default <transition> stub never fires the JS @enter
            // hook, and that hook is where PrimeVue's Dialog binds its
            // document-level Escape listener: without real transitions an Escape
            // case passes whether or not the bug exists.
            stubs: opts.realTransitions ? { teleport: true, transition: false } : { teleport: true }
        }
    })
}

type Wrapper = ReturnType<typeof mountAdmin>

function buttonWithText(w: Wrapper, label: string) {
    return w.findAll('button').find((b) => b.text().includes(label))!
}

function buttonWithIcon(w: Wrapper, icon: string) {
    return w.findAll('button').find((b) => b.find(`.pi-${icon}`).exists())!
}

function filterRow(w: Wrapper, idx: number) {
    return w.get(`[data-test="filter-row-${idx}"]`)
}

async function openEditDialog(w: Wrapper) {
    await buttonWithIcon(w, 'pencil').trigger('click')
    await flushPromises()
}

beforeEach(() => {
    for (const fn of Object.values(api)) fn.mockReset()
    listScanFolders.mockReset()

    api.listLibraries.mockResolvedValue([stored])
    api.getLibraryFilterOptions.mockResolvedValue(filterOptions)
    api.previewLibrary.mockResolvedValue({ track_count: 3, album_count: 1 })
    api.updateLibrary.mockResolvedValue({ ...stored, name: 'Renamed' })
    api.createLibrary.mockResolvedValue({ ...stored, id: 8, name: 'New' })
    listScanFolders.mockResolvedValue([])
})

describe('libraries admin, end to end through the real components', () => {
    it('previews the stored filters on open and sends them back untouched on save', async () => {
        const w = mountAdmin()
        await flushPromises()

        await openEditDialog(w)

        // The builder mounted with the library's own filters and previewed
        // exactly those — one call, not one per row and not an empty one first.
        expect(api.previewLibrary).toHaveBeenCalledTimes(1)
        expect(api.previewLibrary.mock.calls[0][0]).toEqual(stored.filters)
        // The scan folder the server stopped offering is still shown, labelled.
        expect(w.text()).toContain('Gone (not configured)')

        await w.get('#library-name').setValue('  Renamed  ')
        await buttonWithText(w, 'Save').trigger('click')
        await flushPromises()

        expect(api.updateLibrary).toHaveBeenCalledTimes(1)
        const [id, payload] = api.updateLibrary.mock.calls[0] as [number, LibraryInput]
        expect(id).toBe(stored.id)
        // Only the name is trimmed; the filters go back in order, verbatim,
        // including the value the server no longer offers.
        expect(payload.name).toBe('Renamed')
        expect(payload.filters).toEqual(stored.filters)
        expect(payload.filters[1].values).toEqual(['Rock '])
    })

    it("lands a failed save's messages on the row they name and shows the general one once", async () => {
        api.updateLibrary.mockRejectedValue({
            response: {
                status: 422,
                data: {
                    title: 'Unprocessable Entity',
                    status: 422,
                    errors: [
                        {
                            pointer: '/filters/0/values/1',
                            detail: 'scan folder "Gone" is not configured'
                        },
                        { pointer: '/filters', detail: 'too many' }
                    ]
                }
            }
        })

        const w = mountAdmin()
        await flushPromises()
        await openEditDialog(w)

        await buttonWithText(w, 'Save').trigger('click')
        await flushPromises()

        // /filters/0/values/1 names the second value of the first row.
        expect(filterRow(w, 0).text()).toContain('Gone: scan folder "Gone" is not configured')
        expect(filterRow(w, 1).text()).not.toContain('is not configured')

        const general = w.findAll('[data-test="general-errors"]')
        expect(general).toHaveLength(1)
        expect(general[0].text()).toBe('too many')
    })

    it('creates a library with the rows the admin built, in the order they were added', async () => {
        const w = mountAdmin()
        await flushPromises()

        await buttonWithText(w, 'Add library').trigger('click')
        await flushPromises()

        await buttonWithText(w, 'Add filter').trigger('click')
        await flushPromises()
        await buttonWithText(w, 'Add filter').trigger('click')
        await flushPromises()

        // Row 0 keeps the default scan_folder field and picks a folder.
        filterRow(w, 0).findComponent(MultiSelect).vm.$emit('update:modelValue', ['Music'])
        await flushPromises()

        // Row 1 switches field first (which clears its values), then picks.
        filterRow(w, 1).findComponent(Select).vm.$emit('update:modelValue', 'genre')
        await flushPromises()
        filterRow(w, 1).findComponent(MultiSelect).vm.$emit('update:modelValue', ['Rock '])
        await flushPromises()

        await w.get('#library-name').setValue('New')
        await buttonWithText(w, 'Create').trigger('click')
        await flushPromises()

        expect(api.createLibrary).toHaveBeenCalledTimes(1)
        const input = api.createLibrary.mock.calls[0][0] as LibraryInput
        expect(input.name).toBe('New')
        expect(input.filters).toEqual([
            { field: 'scan_folder', values: ['Music'] },
            { field: 'genre', values: ['Rock '] }
        ])
    })

    it('Escape inside the folder picker closes the picker and leaves the library dialog open', async () => {
        api.browseFolders.mockResolvedValue([
            { name: 'Music', path: '/srv/music', has_subfolders: true, is_symlink: false }
        ])
        const w = mountAdmin({ realTransitions: true })
        await flushPromises()
        await buttonWithText(w, 'Add library').trigger('click')
        await flushPromises()
        await buttonWithText(w, 'Add filter').trigger('click')
        filterRow(w, 0).findComponent(Select).vm.$emit('update:modelValue', 'path')
        await flushPromises()
        await w.get('[data-test="browse-path"]').trigger('click')
        await flushPromises()
        expect(w.findComponent(FolderPickerDialog).props('visible')).toBe(true)

        document.dispatchEvent(new KeyboardEvent('keydown', { code: 'Escape', key: 'Escape', bubbles: true }))
        await flushPromises()

        // The library dialog first: when the bug is back BOTH dialogs close, the
        // dialog's content unmounts and the picker can no longer be found at
        // all — asserted in this order, the failure names the defect instead of
        // "Cannot call props on an empty VueWrapper".
        expect(w.findComponent(LibraryDialog).props('visible')).toBe(true)
        expect(w.findComponent(FolderPickerDialog).props('visible')).toBe(false)

        // and the NEXT Escape still closes the library dialog: the fix must not leave it deaf
        document.dispatchEvent(new KeyboardEvent('keydown', { code: 'Escape', key: 'Escape', bubbles: true }))
        await flushPromises()
        expect(w.findComponent(LibraryDialog).props('visible')).toBe(false)
        w.unmount()
    })
})
