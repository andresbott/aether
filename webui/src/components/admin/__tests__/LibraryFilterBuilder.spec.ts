import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import PrimeVue from 'primevue/config'
import type { LibraryFilter, LibraryFilterField, LibraryFilterOptions } from '@/types/libraries'
import type { ScanFolder } from '@/types/scanFolders'

// --- Mocks: network layer + composables, per the task brief -------------------

const previewSpy = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api/Libraries', () => ({ previewLibrary: previewSpy }))

const filterOptionsRef = vi.hoisted(() => ({ value: undefined as LibraryFilterOptions | undefined }))
const filterOptionsErrorRef = vi.hoisted(() => ({ value: false }))
vi.mock('@/composables/useLibraries', () => ({
    useLibraryFilterOptions: () => ({ data: filterOptionsRef, isError: filterOptionsErrorRef })
}))

const scanFoldersRef = vi.hoisted(() => ({ value: undefined as ScanFolder[] | undefined }))
vi.mock('@/composables/useScanFolders', () => ({
    useScanFolders: () => ({ data: scanFoldersRef })
}))

import LibraryFilterBuilder from '@/components/admin/LibraryFilterBuilder.vue'
import { FILTER_FIELDS, LIMITS, optionsFor } from '@/lib/libraryFilters'

// --- Fixtures -------------------------------------------------------------------

const defaultOptions: LibraryFilterOptions = {
    scan_folders: ['Music', 'Podcasts'],
    formats: ['flac', 'mp3'],
    genres: ['Rock', 'Jazz'],
    release_types: ['Album', 'EP']
}

const defaultScanFolders: ScanFolder[] = [
    {
        name: 'Music',
        path: '/music',
        exclude_patterns: [],
        follow_symlinks: false,
        available: true,
        track_count: 10
    },
    {
        name: 'Podcasts',
        path: '/podcasts',
        exclude_patterns: [],
        follow_symlinks: false,
        available: true,
        track_count: 5
    }
]

function mkRow(field: LibraryFilterField, values: string[] = []): LibraryFilter {
    return { field, values }
}

function rowsOf(n: number): LibraryFilter[] {
    return Array.from({ length: n }, () => mkRow('genre'))
}

function valuesOf(n: number): string[] {
    return Array.from({ length: n }, (_, i) => `v${i}`)
}

// --- Stubs ------------------------------------------------------------------
// Minimal components exposing modelValue/options and emitting update:modelValue,
// as MetadataEditorView.scanFolderList.spec.ts stubs Listbox. Each renders its
// options as clickable, data-test-addressable rows rather than reproducing
// PrimeVue's real overlay/listbox internals.

