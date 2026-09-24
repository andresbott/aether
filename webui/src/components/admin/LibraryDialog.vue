<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Checkbox from 'primevue/checkbox'
import Select from 'primevue/select'
import Message from 'primevue/message'
import IconSelect from '@/components/common/IconSelect.vue'
import LibraryFilterBuilder from '@/components/admin/LibraryFilterBuilder.vue'
import SidebarLayoutPicker from '@/components/admin/SidebarLayoutPicker.vue'
import { apiFieldErrorMap } from '@/lib/apiError'
import { DEFAULT_LIBRARY_ICON } from '@/lib/libraryIcons'
import { ALL_LIBRARY_VIEWS, LIBRARY_VIEWS, inDisplayOrder, openingView } from '@/lib/libraryViews'
import type { Library, LibraryFilter, LibraryInput, LibraryView } from '@/types/libraries'

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
    views: LibraryView[]
    default_view: LibraryView
    hide_from_artist_index: boolean
    split_views: boolean
    icon: string
    filters: LibraryFilter[]
}

// A new library browses like the whole catalog: every view, opening on
// Discover — the server's defaults too.
function emptyForm(): FormState {
    return {
        name: '',
        views: [...ALL_LIBRARY_VIEWS],
        default_view: 'discover',
        hide_from_artist_index: false,
        split_views: false,
        icon: DEFAULT_LIBRARY_ICON,
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

// True while the icon picker (a PrimeVue Popover) is open — and until the key
// event that closed it has finished. A TRUSTED key press lets microtasks run
// between the popover's own keydown handler and the document-level listeners,
// and PrimeVue emits `hide` at the start of the leave: clearing this flag
// synchronously would hand `closeOnEscape` back to the Dialog before the same
// Escape reaches it. A macrotask cannot run mid-dispatch.
const iconPickerOpen = ref(false)
let iconPickerCloseTimer: ReturnType<typeof setTimeout> | undefined

function onIconPickerOpenChange(open: boolean) {
    clearTimeout(iconPickerCloseTimer)
    if (open) {
        iconPickerOpen.value = true
        return
    }
    iconPickerCloseTimer = setTimeout(() => {
        iconPickerOpen.value = false
    }, 0)
}

onBeforeUnmount(() => clearTimeout(iconPickerCloseTimer))

watch(
    () => [props.visible, props.library],
    () => {
        if (!props.visible) return
        if (props.library) {
            const lib = props.library
            form.value = {
                name: lib.name,
                views: [...lib.views],
                default_view: lib.default_view,
                hide_from_artist_index: lib.hide_from_artist_index,
                split_views: lib.split_views,
                icon: lib.icon || DEFAULT_LIBRARY_ICON,
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
        clearTimeout(iconPickerCloseTimer)
        iconPickerOpen.value = false
    },
    { immediate: true }
)

const isEditMode = computed(() => props.library !== null)

// The views are a set shown in display order; the one the library opens on is
// picked among them. Unticking the view it opened on moves it to the first view
// left, and the last view cannot be unticked: a library needs at least one.
const openOptions = computed(() => LIBRARY_VIEWS.filter((v) => form.value.views.includes(v.value)))

watch(
    () => form.value.views,
    (views) => {
        if (!views.includes(form.value.default_view)) {
            form.value.default_view = openingView(views) ?? 'albums'
        }
    }
)

function isLastView(view: LibraryView): boolean {
    return form.value.views.length === 1 && form.value.views[0] === view
}

// A failed submit's per-field validation errors, keyed by the JSON Pointer the
// backend names: /name, /icon, the /views family, /default_view,
// /hide_from_artist_index, plus the /filters family (handled entirely by the
// builder — see isFilterPointer below).
const fieldErrors = computed(() => apiFieldErrorMap(props.error))
const KNOWN_POINTERS = ['/name', '/default_view', '/icon', '/hide_from_artist_index']

function isFilterPointer(pointer: string): boolean {
    return pointer === '/filters' || pointer.startsWith('/filters/')
}

// `/views` (none ticked) or `/views/<i>` (an unknown value): all shown on the
// one Views row.
function isViewsPointer(pointer: string): boolean {
    return pointer === '/views' || pointer.startsWith('/views/')
}

const viewsError = computed(() =>
    Object.entries(fieldErrors.value)
        .filter(([pointer]) => isViewsPointer(pointer))
        .map(([, detail]) => detail)
        .join(' ')
)

// Any field error whose pointer we don't render inline (e.g. a future field) is
// shown as a general message so a validation failure is never swallowed silently.
const otherErrors = computed(() =>
    Object.entries(fieldErrors.value)
        .filter(
            ([pointer]) =>
                !KNOWN_POINTERS.includes(pointer) &&
                !isFilterPointer(pointer) &&
                !isViewsPointer(pointer)
        )
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
        views: inDisplayOrder(form.value.views),
        default_view: form.value.default_view,
        hide_from_artist_index: form.value.hide_from_artist_index,
        split_views: form.value.split_views,
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

            <label>Icon</label>
            <IconSelect v-model="form.icon" @update:open="onIconPickerOpenChange" />
            <Message
                v-if="fieldErrors['/icon']"
                class="field-error"
                severity="error"
                size="small"
                variant="simple"
            >
                {{ fieldErrors['/icon'] }}
            </Message>

            <!-- The pages the library offers, in the order its view switcher shows
                 them. A native group: each box has its own label, the row is
                 named by the grid label. -->
            <label id="library-views-label">Views</label>
            <div class="views-field" role="group" aria-labelledby="library-views-label">
                <div v-for="view in LIBRARY_VIEWS" :key="view.value" class="check-option">
                    <Checkbox
                        v-model="form.views"
                        :inputId="`library-view-${view.value}`"
                        :value="view.value"
                        :disabled="isLastView(view.value)"
                        :invalid="!!viewsError"
                    />
                    <label :for="`library-view-${view.value}`">
                        <i :class="view.icon" aria-hidden="true"></i>
                        {{ view.label }}
                    </label>
                </div>
            </div>
            <Message
                v-if="viewsError"
                class="field-error"
                severity="error"
                size="small"
                variant="simple"
            >
                {{ viewsError }}
            </Message>

            <!-- The Select's combobox is a span, which <label for> cannot name. -->
            <label id="library-default-view-label">Opens on</label>
            <Select
                v-model="form.default_view"
                ariaLabelledby="library-default-view-label"
                :options="openOptions"
                optionLabel="label"
                optionValue="value"
                :disabled="openOptions.length < 2"
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

            <!-- How the library sits in the sidebar: one entry whose page
                 switches views, or a section of its own with an entry per view. -->
            <label id="library-sidebar-label">Sidebar</label>
            <SidebarLayoutPicker
                v-model="form.split_views"
                name="library-sidebar"
                ariaLabelledby="library-sidebar-label"
            />

            <!-- Independent of the Artists view above: this only takes the
                 artists off the main Artists page; the library's own Artists
                 view, when ticked, still lists them. -->
            <label for="library-hide-artists">Main Artists page</label>
            <div class="check-option">
                <Checkbox
                    v-model="form.hide_from_artist_index"
                    inputId="library-hide-artists"
                    binary
                    :invalid="!!fieldErrors['/hide_from_artist_index']"
                />
                <label for="library-hide-artists">Hide this library's artists</label>
            </div>
            <Message
                v-if="fieldErrors['/hide_from_artist_index']"
                class="field-error"
                severity="error"
                size="small"
                variant="simple"
            >
                {{ fieldErrors['/hide_from_artist_index'] }}
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
/* The row labels only — a checkbox's own label stays at body weight. */
.form-grid > label {
    font-weight: 500;
}
.views-field {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem 1.25rem;
}
.check-option {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}
.check-option label {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    cursor: pointer;
}
.check-option i {
    color: var(--app-text-secondary);
    font-size: 0.95rem;
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
