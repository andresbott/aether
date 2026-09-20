<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import ToggleSwitch from 'primevue/toggleswitch'
import Select from 'primevue/select'
import Message from 'primevue/message'
import IconSelect from '@/components/common/IconSelect.vue'
import { apiFieldErrorMap } from '@/lib/apiError'
import type { Library, LibraryInput } from '@/types/libraries'

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
}

function emptyForm(): FormState {
    return {
        name: '',
        show_artists: true,
        default_view: 'albums',
        icon: 'folder'
    }
}

const form = ref<FormState>(emptyForm())

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
                icon: lib.icon || 'folder'
            }
        } else {
            form.value = emptyForm()
        }
    },
    { immediate: true }
)

const isEditMode = computed(() => props.library !== null)

// A failed submit's per-field validation errors, keyed by the JSON Pointer the
// backend names (validateDTO in the libraries handler): /name, /default_view,
// /icon.
const fieldErrors = computed(() => apiFieldErrorMap(props.error))
const KNOWN_POINTERS = [
    '/name',
    '/default_view',
    '/icon'
]
// Any field error whose pointer we don't render inline (e.g. a future field) is
// shown as a general message so a validation failure is never swallowed silently.
const otherErrors = computed(() =>
    Object.entries(fieldErrors.value)
        .filter(([pointer]) => !KNOWN_POINTERS.includes(pointer))
        .map(([, detail]) => detail)
)

// The filter UI doesn't exist yet (later tasks build it) — an edit round-trips
// the library's stored filters unchanged so nothing is lost on save.
function buildInput(): LibraryInput {
    return {
        name: form.value.name.trim(),
        show_artists: form.value.show_artists,
        default_view: form.value.default_view,
        icon: form.value.icon,
        filters: props.library?.filters ?? []
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
        :style="{ width: '32rem' }"
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
            <label>Name</label>
            <InputText
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
            <ToggleSwitch v-model="form.show_artists" />

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
            <IconSelect v-model="form.icon" />
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
</style>
