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
    distinctArtistMbids,
    type EditValues,
    type ArtistMbidRow
} from '@/composables/useMetadataEditor'
import MusicBrainzArtistPicker from '@/components/library/MusicBrainzArtistPicker.vue'

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

// name -> new MBID overrides (dirty only), one map per field
const artistMbidEdits = ref<Record<string, string>>({})
const albumArtistMbidEdits = ref<Record<string, string>>({})

const artistMbidRows = computed<ArtistMbidRow[]>(() =>
    distinctArtistMbids(props.selection, 'artists', 'mb_artist_ids')
)
const albumArtistMbidRows = computed<ArtistMbidRow[]>(() =>
    distinctArtistMbids(props.selection, 'album_artists', 'mb_album_artist_ids')
)

// Picker dialog state
const picker = ref<{ open: boolean; scope: 'artist' | 'album_artist'; name: string; current: string }>({
    open: false, scope: 'artist', name: '', current: ''
})

function openPicker(scope: 'artist' | 'album_artist', row: ArtistMbidRow) {
    const edits = scope === 'artist' ? artistMbidEdits.value : albumArtistMbidEdits.value
    picker.value = {
        open: true,
        scope,
        name: row.name,
        current: row.name in edits ? edits[row.name] : row.mbid
    }
}

function onArtistMbidSelect(name: string, mbid: string) {
    artistMbidEdits.value = { ...artistMbidEdits.value, [name]: mbid }
}
function onAlbumArtistMbidSelect(name: string, mbid: string) {
    albumArtistMbidEdits.value = { ...albumArtistMbidEdits.value, [name]: mbid }
}
function onPickerSelect(mbid: string) {
    if (picker.value.scope === 'artist') onArtistMbidSelect(picker.value.name, mbid)
    else onAlbumArtistMbidSelect(picker.value.name, mbid)
}

function displayMbid(scope: 'artist' | 'album_artist', row: ArtistMbidRow): string {
    const edits = scope === 'artist' ? artistMbidEdits.value : albumArtistMbidEdits.value
    if (row.name in edits) return edits[row.name]
    return row.mixed ? '(mixed)' : row.mbid
}

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
    artistMbidEdits.value = {}
    albumArtistMbidEdits.value = {}
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
    if (Object.keys(artistMbidEdits.value).length > 0) out.artist_mbids = { ...artistMbidEdits.value }
    if (Object.keys(albumArtistMbidEdits.value).length > 0) out.album_artist_mbids = { ...albumArtistMbidEdits.value }
    return out
})

const canSave = computed(() => Object.keys(patchFields.value).length > 0 && !props.isSaving)

function save() {
    emit('save', patchFields.value)
}

defineExpose({ onArtistMbidSelect, onAlbumArtistMbidSelect })
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

        <div v-if="artistMbidRows.length" class="mbid-section">
            <h4>Artist MusicBrainz IDs</h4>
            <div v-for="row in artistMbidRows" :key="row.name" class="mbid-row">
                <span class="mbid-name">{{ row.name }}</span>
                <span class="mbid-value">{{ displayMbid('artist', row) || '—' }}</span>
                <Button label="Match" text size="small" @click="openPicker('artist', row)" />
            </div>
        </div>

        <div v-if="albumArtistMbidRows.length" class="mbid-section">
            <h4>Album-artist MusicBrainz IDs</h4>
            <div v-for="row in albumArtistMbidRows" :key="row.name" class="mbid-row">
                <span class="mbid-name">{{ row.name }}</span>
                <span class="mbid-value">{{ displayMbid('album_artist', row) || '—' }}</span>
                <Button label="Match" text size="small" @click="openPicker('album_artist', row)" />
            </div>
        </div>

        <MusicBrainzArtistPicker
            v-model:visible="picker.open"
            :artistName="picker.name"
            :currentMbid="picker.current"
            @select="onPickerSelect"
        />

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
.mbid-section h4 { margin: 0.5rem 0 0.25rem; font-size: 0.85rem; color: var(--app-text-secondary); }
.mbid-row { display: grid; grid-template-columns: 8rem 1fr auto; align-items: center; gap: 0.5rem; }
.mbid-name { font-size: 0.85rem; }
.mbid-value { font-size: 0.8rem; color: var(--app-text-secondary); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
</style>
