<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Checkbox from 'primevue/checkbox'
import ToggleButton from 'primevue/togglebutton'
import Chips from 'primevue/chips'
import Button from 'primevue/button'
import type { Track, PatchFields } from '@/types/metadata'
import {
    diffInitialValues,
    buildPatchFields,
    type AppliedFlags,
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

const values = ref<EditValues>({
    title: '', album: '', artists: [], album_artists: [],
    year: 0, compilation: false
})
const applied = ref<AppliedFlags>({
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
    if (isMass.value) {
        applied.value = {
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
    } else {
        // Single-edit: all fields implicitly "applied" when user changes them;
        // we detect changes in `dirty`.
        applied.value = {
            title: true, album: true, artists: true, album_artists: true,
            year: true, compilation: true
        }
        placeholders.value = {
            title: '', album: '', year: '', artists: '', album_artists: ''
        }
    }
}

watch(() => props.selection, reset, { immediate: true, deep: true })

// For single-edit, only send fields that actually changed from initial.
const singleEditChangedFields = computed<PatchFields>(() => {
    if (isMass.value || props.selection.length !== 1) return {}
    const t = props.selection[0]
    const out: PatchFields = {}
    if (values.value.title !== t.title) out.title = values.value.title
    if (values.value.album !== t.album) out.album = values.value.album
    if (values.value.year !== t.year) out.year = values.value.year
    if (values.value.compilation !== t.compilation) out.compilation = values.value.compilation
    const arrEq = (a: string[], b: string[]) =>
        a.length === b.length && a.every((x, i) => x === b[i])
    if (!arrEq(values.value.artists, t.artists)) out.artists = [...values.value.artists]
    if (!arrEq(values.value.album_artists, t.album_artists)) out.album_artists = [...values.value.album_artists]
    return out
})

const patchFields = computed<PatchFields>(() => {
    if (isMass.value) return buildPatchFields(applied.value, values.value)
    return singleEditChangedFields.value
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

        <div class="field-row">
            <ToggleButton
                v-if="isMass"
                v-model="applied.title"
                onLabel="apply"
                offLabel="apply"
                class="apply-toggle"
            />
            <label>Title</label>
            <InputText v-model="values.title" :placeholder="placeholders.title" />
        </div>

        <div class="field-row">
            <ToggleButton
                v-if="isMass"
                v-model="applied.album"
                onLabel="apply"
                offLabel="apply"
                class="apply-toggle"
            />
            <label>Album</label>
            <InputText v-model="values.album" :placeholder="placeholders.album" />
        </div>

        <div class="field-row">
            <ToggleButton
                v-if="isMass"
                v-model="applied.artists"
                onLabel="apply"
                offLabel="apply"
                class="apply-toggle"
            />
            <label>Artists</label>
            <Chips v-model="values.artists" :placeholder="placeholders.artists" />
        </div>

        <div class="field-row">
            <ToggleButton
                v-if="isMass"
                v-model="applied.album_artists"
                onLabel="apply"
                offLabel="apply"
                class="apply-toggle"
            />
            <label>Album artists</label>
            <Chips v-model="values.album_artists" :placeholder="placeholders.album_artists" />
        </div>

        <div class="field-row">
            <ToggleButton
                v-if="isMass"
                v-model="applied.year"
                onLabel="apply"
                offLabel="apply"
                class="apply-toggle"
            />
            <label>Year</label>
            <InputNumber v-model="values.year" :useGrouping="false" />
        </div>

        <div class="field-row">
            <ToggleButton
                v-if="isMass"
                v-model="applied.compilation"
                onLabel="apply"
                offLabel="apply"
                class="apply-toggle"
            />
            <label>Compilation</label>
            <Checkbox v-model="values.compilation" :binary="true" />
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
    grid-template-columns: 6rem 8rem 1fr;
    align-items: center;
    gap: 0.5rem;
}
.field-row label {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
}
.apply-toggle {
    min-width: 6rem;
}
.actions {
    margin-top: 1rem;
    display: flex;
    justify-content: flex-end;
}
</style>
