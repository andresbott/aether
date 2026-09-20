<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import Select from 'primevue/select'
import MultiSelect from 'primevue/multiselect'
import AutoComplete from 'primevue/autocomplete'
import SelectButton from 'primevue/selectbutton'
import Button from 'primevue/button'
import Message from 'primevue/message'
import FolderPickerDialog from '@/components/admin/FolderPickerDialog.vue'
import { useLibraryFilterOptions } from '@/composables/useLibraries'
import { useScanFolders } from '@/composables/useScanFolders'
import { previewLibrary } from '@/lib/api/Libraries'
import { isCanceledError } from '@/lib/apiError'
import {
    FILTER_FIELDS,
    LIMITS,
    optionsFor,
    rowErrors,
    valueLabel,
    type FieldMeta,
    type ValueOption
} from '@/lib/libraryFilters'
import type { LibraryFilter, LibraryFilterField } from '@/types/libraries'

// A library's filters are AND-ed, the values inside one filter are OR-ed, and
// no filters at all means the whole catalog (see lib/libraryFilters.ts). This
// is a controlled component: every change is emitted rather than held
// locally, so the parent dialog owns the actual filter list.
const props = defineProps<{
    modelValue: LibraryFilter[]
    errors: Record<string, string>
}>()

const emit = defineEmits<{
    (e: 'update:modelValue', value: LibraryFilter[]): void
}>()

const { data: filterOptions, isError: filterOptionsFailed } = useLibraryFilterOptions()
const { data: scanFolders } = useScanFolders()

const yesNoOptions = [
    { label: 'Yes', value: 'true' },
    { label: 'No', value: 'false' }
]

function fieldMeta(field: LibraryFilterField): FieldMeta {
    return FILTER_FIELDS.find((m) => m.field === field) ?? FILTER_FIELDS[0]
}

// scan-folders and options both pick from a server-offered list via the same
// MultiSelect; paths and yes/no get their own controls in the template.
function isChipControl(field: LibraryFilterField): boolean {
    const control = fieldMeta(field).control
    return control === 'scan-folders' || control === 'options'
}

// `.value` is read explicitly here (never bare `filterOptions` in the
// template) so this keeps working however the composable is provided —
// including a spec's plain `{ data: { value } }` stand-in.
function optionsForRow(row: LibraryFilter): ValueOption[] {
    return optionsFor(row.field, row.values, filterOptions.value)
}

// Until the lists arrive nothing is known about what a field offers, so
// optionsFor() shows the stored values plainly instead of calling them gone
// (see lib/libraryFilters.ts). A failure never delivers them at all, and then
// the empty pick-lists need saying out loud rather than looking like an empty
// catalog.
const optionsFailed = computed(() => filterOptionsFailed.value === true)

function addFilter() {
    if (props.modelValue.length >= LIMITS.filters) return
    emit('update:modelValue', [...props.modelValue, { field: 'scan_folder', values: [] }])
}

function removeFilter(idx: number) {
    emit('update:modelValue', props.modelValue.filter((_, i) => i !== idx))
}

// Changing a row's field always starts its values over: the old values belong
// to the old field's option set and are meaningless (often invalid) against
// the new one.
function onFieldChange(idx: number, field: LibraryFilterField) {
    emit(
        'update:modelValue',
        props.modelValue.map((f, i) => (i === idx ? { field, values: [] } : f))
    )
}

function onValuesChange(idx: number, values: string[]) {
    emit('update:modelValue', props.modelValue.map((f, i) => (i === idx ? { ...f, values } : f)))
}

// SelectButton is single-valued; a row's `values` is the array shape every
// other control shares, so a pick becomes a one-element array and a deselect
// (allowEmpty) becomes none — a valid, incomplete row, same as a fresh one.
function onYesNoChange(idx: number, value: string | null) {
    onValuesChange(idx, value ? [value] : [])
}

// --- Folder browsing (path filters) -----------------------------------------
// One dialog instance shared by every path row; `browsingRowIndex` says which
// row a confirmed pick is appended to.
const browsingRowIndex = ref<number | null>(null)

