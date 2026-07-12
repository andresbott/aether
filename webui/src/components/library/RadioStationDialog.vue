<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import FileUpload from 'primevue/fileupload'
import Message from 'primevue/message'
import type { InternetRadioStation } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'

const MAX_COVER_BYTES = 5 * 1024 * 1024

const props = defineProps<{
    visible: boolean
    station: InternetRadioStation | null
    submitting: boolean
}>()

const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    (
        e: 'submit',
        input: {
            name: string
            streamUrl: string
            homepageUrl?: string
            coverFile?: File
            coverClear?: boolean
        }
    ): void
    (e: 'cancel'): void
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

function resetCoverState() {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
    selectedFile.value = null
    coverClear.value = false
    sizeError.value = null
}

watch(
    () => [props.visible, props.station],
    () => {
        if (!props.visible) {
            resetCoverState()
            return
        }
        resetCoverState()
        if (props.station) {
            form.value = {
                name: props.station.name,
                streamUrl: props.station.streamUrl,
                homepageUrl: props.station.homepageUrl ?? ''
            }
        } else {
            form.value = emptyForm()
        }
    },
    { immediate: true }
)

const isEditMode = computed(() => props.station !== null)

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
    emit('submit', {
        name: form.value.name.trim(),
        streamUrl: form.value.streamUrl.trim(),
        homepageUrl: homepage === '' ? undefined : homepage,
        coverFile: selectedFile.value ?? undefined,
        coverClear: coverClear.value || undefined
    })
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
        :header="isEditMode ? 'Edit Station' : 'Add Station'"
        :style="{ width: '32rem' }"
    >
        <Message severity="warn" :closable="false" class="deprecation-note">
            Deprecated — use Settings → Radio Stations to add or edit stations.
        </Message>

        <div class="form-grid">
            <label>Name</label>
            <InputText v-model="form.name" placeholder="e.g. BBC Radio 1" @keyup.enter="onSubmit" />

            <label>Stream URL</label>
            <InputText
                v-model="form.streamUrl"
                placeholder="http://example.com/stream"
                @keyup.enter="onSubmit"
            />

            <label>Homepage URL</label>
            <InputText v-model="form.homepageUrl" placeholder="optional" @keyup.enter="onSubmit" />

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

        <template #footer>
            <Button label="Cancel" text @click="onCancel" />
            <Button
                :label="isEditMode ? 'Save' : 'Create'"
                :loading="submitting"
                :disabled="!canSubmit"
                @click="onSubmit"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.deprecation-note {
    margin-bottom: 0.75rem;
}
.form-grid {
    display: grid;
    grid-template-columns: 8rem 1fr;
    gap: 0.75rem 1rem;
    align-items: start;
}
.form-grid label {
    font-weight: 500;
    padding-top: 0.5rem;
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
</style>