const stubs = {
    Select: {
        name: 'Select',
        props: ['modelValue', 'options', 'optionLabel', 'optionValue'],
        emits: ['update:modelValue'],
        template:
            '<div class="select-stub">' +
            '<button type="button" v-for="o in options" :key="o[optionValue]" ' +
            ':data-test="`select-opt-${o[optionValue]}`" ' +
            '@click="$emit(\'update:modelValue\', o[optionValue])">{{ o[optionLabel] }}</button>' +
            '</div>'
    },
    MultiSelect: {
        name: 'MultiSelect',
        props: [
            'modelValue',
            'options',
            'optionLabel',
            'optionValue',
            'display',
            'filter',
            'showToggleAll',
            'invalid'
        ],
        emits: ['update:modelValue'],
        template:
            '<div class="multiselect-stub">' +
            '<div class="ms-option" v-for="o in options" :key="o[optionValue]" ' +
            ':data-test="`ms-opt-${o[optionValue]}`" ' +
            '@click="$emit(\'update:modelValue\', [...(modelValue || []), o[optionValue]])">' +
            '<slot name="option" :option="o">{{ o[optionLabel] }}</slot>' +
            '</div>' +
            '</div>'
    },
    AutoComplete: {
        name: 'AutoComplete',
        // `multiple` is declared as Boolean (not just named) so a bare
        // `multiple` attribute — the real AutoComplete's own free-entry-chips
        // pattern (see GenreChips.vue) — casts to `true` rather than the
        // empty-string Vue gives an undeclared-type prop for a valueless
        // attribute.
        props: {
            modelValue: { type: Array },
            multiple: { type: Boolean },
            typeahead: { type: Boolean },
            placeholder: { type: String },
            invalid: { type: Boolean }
        },
        emits: ['update:modelValue'],
        template:
            '<input class="autocomplete-stub" :placeholder="placeholder" ' +
            '@keyup.enter="$emit(\'update:modelValue\', [...(modelValue || []), $event.target.value]); $event.target.value = \'\'" />'
    },
    SelectButton: {
        name: 'SelectButton',
        props: ['modelValue', 'options', 'optionLabel', 'optionValue', 'invalid'],
        emits: ['update:modelValue'],
        template:
            '<div class="selectbutton-stub">' +
            '<button type="button" v-for="o in options" :key="o[optionValue]" ' +
            ':data-test="`sb-opt-${o[optionValue]}`" ' +
            '@click="$emit(\'update:modelValue\', modelValue === o[optionValue] ? null : o[optionValue])">{{ o[optionLabel] }}</button>' +
            '</div>'
    },
    Button: {
        name: 'Button',
        props: ['label', 'disabled'],
        inheritAttrs: false,
        template:
            '<button :disabled="disabled" :aria-label="$attrs[\'aria-label\']" :data-test="$attrs[\'data-test\']" @click="$emit(\'click\')">{{ label }}</button>'
    },
    Message: {
        name: 'Message',
        props: ['severity', 'closable'],
        template: '<div class="message-stub"><slot /></div>'
    },
    FolderPickerDialog: {
        name: 'FolderPickerDialog',
        props: ['visible'],
        emits: ['update:visible', 'select'],
        template: '<div v-if="visible" data-test="folder-picker-dialog" />'
    }
}

// --- Mount + query helpers ----------------------------------------------------

function mountBuilder(modelValue: LibraryFilter[], errors: Record<string, string> = {}) {
    return mount(LibraryFilterBuilder, {
        props: { modelValue, errors },
        global: { stubs }
    })
}

// The paths control's real PrimeVue component, for the one case that is about
// what PrimeVue itself does with typed text; everything else keeps the stub.
const { AutoComplete: _autoCompleteStub, ...stubsWithRealAutoComplete } = stubs

function mountBuilderWithRealPathsControl(modelValue: LibraryFilter[]) {
    return mount(LibraryFilterBuilder, {
        props: { modelValue, errors: {} },
        global: { plugins: [PrimeVue], stubs: stubsWithRealAutoComplete }
    })
}

function row(w: ReturnType<typeof mountBuilder>, idx: number) {
    return w.get(`[data-test="filter-row-${idx}"]`)
}

function fieldSelectIn(w: ReturnType<typeof mountBuilder>, idx: number) {
    return row(w, idx).findComponent({ name: 'Select' })
}

function multiSelectIn(w: ReturnType<typeof mountBuilder>, idx: number) {
    return row(w, idx).findComponent({ name: 'MultiSelect' })
}

function autoCompleteIn(w: ReturnType<typeof mountBuilder>, idx: number) {
    return row(w, idx).findComponent({ name: 'AutoComplete' })
}

function selectButtonIn(w: ReturnType<typeof mountBuilder>, idx: number) {
    return row(w, idx).findComponent({ name: 'SelectButton' })
}

function lastEmittedFilters(w: ReturnType<typeof mountBuilder>): LibraryFilter[] {
    const emitted = w.emitted('update:modelValue')!
    return emitted[emitted.length - 1][0] as LibraryFilter[]
}

beforeEach(() => {
    previewSpy.mockReset()
    previewSpy.mockResolvedValue({ track_count: 0, album_count: 0 })
    filterOptionsRef.value = defaultOptions
    filterOptionsErrorRef.value = false
    scanFoldersRef.value = defaultScanFolders
})