function openBrowseFor(idx: number) {
    browsingRowIndex.value = idx
}

function onBrowseVisibleChange(visible: boolean) {
    if (!visible) browsingRowIndex.value = null
}

function onBrowseSelect(path: string) {
    const idx = browsingRowIndex.value
    const row = idx === null ? undefined : props.modelValue[idx]
    if (idx === null || !row || row.values.includes(path)) return
    onValuesChange(idx, [...row.values, path])
}

// --- Scan folders that are gone -------------------------------------------------
// A stored scan_folder value naming a folder that is no longer configured is
// refused by the server (422 at /filters/i/values/j), and the dialog always
// sends `filters` — so it blocks saving the whole library, not just that row,
// with nothing to say so before the first Save. While the options are still
// loading nothing is flagged `missing` at all (see optionsFor), so this is
// silent until they have landed.
function danglingFolders(row: LibraryFilter): string[] {
    if (row.field !== 'scan_folder') return []
    return optionsForRow(row)
        .filter((o) => o.missing)
        .map((o) => o.value)
}

// --- Scan folders that are no longer usable -----------------------------------
interface UnavailableNote {
    name: string
    problem: string
}

function unavailableNotes(row: LibraryFilter): UnavailableNote[] {
    if (row.field !== 'scan_folder') return []
    const folders = scanFolders.value ?? []
    return row.values.flatMap((v) => {
        const folder = folders.find((f) => f.name === v)
        return folder && !folder.available ? [{ name: folder.name, problem: folder.problem ?? '' }] : []
    })
}

// --- Server errors, matched by row index ---------------------------------------
const errorInfo = computed(() => rowErrors(props.errors))
const generalErrors = computed(() => errorInfo.value.general)

function rowError(idx: number) {
    return errorInfo.value.rows[idx] ?? { value: {} }
}

function rowValueErrors(idx: number): { index: number; detail: string }[] {
    const perValue = errorInfo.value.rows[idx]?.value ?? {}
    return Object.entries(perValue).map(([i, detail]) => ({ index: Number(i), detail }))
}

// Whichever control holds the values of a complained-about row shows the error
// state — whether the server named the list (/filters/i/values) or one value
// inside it (/filters/i/values/j), the admin has to fix it in the same place.
function rowInvalid(idx: number): boolean {
    const err = rowError(idx)
    return !!err.values || Object.keys(err.value).length > 0
}

// --- Limits (advisory only: the server still has the last word) -----------------
const totalValues = computed(() => props.modelValue.reduce((sum, f) => sum + f.values.length, 0))

// --- Live preview ----------------------------------------------------------
// Debounced like MetadataEditorView's folder search: a plain setTimeout,
// cleared on every change and on unmount. `previewAbort` also supersedes an
// in-flight request the instant a newer one starts, and is nulled (not just
// aborted) on unmount so a response landing after teardown is always
// recognised as stale and ignored.
type PreviewState =
    | { status: 'ok'; trackCount: number; albumCount: number }
    | { status: 'invalid' }
    | { status: 'none' }

const previewState = ref<PreviewState>({ status: 'none' })
let previewTimer: ReturnType<typeof setTimeout> | undefined
let previewAbort: AbortController | null = null

function isValidationError(err: unknown): boolean {
    return (
        typeof err === 'object' &&
        err !== null &&
        (err as { response?: { status?: number } }).response?.status === 422
    )
}

async function runPreview() {
    previewAbort?.abort()
    const abort = new AbortController()
    previewAbort = abort
    try {
        const result = await previewLibrary(props.modelValue, abort.signal)
        if (previewAbort !== abort) return
        previewState.value = { status: 'ok', trackCount: result.track_count, albumCount: result.album_count }
    } catch (err) {
        if (previewAbort !== abort || isCanceledError(err)) return
        previewState.value = isValidationError(err) ? { status: 'invalid' } : { status: 'none' }
    } finally {
        if (previewAbort === abort) previewAbort = null
    }
}

function schedulePreview() {
    if (previewTimer) clearTimeout(previewTimer)
    previewTimer = setTimeout(runPreview, 400)
}

