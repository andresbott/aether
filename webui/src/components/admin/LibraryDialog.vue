<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import ToggleSwitch from 'primevue/toggleswitch'
import Select from 'primevue/select'
import Message from 'primevue/message'
import IconSelect from '@/components/common/IconSelect.vue'
import LibraryFilterBuilder from '@/components/admin/LibraryFilterBuilder.vue'
import { apiFieldErrorMap } from '@/lib/apiError'
import type { Library, LibraryFilter, LibraryInput } from '@/types/libraries'

const props = defineProps<{
    visible: boolean
    library: Library | null  // null = create mode
    submitting: boolean
    // The last failed submit, so a 422's per-field errors[] can be shown on the
    // field its pointer names. Undefined until a submit fails.
    error?: unknown
}>()

const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    (e: 'submit', input: LibraryInput): void
    (e: 'cancel'): void
}>()

interface FormState {
    name: string
    show_artists: boolean
    default_view: 'albums' | 'artists'
    icon: string
    filters: LibraryFilter[]
}

function emptyForm(): FormState {
    return {
        name: '',
        show_artists: true,
        default_view: 'albums',
        icon: 'folder',
        filters: []
    }
}

const form = ref<FormState>(emptyForm())

// Whether the admin has edited the filters since the last failed submit — see
// the errors-visibility comment below, by the `builderErrors` computed.
const filtersTouchedSinceError = ref(false)

// True while the builder's folder picker is open — see the builder's
// `update:browsing` emit for why the Dialog below must then ignore Escape.
const pickerOpen = ref(false)

// The same for the icon picker: a PrimeVue Popover, which hides on Escape
// without stopping propagation — see IconSelect's `update:open` emit.
const iconPickerOpen = ref(false)

watch(
    () => [props.visible, props.library],
    () => {
        if (!props.visible) return
        if (props.library) {
            const lib = props.library
            form.value = {
                name: lib.name,
                show_artists: lib.show_artists,
                default_view: lib.default_view,
                icon: lib.icon || 'folder',
                // Copied, never the vue-query cache's own arrays: editing (even
                // abandoning an edit to) this form must not mutate objects other
                // views are reading.
                filters: lib.filters.map((f) => ({ field: f.field, values: [...f.values] }))
            }
        } else {
            form.value = emptyForm()
        }
        // A freshly (re)opened dialog is a new editing session: any stale-error
        // suppression left over from a previous visit no longer applies. The
        // builder and the icon picker unmount with the dialog's content and so
        // cannot report themselves closed on the way out; reset both here too.
        filtersTouchedSinceError.value = false
        pickerOpen.value = false
        iconPickerOpen.value = false
    },
    { immediate: true }
)

const isEditMode = computed(() => props.library !== null)

// A failed submit's per-field validation errors, keyed by the JSON Pointer the
// backend names (validateDTO in the libraries handler): /name, /default_view,
// /icon, /show_artists, plus the /filters family (handled entirely by the
// builder — see isFilterPointer below).
const fieldErrors = computed(() => apiFieldErrorMap(props.error))
const KNOWN_POINTERS = [
    '/name',
    '/default_view',
    '/icon',
    '/show_artists'
]

function isFilterPointer(pointer: string): boolean {
    return pointer === '/filters' || pointer.startsWith('/filters/')
}

// Any field error whose pointer we don't render inline (e.g. a future field) is
// shown as a general message so a validation failure is never swallowed silently.
const otherErrors = computed(() =>
    Object.entries(fieldErrors.value)
        .filter(([pointer]) => !KNOWN_POINTERS.includes(pointer) && !isFilterPointer(pointer))
        .map(([, detail]) => detail)
)

// The server's /filters… pointers are POSITIONAL — they index the filters as
// they were last SENT. If the admin edits the filters after a failed submit
// (removes a row, changes a row's field or values), a leftover /filters… error
// would attach to the wrong row, so it is hidden from the builder as soon as
// that happens; it returns only with the next failed submit (a new `error`
// prop, which resets the flag below). Errors of other fields are unaffected.
watch(
    () => props.error,
    () => {
        filtersTouchedSinceError.value = false
    }
)