describe('LibraryFilterBuilder', () => {
    // ---- Behaviour 1: empty state + adding a filter --------------------------
    describe('behaviour 1: empty state and adding a filter', () => {
        it('shows the empty-catalog hint and an Add filter button with no filters', () => {
            const w = mountBuilder([])
            expect(w.get('[data-test="empty-hint"]').text()).toBe(
                'No filters: this library shows the whole catalog.'
            )
            expect(w.find('[data-test="add-filter"]').exists()).toBe(true)
        })

        it('hides the empty-catalog hint once there is at least one filter', () => {
            const w = mountBuilder([mkRow('scan_folder')])
            expect(w.find('[data-test="empty-hint"]').exists()).toBe(false)
        })

        it('appends a blank scan_folder filter and emits when Add filter is clicked', async () => {
            const w = mountBuilder([])
            await w.get('[data-test="add-filter"]').trigger('click')
            expect(w.emitted('update:modelValue')![0][0]).toEqual([{ field: 'scan_folder', values: [] }])
        })
    })

    // ---- Behaviour 2: row shape ------------------------------------------------
    describe('behaviour 2: row shape (field select, value control, remove)', () => {
        it('renders one row per filter with a field Select offering FILTER_FIELDS', () => {
            const w = mountBuilder([mkRow('scan_folder'), mkRow('genre')])
            expect(w.findAll('[data-test^="filter-row-"]')).toHaveLength(2)
            const select = fieldSelectIn(w, 0)
            expect(select.props('options')).toEqual(FILTER_FIELDS)
            expect(select.props('optionLabel')).toBe('label')
            expect(select.props('optionValue')).toBe('field')
        })

        it('gives the field select and the remove button accessible names', () => {
            const w = mountBuilder([mkRow('scan_folder')])
            expect(row(w, 0).find('[aria-label="Filter field"]').exists()).toBe(true)
            expect(row(w, 0).find('[aria-label="Remove filter"]').exists()).toBe(true)
        })

        it("resets that row's values to [] and emits when the field changes", async () => {
            const w = mountBuilder([mkRow('scan_folder', ['Music'])])
            await row(w, 0).get('[data-test="select-opt-genre"]').trigger('click')
            expect(w.emitted('update:modelValue')![0][0]).toEqual([{ field: 'genre', values: [] }])
        })

        it('emits the array without that row when Remove is clicked', async () => {
            const w = mountBuilder([mkRow('scan_folder', ['Music']), mkRow('genre', ['Rock'])])
            await row(w, 1).get('[data-test="remove-filter"]').trigger('click')
            expect(w.emitted('update:modelValue')![0][0]).toEqual([
                { field: 'scan_folder', values: ['Music'] }
            ])
        })

        it('allows the same field to be used by several rows', async () => {
            const w = mountBuilder([mkRow('genre', ['Rock']), mkRow('genre', ['Jazz'])])
            expect(multiSelectIn(w, 0).props('modelValue')).toEqual(['Rock'])
            expect(multiSelectIn(w, 1).props('modelValue')).toEqual(['Jazz'])
            // Changing one row's field is unaffected by another row sharing it.
            await row(w, 0).get('[data-test="select-opt-format"]').trigger('click')
            expect(w.emitted('update:modelValue')![0][0]).toEqual([
                { field: 'format', values: [] },
                { field: 'genre', values: ['Jazz'] }
            ])
        })
    })

    // ---- Behaviour 3: value control by control type ----------------------------
    describe('behaviour 3: value control by field control type', () => {
        it('enables the MultiSelect filter only for genre', () => {
            const w = mountBuilder([mkRow('genre'), mkRow('format'), mkRow('scan_folder')])
            expect(multiSelectIn(w, 0).props('filter')).toBe(true)
            expect(multiSelectIn(w, 1).props('filter')).toBe(false)
            expect(multiSelectIn(w, 2).props('filter')).toBe(false)
        })

        it('uses chip display, optionLabel/optionValue and hides the toggle-all checkbox', () => {
            const w = mountBuilder([mkRow('format')])
            const ms = multiSelectIn(w, 0)
            expect(ms.props('display')).toBe('chip')
            expect(ms.props('optionLabel')).toBe('label')
            expect(ms.props('optionValue')).toBe('value')
            expect(ms.props('showToggleAll')).toBe(false)
        })

        it('offers exactly what optionsFor computes for the row', () => {
            const w = mountBuilder([mkRow('format', ['flac'])])
            expect(multiSelectIn(w, 0).props('options')).toEqual(
                optionsFor('format', ['flac'], defaultOptions)
            )
        })

        it('renders a value the server no longer offers with the option-missing class', () => {
            const w = mountBuilder([mkRow('scan_folder', ['Music', 'Gone'])])
            const ms = multiSelectIn(w, 0)
            expect(ms.find('[data-test="ms-opt-Music"] .option-missing').exists()).toBe(false)
            expect(ms.find('[data-test="ms-opt-Gone"] .option-missing').exists()).toBe(true)
        })

        it('uses an AutoComplete (multiple, no typeahead) for a paths field', () => {
            const w = mountBuilder([mkRow('path', ['/a'])])
            const ac = autoCompleteIn(w, 0)
            expect(ac.exists()).toBe(true)
            expect(ac.props('multiple')).toBe(true)
            expect(ac.props('typeahead')).toBe(false)
        })

        it('shows a Browse… button next to the paths AutoComplete', () => {
            const w = mountBuilder([mkRow('path')])
            expect(row(w, 0).get('[data-test="browse-path"]').text()).toBe('Browse…')
        })

        it('opens FolderPickerDialog from Browse… and appends the picked path if not already present', async () => {
            const w = mountBuilder([mkRow('path', ['/music/existing'])])
            expect(w.find('[data-test="folder-picker-dialog"]').exists()).toBe(false)

            await row(w, 0).get('[data-test="browse-path"]').trigger('click')
            const dialog = w.findComponent({ name: 'FolderPickerDialog' })
            expect(dialog.props('visible')).toBe(true)

            dialog.vm.$emit('select', '/music/new')
            expect(lastEmittedFilters(w)[0].values).toEqual(['/music/existing', '/music/new'])
        })

        it('does not duplicate a path already present when it is picked again', async () => {
            const w = mountBuilder([mkRow('path', ['/music/existing'])])
            await row(w, 0).get('[data-test="browse-path"]').trigger('click')
            const dialog = w.findComponent({ name: 'FolderPickerDialog' })
            dialog.vm.$emit('select', '/music/existing')
            expect(w.emitted('update:modelValue')).toBeUndefined()
        })

        it("uses a SelectButton mapping Yes/No to ['true']/['false'] for compilation", () => {
            const w = mountBuilder([mkRow('compilation', ['true'])])
            const sb = selectButtonIn(w, 0)
            expect(sb.exists()).toBe(true)
            expect(sb.props('modelValue')).toBe('true')
            expect(sb.props('options')).toEqual([
                { label: 'Yes', value: 'true' },
                { label: 'No', value: 'false' }
            ])
        })

        it("emits ['false'] when No is picked", () => {
            const w = mountBuilder([mkRow('compilation', ['true'])])
            selectButtonIn(w, 0).vm.$emit('update:modelValue', 'false')
            expect(lastEmittedFilters(w)[0].values).toEqual(['false'])
        })

        it('emits [] when the yes/no selection is cleared', () => {
            const w = mountBuilder([mkRow('compilation', ['true'])])
            selectButtonIn(w, 0).vm.$emit('update:modelValue', null)
            expect(lastEmittedFilters(w)[0].values).toEqual([])
        })
    })

    // ---- Behaviour 4: the app itself never rewrites a value ---------------------
    describe('behaviour 4: the app never rewrites a value, though typed chips are trimmed', () => {
        it('emits a selected value with a trailing space exactly as given', async () => {
            filterOptionsRef.value = { ...defaultOptions, genres: ['Rock '] }
            const w = mountBuilder([mkRow('genre')])
            await multiSelectIn(w, 0).get('.ms-option').trigger('click')
            expect(lastEmittedFilters(w)[0].values).toEqual(['Rock '])
        })

        // This one mounts the REAL AutoComplete, because what it pins is
        // PrimeVue's own behaviour: its chips input commits
        // event.target.value.trim() on Enter, so a TYPED path is trimmed and
        // only "Browse…" can add one verbatim. If a PrimeVue upgrade changes
        // that, this case fails and docs/agents/frontend.md must follow.
        it('commits a typed path trimmed, because PrimeVue trims what is typed into a chips input', async () => {
            const w = mountBuilderWithRealPathsControl([mkRow('path')])
            const input = row(w, 0).get('.p-autocomplete input')
            await input.setValue('  /srv/music/x  ')
            await input.trigger('keydown', { code: 'Enter', key: 'Enter' })
            expect(lastEmittedFilters(w)[0].values).toEqual(['/srv/music/x'])
        })
    })

    // ---- Behaviour 5: unavailable scan folder note ------------------------------
    describe('behaviour 5: unavailable scan folder note', () => {
        it('shows the not-usable note for a scan_folder value the server reports unavailable', () => {
            scanFoldersRef.value = [
                {
                    name: 'Music',
                    path: '/music',
                    exclude_patterns: [],
                    follow_symlinks: false,
                    available: false,
                    problem: 'root "/music" is unavailable',
                    track_count: 0
                }
            ]
            const w = mountBuilder([mkRow('scan_folder', ['Music'])])
            expect(row(w, 0).get('[data-test="folder-unavailable-note"]').text()).toBe(
                'Music is not usable right now — scans and re-indexing refuse it: root "/music" is unavailable'
            )
        })

        it('shows no note for an available scan_folder value', () => {
            const w = mountBuilder([mkRow('scan_folder', ['Music'])])
            expect(row(w, 0).find('[data-test="folder-unavailable-note"]').exists()).toBe(false)
        })
    })

    // ---- Behaviour 6: server errors matched by row index ------------------------
    describe('behaviour 6: server errors matched by row index', () => {
        const modelValue: LibraryFilter[] = [
            mkRow('scan_folder', ['Music']),
            mkRow('genre', ['Rock', 'Jazz'])
        ]
        const errors = {
            '/filters/0/field': 'unknown field',
            '/filters/1/values': 'too many values',
            '/filters/1/values/0': 'not a real genre',
            '/filters': 'at least one filter is required'
        }

        it('shows a field error on the row it belongs to, and not on others', () => {
            const w = mountBuilder(modelValue, errors)
            expect(row(w, 0).get('[data-test="row-field-error"]').text()).toBe('unknown field')
            expect(row(w, 1).find('[data-test="row-field-error"]').exists()).toBe(false)
        })

        it('shows the values error and a per-value error formatted "<label>: <detail>"', () => {
            const w = mountBuilder(modelValue, errors)
            expect(row(w, 1).get('[data-test="row-values-error"]').text()).toBe('too many values')
            expect(row(w, 1).get('[data-test="row-value-error"]').text()).toBe('Rock: not a real genre')
            expect(row(w, 0).find('[data-test="row-values-error"]').exists()).toBe(false)
        })

        it('shows general errors above the rows', () => {
            const w = mountBuilder(modelValue, errors)
            expect(w.get('[data-test="general-errors"]').text()).toContain(
                'at least one filter is required'
            )
            const html = w.html()
            expect(html.indexOf('general-errors')).toBeLessThan(html.indexOf('filter-row-0'))
        })

        it('shows no errors at all when the errors map is empty', () => {
            const w = mountBuilder(modelValue, {})
            expect(w.find('[data-test="general-errors"]').exists()).toBe(false)
            expect(w.find('[data-test="row-field-error"]').exists()).toBe(false)
            expect(w.find('[data-test="row-values-error"]').exists()).toBe(false)
            expect(w.find('[data-test="row-value-error"]').exists()).toBe(false)
        })

        // The control an admin has to fix must look wrong, whether the server
        // complained about the value list or about one value inside it.
        it('marks the value control invalid when the only error names one of its values', () => {
            const w = mountBuilder([mkRow('genre', ['Rock'])], {
                '/filters/0/values/0': 'not a real genre'
            })
            expect(multiSelectIn(w, 0).props('invalid')).toBe(true)
        })

        it('marks the paths and yes/no controls invalid as well, not only the MultiSelect', () => {
            const wPaths = mountBuilder([mkRow('path', ['/a'])], {
                '/filters/0/values/0': 'no such folder'
            })
            expect(autoCompleteIn(wPaths, 0).props('invalid')).toBe(true)

            const wYesNo = mountBuilder([mkRow('compilation', ['true'])], {
                '/filters/0/values': 'pick one'
            })
            expect(selectButtonIn(wYesNo, 0).props('invalid')).toBe(true)
        })

        it('leaves a row the server did not complain about valid', () => {
            const w = mountBuilder([mkRow('genre', ['Rock']), mkRow('format', ['flac'])], {
                '/filters/0/values': 'too many values'
            })
            expect(multiSelectIn(w, 0).props('invalid')).toBe(true)
            expect(multiSelectIn(w, 1).props('invalid')).toBe(false)
        })
    })

    // ---- Behaviour 7: limits ------------------------------------------------------
    describe('behaviour 7: limits (advisory; the server still has the last word)', () => {
        it('disables Add filter at the row limit and not just below it', () => {
            const wUnder = mountBuilder(rowsOf(LIMITS.filters - 1))
            expect(wUnder.get('[data-test="add-filter"]').attributes('disabled')).toBeUndefined()

            const wAt = mountBuilder(rowsOf(LIMITS.filters))
            expect(wAt.get('[data-test="add-filter"]').attributes('disabled')).toBeDefined()
        })

        it('shows the per-filter limit note at the value limit and not just below it', () => {
            const wUnder = mountBuilder([mkRow('genre', valuesOf(LIMITS.valuesPerFilter - 1))])
            expect(wUnder.find('[data-test="row-limit-note"]').exists()).toBe(false)

            const wAt = mountBuilder([mkRow('genre', valuesOf(LIMITS.valuesPerFilter))])
            expect(wAt.get('[data-test="row-limit-note"]').text()).toBe('At most 100 values per filter.')
        })

        it('shows the total limit note only once the total exceeds it', () => {
            // Three rows well under the per-row cap, so only the TOTAL note is
            // under test here.
            const over = Math.floor(LIMITS.valuesTotal / 3) + 1
            const wOver = mountBuilder([
                mkRow('genre', valuesOf(over)),
                mkRow('format', valuesOf(over)),
                mkRow('release_type', valuesOf(over))
            ])
            expect(wOver.get('[data-test="total-limit-note"]').text()).toBe(
                'At most 200 values across all filters.'
            )

            const atLimitEach = Math.floor(LIMITS.valuesTotal / 4)
            const wAtLimit = mountBuilder([
                mkRow('genre', valuesOf(atLimitEach)),
                mkRow('format', valuesOf(atLimitEach)),
                mkRow('release_type', valuesOf(atLimitEach)),
                mkRow('scan_folder', valuesOf(LIMITS.valuesTotal - 3 * atLimitEach))
            ])
            expect(wAtLimit.find('[data-test="total-limit-note"]').exists()).toBe(false)
        })
    })

    // ---- Behaviour 8: live preview -------------------------------------------------
    describe('behaviour 8: live preview', () => {
        beforeEach(() => vi.useFakeTimers())
        afterEach(() => vi.useRealTimers())

        it('runs a preview once on mount with the current filters', () => {
            const modelValue = [mkRow('format', ['flac'])]
            mountBuilder(modelValue)
            expect(previewSpy).toHaveBeenCalledTimes(1)
            expect(previewSpy).toHaveBeenCalledWith(modelValue, expect.any(AbortSignal))
        })

        it('renders "Matches N tracks in M albums." on success', async () => {
            previewSpy.mockResolvedValueOnce({ track_count: 5, album_count: 2 })
            const w = mountBuilder([mkRow('format', ['flac'])])
            await vi.advanceTimersByTimeAsync(0)
            expect(w.get('[data-test="filter-preview"]').text()).toBe('Matches 5 tracks in 2 albums.')
        })

        it('debounces a preview 400ms after the last change, clearing any pending one', async () => {
            const w = mountBuilder([mkRow('genre', [])])
            await vi.advanceTimersByTimeAsync(0)
            expect(previewSpy).toHaveBeenCalledTimes(1)

            await w.setProps({ modelValue: [mkRow('genre', ['Rock'])] })
            await vi.advanceTimersByTimeAsync(200)
            expect(previewSpy).toHaveBeenCalledTimes(1)

            // A second change before the first's debounce elapsed restarts the clock.
            await w.setProps({ modelValue: [mkRow('genre', ['Rock', 'Jazz'])] })
            await vi.advanceTimersByTimeAsync(200)
            expect(previewSpy).toHaveBeenCalledTimes(1)

            await vi.advanceTimersByTimeAsync(200)
            expect(previewSpy).toHaveBeenCalledTimes(2)
            expect(previewSpy).toHaveBeenLastCalledWith(
                [mkRow('genre', ['Rock', 'Jazz'])],
                expect.any(AbortSignal)
            )
        })

        it('aborts an in-flight preview when a newer one starts and ignores its late response', async () => {
            let firstSignal: AbortSignal | undefined
            let resolveFirst!: (v: { track_count: number; album_count: number }) => void
            previewSpy.mockImplementationOnce((_filters: LibraryFilter[], signal: AbortSignal) => {
                firstSignal = signal
                return new Promise((resolve) => {
                    resolveFirst = resolve
                })
            })
            const w = mountBuilder([mkRow('genre', ['Rock'])])
            await vi.advanceTimersByTimeAsync(0)
            expect(previewSpy).toHaveBeenCalledTimes(1)
            expect(firstSignal?.aborted).toBe(false)

            let secondSignal: AbortSignal | undefined
            previewSpy.mockImplementationOnce((_filters: LibraryFilter[], signal: AbortSignal) => {
                secondSignal = signal
                return new Promise(() => {})
            })
            await w.setProps({ modelValue: [mkRow('genre', ['Rock', 'Jazz'])] })
            await vi.advanceTimersByTimeAsync(400)
            expect(previewSpy).toHaveBeenCalledTimes(2)
            expect(firstSignal?.aborted).toBe(true)
            expect(secondSignal?.aborted).toBe(false)

            // The stale first response, arriving late, must not overwrite the display.
            resolveFirst({ track_count: 999, album_count: 999 })
            await vi.advanceTimersByTimeAsync(0)
            expect(w.find('[data-test="filter-preview"]').exists()).toBe(false)
        })

        it('renders "Fix the filters to see what they match." on a 422', async () => {
            previewSpy.mockRejectedValueOnce({ response: { status: 422 } })
            const w = mountBuilder([mkRow('genre', [])])
            await vi.advanceTimersByTimeAsync(0)
            expect(w.get('[data-test="filter-preview"]').text()).toBe(
                'Fix the filters to see what they match.'
            )
        })

        it('renders nothing on any other error, clearing a previous result', async () => {
            previewSpy.mockResolvedValueOnce({ track_count: 3, album_count: 1 })
            const w = mountBuilder([mkRow('genre', ['Rock'])])
            await vi.advanceTimersByTimeAsync(0)
            expect(w.find('[data-test="filter-preview"]').exists()).toBe(true)

            previewSpy.mockRejectedValueOnce(new Error('network down'))
            await w.setProps({ modelValue: [mkRow('genre', ['Jazz'])] })
            await vi.advanceTimersByTimeAsync(400)
            expect(w.find('[data-test="filter-preview"]').exists()).toBe(false)
        })

        it('sends a row with no values in the request as-is (not filtered out)', () => {
            const modelValue = [mkRow('genre', [])]
            mountBuilder(modelValue)
            expect(previewSpy).toHaveBeenCalledWith(modelValue, expect.any(AbortSignal))
        })

        it('clears the pending debounce timer on unmount so it never fires', async () => {
            const w = mountBuilder([mkRow('genre', [])])
            await vi.advanceTimersByTimeAsync(0)
            expect(previewSpy).toHaveBeenCalledTimes(1)

            await w.setProps({ modelValue: [mkRow('genre', ['Rock'])] })
            w.unmount()
            await vi.advanceTimersByTimeAsync(400)
            expect(previewSpy).toHaveBeenCalledTimes(1)
        })
    })

    // ---- Behaviour 9: the pick-lists are loading, or failed to load ---------------
    // "Not loaded" is not "not offered": every open of the dialog on a fresh
    // page load renders once before the options arrive, and a failed
    // GET /libraries/filter-options never delivers them at all.
    describe('behaviour 9: the value pick-lists are still loading or failed to load', () => {
        it('shows the stored values plainly while the options have not loaded', () => {
            filterOptionsRef.value = undefined
            const w = mountBuilder([mkRow('scan_folder', ['Music']), mkRow('genre', ['Rock'])])
            expect(multiSelectIn(w, 0).props('options')).toEqual([
                { label: 'Music', value: 'Music', missing: false }
            ])
            expect(w.text()).not.toContain('not configured')
            expect(w.text()).not.toContain('not in the catalog')
            expect(w.find('.option-missing').exists()).toBe(false)
        })

        it('renders one line saying the values could not be loaded when the query failed', () => {
            filterOptionsRef.value = undefined
            filterOptionsErrorRef.value = true
            const w = mountBuilder([mkRow('scan_folder', ['Music'])])
            const line = w.get('[data-test="filter-options-error"]')
            expect(line.text()).toBe(
                'Could not load the values to pick from. Stored values are shown as they are; reload the page to try again.'
            )
            expect(line.attributes('role')).toBe('alert')
            expect(w.text()).not.toContain('not configured')
        })

        it('shows no such line while the options are merely still loading', () => {
            filterOptionsRef.value = undefined
            const w = mountBuilder([mkRow('scan_folder', ['Music'])])
            expect(w.find('[data-test="filter-options-error"]').exists()).toBe(false)
        })

        it('shows no such line, and flags a gone value again, once the options arrive', () => {
            const w = mountBuilder([mkRow('scan_folder', ['Gone'])])
            expect(w.find('[data-test="filter-options-error"]').exists()).toBe(false)
            expect(w.text()).toContain('not configured')
        })
    })

    // ---- Behaviour 10: a scan folder that is gone blocks every save ---------------
    // The dialog always sends `filters`, so a stored scan_folder value whose
    // folder is no longer configured is re-sent on every Save and refused with
    // a 422 — the library cannot be saved at all until it is dealt with, which
    // the admin has no way of knowing before trying.
    describe('behaviour 10: a dangling scan folder blocks saving', () => {
        it('names the gone folder and says the server refuses to save until it is dealt with', () => {
            const w = mountBuilder([mkRow('scan_folder', ['Music', 'Gone'])])
            const notes = row(w, 0).findAll('[data-test="dangling-folder-note"]')
            expect(notes).toHaveLength(1)
            expect(notes[0].text()).toBe(
                "Gone is not configured any more — the server refuses to save this library until you remove it or pick the folder's new name."
            )
        })

        it('shows no such note while the options have not loaded', () => {
            filterOptionsRef.value = undefined
            const w = mountBuilder([mkRow('scan_folder', ['Gone'])])
            expect(w.find('[data-test="dangling-folder-note"]').exists()).toBe(false)
        })

        it('shows no such note for a configured folder', () => {
            const w = mountBuilder([mkRow('scan_folder', ['Music'])])
            expect(w.find('[data-test="dangling-folder-note"]').exists()).toBe(false)
        })

        it('shows no such note for another field whose value the catalog no longer has', () => {
            // Only a scan_folder value is refused on save; a genre the catalog
            // lost is merely unmatched, and saving it back is allowed.
            const w = mountBuilder([mkRow('genre', ['Disco'])])
            expect(w.find('[data-test="dangling-folder-note"]').exists()).toBe(false)
        })
    })
})