// Any external change to the filters (including one this component just
// emitted and got back via v-model) restarts the debounce.
watch(() => props.modelValue, schedulePreview)

onMounted(() => {
    runPreview()
})

onUnmounted(() => {
    if (previewTimer) clearTimeout(previewTimer)
    previewAbort?.abort()
    previewAbort = null
})
</script>

<template>
    <div class="filter-builder">
        <p v-if="modelValue.length === 0" class="hint" data-test="empty-hint">
            No filters: this library shows the whole catalog.
        </p>

        <Message
            v-if="generalErrors.length > 0"
            severity="error"
            :closable="false"
            class="general-errors"
            data-test="general-errors"
        >
            <div v-for="(msg, i) in generalErrors" :key="i">{{ msg }}</div>
        </Message>

        <p v-if="totalValues > LIMITS.valuesTotal" class="hint limit-note" data-test="total-limit-note">
            At most {{ LIMITS.valuesTotal }} values across all filters.
        </p>

        <p
            v-if="optionsFailed"
            class="filter-error"
            role="alert"
            data-test="filter-options-error"
        >
            Could not load the values to pick from. Stored values are shown as they are; reload
            the page to try again.
        </p>

        <!-- Rows are keyed by position, not identity: a filter has no id of its
             own, the same field may repeat across rows, and the server's error
             pointers (/filters/N/...) address a row by that same position. -->
        <div
            v-for="(row, idx) in modelValue"
            :key="idx"
            class="filter-row"
            :data-test="`filter-row-${idx}`"
        >
            <div class="filter-row-main">
                <Select
                    :modelValue="row.field"
                    @update:modelValue="onFieldChange(idx, $event)"
                    :options="FILTER_FIELDS"
                    optionLabel="label"
                    optionValue="field"
                    aria-label="Filter field"
                    :invalid="!!rowError(idx).field"
                    class="field-select"
                />

                <div class="value-control">
                    <MultiSelect
                        v-if="isChipControl(row.field)"
                        :modelValue="row.values"
                        @update:modelValue="onValuesChange(idx, $event)"
                        :options="optionsForRow(row)"
                        optionLabel="label"
                        optionValue="value"
                        display="chip"
                        :filter="row.field === 'genre'"
                        :show-toggle-all="false"
                        :aria-label="`${fieldMeta(row.field).label} values`"
                        :invalid="rowInvalid(idx)"
                    >
                        <template #option="slotProps">
                            <span :class="{ 'option-missing': slotProps.option.missing }">
                                {{ slotProps.option.label }}
                            </span>
                        </template>
                    </MultiSelect>

                    <div v-else-if="fieldMeta(row.field).control === 'paths'" class="paths-control">
                        <!-- Typed entry is NOT verbatim: PrimeVue's chips input
                             commits event.target.value.trim() on Enter, so a
                             path whose real name is padded can only be added
                             with "Browse…", which appends the server's own path
                             unchanged. What this component must never do is
                             rewrite a value itself (see lib/libraryFilters.ts);
                             the server then Cleans a path and rejects a
                             relative one. LibraryFilterBuilder.spec.ts pins the
                             trimming against the real AutoComplete. -->
                        <AutoComplete
                            :modelValue="row.values"
                            @update:modelValue="onValuesChange(idx, $event)"
                            multiple
                            :typeahead="false"
                            placeholder="Add path and press Enter"
                            :aria-label="`${fieldMeta(row.field).label} values`"
                            :invalid="rowInvalid(idx)"
                            class="paths-input"
                        />
                        <Button label="Browse…" text data-test="browse-path" @click="openBrowseFor(idx)" />
                    </div>

                    <SelectButton
                        v-else-if="fieldMeta(row.field).control === 'yes-no'"
                        :modelValue="row.values[0] ?? null"
                        @update:modelValue="onYesNoChange(idx, $event)"
                        :options="yesNoOptions"
                        optionLabel="label"
                        optionValue="value"
                        :aria-label="`${fieldMeta(row.field).label} value`"
                        :invalid="rowInvalid(idx)"
                    />
                </div>

                <Button
                    icon="pi pi-trash"
                    text
                    rounded
                    size="small"
                    severity="danger"
                    :aria-label="`Remove filter ${idx + 1}`"
                    data-test="remove-filter"
                    @click="removeFilter(idx)"
                />
            </div>

            <small
                v-if="rowError(idx).field"
                class="filter-error"
                role="alert"
                data-test="row-field-error"
            >
                {{ rowError(idx).field }}
            </small>
            <small
                v-if="rowError(idx).values"
                class="filter-error"
                role="alert"
                data-test="row-values-error"
            >
                {{ rowError(idx).values }}
            </small>
            <small
                v-for="ve in rowValueErrors(idx)"
                :key="ve.index"
                class="filter-error"
                role="alert"
                data-test="row-value-error"
            >
                {{ valueLabel(row.field, row.values[ve.index]) }}: {{ ve.detail }}
            </small>

            <small
                v-for="name in danglingFolders(row)"
                :key="name"
                class="filter-error"
                data-test="dangling-folder-note"
            >
                {{ name }} is not configured any more — the server refuses to save this library
                until you remove it or pick the folder's new name.
            </small>

            <small
                v-for="note in unavailableNotes(row)"
                :key="note.name"
                class="hint"
                data-test="folder-unavailable-note"
            >
                {{ note.name }} is not usable right now — scans and re-indexing refuse it: {{ note.problem }}
            </small>

            <small
                v-if="row.values.length >= LIMITS.valuesPerFilter"
                class="hint limit-note"
                data-test="row-limit-note"
            >
                At most {{ LIMITS.valuesPerFilter }} values per filter.
            </small>
        </div>

        <Button
            label="Add filter"
            icon="pi pi-plus"
            data-test="add-filter"
            :disabled="modelValue.length >= LIMITS.filters"
            @click="addFilter"
        />

        <div
            v-if="previewState.status !== 'none'"
            class="filter-preview"
            role="status"
            data-test="filter-preview"
        >
            <template v-if="previewState.status === 'ok'">
                Matches {{ previewState.trackCount }} tracks in {{ previewState.albumCount }} albums.
            </template>
            <template v-else>Fix the filters to see what they match.</template>
        </div>

        <FolderPickerDialog
            :visible="browsingRowIndex !== null"
            @update:visible="onBrowseVisibleChange"
            @select="onBrowseSelect"
        />
    </div>