const builderErrors = computed<Record<string, string>>(() => {
    if (!filtersTouchedSinceError.value) return fieldErrors.value
    const out: Record<string, string> = {}
    for (const [pointer, detail] of Object.entries(fieldErrors.value)) {
        if (!isFilterPointer(pointer)) out[pointer] = detail
    }
    return out
})

function onFiltersUpdate(filters: LibraryFilter[]) {
    form.value.filters = filters
    filtersTouchedSinceError.value = true
}

// The dialog always sends `filters` exactly as the builder holds it: on an
// update an absent key would keep the stored filters and `[]` would clear
// them, so round-tripping precisely what is shown avoids that ambiguity.
function buildInput(): LibraryInput {
    return {
        name: form.value.name.trim(),
        show_artists: form.value.show_artists,
        default_view: form.value.default_view,
        icon: form.value.icon,
        filters: form.value.filters
    }
}

function onSubmit() {
    emit('submit', buildInput())
}

function onCancel() {
    emit('cancel')
    emit('update:visible', false)
}

const defaultViewOptions = [
    { label: 'Albums', value: 'albums' },
    { label: 'Artists', value: 'artists' }
]
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="emit('update:visible', $event)"
        modal
        :header="isEditMode ? 'Edit Library' : 'Add Library'"
        :closeOnEscape="!pickerOpen && !iconPickerOpen"
        :style="{ width: 'min(92vw, 44rem)' }"
    >
        <Message
            v-if="otherErrors.length"
            class="form-error"
            severity="error"
            :closable="false"
        >
            <ul class="form-error-list">
                <li v-for="(m, i) in otherErrors" :key="i">{{ m }}</li>
            </ul>
        </Message>

        <div class="form-grid">
            <label for="library-name">Name</label>
            <InputText
                id="library-name"
                v-model="form.name"
                placeholder="e.g. Main"
                :invalid="!!fieldErrors['/name']"
            />
            <Message
                v-if="fieldErrors['/name']"
                class="field-error"
                severity="error"
                size="small"
                variant="simple"
            >
                {{ fieldErrors['/name'] }}
            </Message>

            <label>Show artists</label>
            <ToggleSwitch v-model="form.show_artists" :invalid="!!fieldErrors['/show_artists']" />
            <Message
                v-if="fieldErrors['/show_artists']"
                class="field-error"
                severity="error"
                size="small"
                variant="simple"
            >
                {{ fieldErrors['/show_artists'] }}
            </Message>

            <label>Default view</label>
            <Select
                v-model="form.default_view"
                :options="defaultViewOptions"
                optionLabel="label"
                optionValue="value"
                :invalid="!!fieldErrors['/default_view']"
            />
            <Message
                v-if="fieldErrors['/default_view']"
                class="field-error"
                severity="error"
                size="small"
                variant="simple"
            >
                {{ fieldErrors['/default_view'] }}
            </Message>

            <label>Icon</label>
            <IconSelect v-model="form.icon" @update:open="iconPickerOpen = $event" />
            <Message
                v-if="fieldErrors['/icon']"
                class="field-error"
                severity="error"
                size="small"
                variant="simple"
            >
                {{ fieldErrors['/icon'] }}
            </Message>
        </div>

        <div class="filters-section">
            <label class="filters-heading">Filters</label>
            <LibraryFilterBuilder
                :modelValue="form.filters"
                :errors="builderErrors"
                @update:modelValue="onFiltersUpdate"
                @update:browsing="pickerOpen = $event"
            />
            <p class="filters-help">
                Filters narrow the library: every filter must match; inside one filter any value may.
            </p>
        </div>

        <template #footer>
            <Button label="Cancel" text @click="onCancel" />
            <Button
                :label="isEditMode ? 'Save' : 'Create'"
                :loading="submitting"
                @click="onSubmit"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.form-grid {
    display: grid;
    grid-template-columns: 10rem 1fr;
    gap: 0.75rem 1rem;
    align-items: center;
}
.form-grid label {
    font-weight: 500;
}
.form-grid > .p-message {
    grid-column: 2 / 3;
}
.field-error {
    grid-column: 2 / 3;
    margin-top: -0.25rem;
}
.form-error {
    margin-bottom: 1rem;
}
.form-error-list {
    margin: 0;
    padding-left: 1.1rem;
}
.filters-section {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    margin-top: 1.25rem;
}
.filters-heading {
    font-weight: 500;
}
.filters-help {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
    margin: 0;
}
</style>
