<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Checkbox from 'primevue/checkbox'
import Chips from 'primevue/chips'
import Button from 'primevue/button'
import type { Track, PatchFields } from '@/types/metadata'
import {
    diffInitialValues,
    type EditValues
} from '@/composables/useMetadataEditor'

const props = defineProps<{
    selection: Track[]
    isSaving: boolean
}>()
const emit = defineEmits<{
    (e: 'save', fields: PatchFields): void
}>()

const isMass = computed(() => props.selection.length > 1)

type DirtyFlags = {
    title: boolean
    album: boolean
    artists: boolean
    album_artists: boolean
    year: boolean
    compilation: boolean
}

const values = ref<EditValues>({
    title: '', album: '', artists: [], album_artists: [],
    year: 0, compilation: false
})
const dirty = ref<DirtyFlags>({
    title: false, album: false, artists: false, album_artists: false,
    year: false, compilation: false
})
const placeholders = ref({
    title: '', album: '', year: '', artists: '', album_artists: ''
})

function reset() {
    if (props.selection.length === 0) return
    const diff = diffInitialValues(props.selection)
    values.value = {
        title: diff.title.value,
        album: diff.album.value,
        artists: [...diff.artists.value],
        album_artists: [...diff.album_artists.value],
        year: diff.year.value,
        compilation: diff.compilation.value
    }
    dirty.value = {
        title: false, album: false, artists: false, album_artists: false,
        year: false, compilation: false
    }
    placeholders.value = {
        title: diff.title.shared ? '' : '(multiple values)',
        album: diff.album.shared ? '' : '(multiple values)',
        year: diff.year.shared ? '' : '(multiple values)',
        artists: diff.artists.shared ? '' : '(multiple values)',
        album_artists: diff.album_artists.shared ? '' : '(multiple values)'
    }
}

watch(() => props.selection, reset, { immediate: true, deep: true })

const patchFields = computed<PatchFields>(() => {
    const out: PatchFields = {}
    if (dirty.value.album) out.album = values.value.album
    if (dirty.value.artists) out.artists = [...values.value.artists]
    if (dirty.value.album_artists) out.album_artists = [...values.value.album_artists]
    if (dirty.value.year) out.year = values.value.year
    if (dirty.value.compilation) out.compilation = values.value.compilation
    // title is only editable in single-track mode
    if (!isMass.value && dirty.value.title) out.title = values.value.title
    return out
})

const canSave = computed(() => Object.keys(patchFields.value).length > 0 && !props.isSaving)

function save() {
    emit('save', patchFields.value)
}
</script>

<template>
    <div v-if="selection.length === 0" class="empty">
        Select one or more tracks to edit.
    </div>
    <div v-else class="edit-panel">
        <h3>
            {{ isMass ? `Editing ${selection.length} tracks` : 'Editing 1 track' }}
        </h3>

        <div v-if="!isMass" class="field-row">
            <label>Title</label>
            <InputText
                v-model="values.title"
                @update:modelValue="dirty.title = true"
                :placeholder="placeholders.title"
            />
        </div>

        <div class="field-row">
            <label>Album</label>
            <InputText
                v-model="values.album"
                @update:modelValue="dirty.album = true"
                :placeholder="placeholders.album"
            />
        </div>

        <div class="field-row">
            <label>Artists</label>
            <Chips
                v-model="values.artists"
                @update:modelValue="dirty.artists = true"
                :placeholder="placeholders.artists"
            />
        </div>

        <div class="field-row">
            <label>Album artists</label>
            <Chips
                v-model="values.album_artists"
                @update:modelValue="dirty.album_artists = true"
                :placeholder="placeholders.album_artists"
            />
        </div>

        <div class="field-row">
            <label>Year</label>
            <InputNumber
                v-model="values.year"
                @update:modelValue="dirty.year = true"
                :useGrouping="false"
                :placeholder="placeholders.year"
            />
        </div>

        <div class="field-row">
            <label>Compilation</label>
            <Checkbox
                v-model="values.compilation"
                @update:modelValue="dirty.compilation = true"
                :binary="true"
            />
        </div>

        <div class="actions">
            <Button
                label="Save"
                icon="pi pi-save"
                :disabled="!canSave"
                :loading="isSaving"
                @click="save"
            />
        </div>
    </div>
</template>

<style scoped>
.edit-panel {
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
.field-row label {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
}
.field-row :deep(.p-inputtext),
.field-row :deep(.p-inputnumber),
.field-row :deep(.p-chips) {
    width: 100%;
}
.field-row :deep(.p-inputnumber-input) {
    width: 100%;
}
.actions {
    margin-top: 1rem;
    display: flex;
    justify-content: flex-end;
}
</style>
