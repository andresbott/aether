<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import InputSwitch from 'primevue/inputswitch'
import Textarea from 'primevue/textarea'
import Dropdown from 'primevue/dropdown'
import Message from 'primevue/message'
import type { Library, LibraryInput } from '@/types/libraries'

const props = defineProps<{
    visible: boolean
    library: Library | null  // null = create mode
    submitting: boolean
}>()

const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    (e: 'submit', input: LibraryInput): void
    (e: 'cancel'): void
}>()

interface FormState {
    name: string
    path: string
    excludesText: string
    follow_symlinks: boolean
    show_artists: boolean
    default_view: 'albums' | 'artists'
}

function emptyForm(): FormState {
    return {
        name: '',
        path: '',
        excludesText: '',
        follow_symlinks: true,
        show_artists: true,
        default_view: 'albums'
    }
}

const form = ref<FormState>(emptyForm())
const initialPath = ref('')

watch(
    () => [props.visible, props.library],
    () => {
        if (!props.visible) return
        if (props.library) {
            const lib = props.library
            form.value = {
                name: lib.name,
                path: lib.path,
                excludesText: (lib.exclude_patterns ?? []).join('\n'),
                follow_symlinks: lib.follow_symlinks,
                show_artists: lib.show_artists,
                default_view: lib.default_view
            }
            initialPath.value = lib.path
        } else {
            form.value = emptyForm()
            initialPath.value = ''
        }
    },
    { immediate: true }
)

const isEditMode = computed(() => props.library !== null)
const pathChanged = computed(() => isEditMode.value && form.value.path !== initialPath.value)

function buildInput(): LibraryInput {
    const excludes = form.value.excludesText
        .split('\n')
        .map((s) => s.trim())
        .filter((s) => s.length > 0)
    return {
        name: form.value.name.trim(),
        path: form.value.path.trim(),
        exclude_patterns: excludes,
        follow_symlinks: form.value.follow_symlinks,
        show_artists: form.value.show_artists,
        default_view: form.value.default_view
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
        <div class="form-grid">
            <label>Name</label>
            <InputText v-model="form.name" placeholder="e.g. Main" />

            <label>Path</label>
            <InputText v-model="form.path" placeholder="/srv/music" />

            <Message v-if="pathChanged" severity="warn" :closable="false">
                Changing the path will wipe existing tracks under the old path.
                The library will be empty until the next scan.
            </Message>

            <label>Follow symlinks</label>
            <InputSwitch v-model="form.follow_symlinks" />

            <label>Show artists</label>
            <InputSwitch v-model="form.show_artists" />

            <label>Default view</label>
            <Dropdown
                v-model="form.default_view"
                :options="defaultViewOptions"
                optionLabel="label"
                optionValue="value"
            />

            <label>Exclude patterns</label>
            <Textarea
                v-model="form.excludesText"
                rows="4"
                placeholder="One Go regex per line"
            />
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
</style>
