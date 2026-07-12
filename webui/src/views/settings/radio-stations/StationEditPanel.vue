<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import FileUpload from 'primevue/fileupload'
import Message from 'primevue/message'
import type { InternetRadioStation } from '@/types/subsonic'
import type { RadioStationPrefill } from '@/types/radiobrowser'
import { subsonicClient } from '@/lib/api/subsonic'

const MAX_COVER_BYTES = 5 * 1024 * 1024

const props = defineProps<{
    station: InternetRadioStation | null
    adding: boolean
    submitting: boolean
    // When adding, optional values to seed the form with (e.g. imported from
    // radio-browser). coverFile, when present, pre-fills the cover.
    initial?: RadioStationPrefill | null
}>()

const emit = defineEmits<{
    (
        e: 'save',
        input: {
            name: string
            streamUrl: string
            homepageUrl?: string
            coverFile?: File
            coverClear?: boolean
        }
    ): void
    (e: 'delete'): void
}>()

interface FormState {
    name: string
    streamUrl: string
    homepageUrl: string
}

function emptyForm(): FormState {
    return { name: '', streamUrl: '', homepageUrl: '' }
}

const form = ref<FormState>(emptyForm())
const selectedFile = ref<File | null>(null)
const previewUrl = ref<string | null>(null)
const coverClear = ref(false)
const sizeError = ref<string | null>(null)

// The panel edits an existing station, blanks for a new one, or sits idle.
const isEditMode = computed(() => props.station !== null)
const active = computed(() => props.adding || props.station !== null)

function resetCoverState() {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
    selectedFile.value = null
    coverClear.value = false
    sizeError.value = null
}

// Re-seed the form whenever the selected station, add mode, or the add-mode
// prefill changes.
watch(
    () => [props.station, props.adding, props.initial],
    () => {
        resetCoverState()
        if (props.station) {
            form.value = {
                name: props.station.name,
                streamUrl: props.station.streamUrl,
                homepageUrl: props.station.homepageUrl ?? ''
            }
        } else if (props.adding && props.initial) {
            const init = props.initial
            form.value = {
                name: init.name,
                streamUrl: init.streamUrl,
                homepageUrl: init.homepageUrl ?? ''
            }
            if (init.coverFile) {
                selectedFile.value = init.coverFile
                previewUrl.value = URL.createObjectURL(init.coverFile)
            }
        } else {
            form.value = emptyForm()
        }
    },
    { immediate: true }
)

const hasExistingCover = computed(
    () => isEditMode.value && !!props.station?.coverArt && !coverClear.value
)

const displayedCoverUrl = computed(() => {
    if (previewUrl.value) return previewUrl.value
    if (hasExistingCover.value && props.station?.coverArt) {
        return subsonicClient.getCoverArtUrl(props.station.coverArt, 256)
    }
    return null
})

const canSubmit = computed(
    () =>
        form.value.name.trim().length > 0 &&
        form.value.streamUrl.trim().length > 0 &&
        sizeError.value === null
)

function onFileSelect(event: { files: File[] }) {
    const file = event.files?.[0]
    if (!file) return
    if (file.size > MAX_COVER_BYTES) {
        sizeError.value = `File is ${(file.size / 1024 / 1024).toFixed(1)} MB — max is 5 MB`
        return
    }
    sizeError.value = null
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    selectedFile.value = file
    previewUrl.value = URL.createObjectURL(file)
    coverClear.value = false
}

function onRemoveCover() {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
    selectedFile.value = null
    coverClear.value = true
}

function onSubmit() {
    if (!canSubmit.value) return
    const homepage = form.value.homepageUrl.trim()
    emit('save', {
        name: form.value.name.trim(),
        streamUrl: form.value.streamUrl.trim(),
        homepageUrl: homepage === '' ? undefined : homepage,
        coverFile: selectedFile.value ?? undefined,
        coverClear: coverClear.value || undefined
    })
}
</script>

<template>
    <div v-if="!active" class="empty">Select a station to edit, or add a new one.</div>

    <div v-else class="edit-panel">
        <h3>{{ isEditMode ? 'Edit station' : 'Add station' }}</h3>

        <div class="field-row">
            <label>Name</label>
            <InputText
                class="field-name"
                v-model="form.name"
                placeholder="e.g. BBC Radio 1"
                @keyup.enter="onSubmit"
            />
        </div>

        <div class="field-row">
            <label>Stream URL</label>
            <InputText
                class="field-stream-url"
                v-model="form.streamUrl"
                placeholder="http://example.com/stream"
                @keyup.enter="onSubmit"
            />
        </div>

        <div class="field-row">
            <label>Homepage URL</label>
            <InputText
                class="field-homepage"
                v-model="form.homepageUrl"
                placeholder="optional"
                @keyup.enter="onSubmit"
            />
        </div>

        <div class="field-block">
            <label>Cover</label>
            <div class="cover-row">
                <div class="cover-preview">
                    <img v-if="displayedCoverUrl" :src="displayedCoverUrl" alt="cover" />
                    <div v-else class="cover-placeholder">
                        <i class="pi pi-image" style="font-size: 1.5rem"></i>
                    </div>
                </div>
                <div class="cover-actions">
                    <FileUpload
                        mode="basic"
                        accept="image/png,image/jpeg"
                        :maxFileSize="MAX_COVER_BYTES"
                        :auto="false"
                        chooseLabel="Choose image"
                        @select="onFileSelect"
                    />
                    <Button
                        v-if="hasExistingCover || selectedFile"
                        text
                        severity="secondary"
                        label="Remove cover"
                        @click="onRemoveCover"
                    />
                    <Message v-if="sizeError" severity="error" :closable="false">
                        {{ sizeError }}
                    </Message>
                    <small v-if="coverClear" class="cleared-note">
                        Cover will be cleared on save.
                    </small>
                </div>
            </div>
        </div>

        <div class="actions">
            <Button
                v-if="isEditMode"
                label="Delete"
                icon="pi pi-trash"
                text
                severity="danger"
                @click="emit('delete')"
            />
            <span class="actions-spacer" />
            <Button
                :label="isEditMode ? 'Save' : 'Create'"
                icon="pi pi-save"
                :loading="submitting"
                :disabled="!canSubmit"
                @click="onSubmit"
            />
        </div>
    </div>
</template>

<style scoped>
.edit-panel {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 1rem;
    background: var(--app-surface);
    border: 1px solid var(--app-border);
    border-radius: 6px;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
}
.empty {
    padding: 2rem;
    text-align: center;
    color: var(--app-text-secondary);
}
.field-row {
    display: grid;
    grid-template-columns: 8rem 1fr;
    align-items: center;
    gap: 0.5rem;
}
.field-block {
    display: grid;
    grid-template-columns: 8rem 1fr;
    align-items: start;
    gap: 0.5rem;
}
.field-row label,
.field-block label {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
}
.field-row :deep(.p-inputtext) {
    width: 100%;
}
.cover-row {
    display: flex;
    gap: 1rem;
    align-items: flex-start;
}
.cover-preview {
    width: 6rem;
    height: 6rem;
    border-radius: 8px;
    overflow: hidden;
    background: var(--app-bg-subtle, #f3f4f6);
    flex-shrink: 0;
}
.cover-preview img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}
.cover-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--app-text-secondary);
}
.cover-actions {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    align-items: flex-start;
    min-width: 0;
}
.cleared-note {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
}
.actions {
    margin-top: 1rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
}
.actions-spacer {
    flex: 1;
}
</style>