</template>

<style scoped>
.filter-builder {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
}
.hint {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
    margin: 0;
}
.filter-row {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    padding: 0.6rem 0.75rem;
    border: 1px solid var(--app-border);
    border-radius: 6px;
}
/* On a phone the three controls do not fit side by side: the value control is
   allowed to take its own line (14rem basis) rather than being squeezed to a
   few characters. */
.filter-row-main {
    display: flex;
    align-items: flex-start;
    gap: 0.5rem;
    flex-wrap: wrap;
}
.field-select {
    flex: 0 0 10rem;
}
.value-control {
    flex: 1 1 14rem;
    min-width: 0;
}
.value-control :deep(.p-multiselect),
.value-control :deep(.p-autocomplete) {
    width: 100%;
}
/* The chips input inside the AutoComplete is its own element and does not
   inherit that width — the same rule GenreChips.vue needs. */
.value-control :deep(.p-autocomplete-input-multiple) {
    width: 100%;
}
.paths-control {
    display: flex;
    align-items: center;
    gap: 0.4rem;
}
.paths-input {
    flex: 1;
    min-width: 0;
}
.filter-error {
    color: var(--p-red-600, #dc2626);
    font-size: 0.8rem;
    /* The builder spaces its children with `gap`, so the one <p> that carries
       this class must not add the browser's default paragraph margin on top. */
    margin: 0;
}
.limit-note {
    font-size: 0.8rem;
}
.option-missing {
    font-style: italic;
    color: var(--app-text-secondary);
}
.filter-preview {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
    padding-top: 0.5rem;
    border-top: 1px solid var(--app-border);
}
.general-errors div {
    margin: 0;
}
</style>
