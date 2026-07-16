<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import InputText from 'primevue/inputtext'
import Button from 'primevue/button'
import FileUpload from 'primevue/fileupload'
import Message from 'primevue/message'
import type { InternetRadioStation } from '@/types/subsonic'
import type { RadioStationPrefill } from '@/types/radiobrowser'
import { subsonicClient } from '@/lib/api/subsonic'

const MAX_COVER_BYTES = 5 * 1024 * 1024

const props = defineProps<{
    station: InternetRadioStation | null
    prefill?: RadioStationPrefill | null
}>()

interface StationInput {
    name: string
    streamUrl: string
    homepageUrl?: string
    coverFile?: File
    coverClear?: boolean
}

const emit = defineEmits<{
    (e: 'change', payload: { input: StationInput; valid: boolean; dirty: boolean }): void
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
const baseline = ref<FormState>(emptyForm())
const selectedFile = ref<File | null>(null)
const previewUrl = ref<string | null>(null)
const coverClear = ref(false)
const sizeError = ref<string | null>(null)

const isEditMode = computed(() => props.station !== null)

function resetCoverState() {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
    selectedFile.value = null
    coverClear.value = false
    sizeError.value = null
}

// Seed the form from the station (edit), the prefill (create-from-Discover), or
// blanks. Snapshot the baseline afterward so `dirty` reflects user edits only.
watch(
    () => [props.station, props.prefill],
    () => {
        resetCoverState()
        if (props.station) {
            form.value = {
                name: props.station.name,
                streamUrl: props.station.streamUrl,
                homepageUrl: props.station.homepageUrl ?? ''
            }
        } else if (props.prefill) {
            const init = props.prefill
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
        baseline.value = { ...form.value }
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

const valid = computed(
    () =>
        form.value.name.trim().length > 0 &&
        form.value.streamUrl.trim().length > 0 &&
        sizeError.value === null
)

const dirty = computed(
    () =>
        form.value.name !== baseline.value.name ||
        form.value.streamUrl !== baseline.value.streamUrl ||
        form.value.homepageUrl !== baseline.value.homepageUrl ||
        selectedFile.value !== null ||
        coverClear.value
)

const input = computed<StationInput>(() => {
    const homepage = form.value.homepageUrl.trim()
    return {
        name: form.value.name.trim(),
        streamUrl: form.value.streamUrl.trim(),
        homepageUrl: homepage === '' ? undefined : homepage,
        coverFile: selectedFile.value ?? undefined,
        coverClear: coverClear.value || undefined
    }
})

watch(
    [input, valid, dirty],
    () => emit('change', { input: input.value, valid: valid.value, dirty: dirty.value }),
    { immediate: true }
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
</script>

<template>
    <div class="radio-station-form">
        <div class="field-row">
            <label>Name</label>
            <InputText class="field-name" v-model="form.name" placeholder="e.g. BBC Radio 1" />
        </div>

        <div class="field-row">
            <label>Stream URL</label>
            <InputText
                class="field-stream-url"
                v-model="form.streamUrl"
                placeholder="http://example.com/stream"
            />
        </div>

        <div class="field-row">
            <label>Homepage URL</label>
            <InputText class="field-homepage" v-model="form.homepageUrl" placeholder="optional" />
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
    </div>
</template>

<style scoped>
.radio-station-form {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
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
</style>
