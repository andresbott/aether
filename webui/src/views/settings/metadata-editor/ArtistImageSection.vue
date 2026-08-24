<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Button from 'primevue/button'
import ArtistImageSearchDialog from '@/components/library/ArtistImageSearchDialog.vue'
import CollapsibleSection from './CollapsibleSection.vue'
import { resolveArtistFolder, getArtistImageUrl } from '@/lib/api/Metadata'
import type { ArtistFolderInfo } from '@/types/metadata'
import type { ArtistImagePick } from '@/types/artists'
import type { EditSession } from '@/composables/useEditSession'

const props = defineProps<{
    libraryId: number | null
    session: EditSession
    // The currently selected folder (library-relative), or null. The section
    // appears only when this folder resolves to an artist folder (itself, or an
    // ancestor when an album/disc is selected).
    folderPath: string | null
}>()

// Whether the selected folder resolves to an artist folder, plus that folder's
// name, path and current image. Refreshed on folder change and after a save.
const serverInfo = ref<ArtistFolderInfo | null>(null)
let refreshSeq = 0
async function refresh() {
    const seq = ++refreshSeq
    if (props.libraryId === null || !props.folderPath) {
        serverInfo.value = null
        return
    }
    try {
        const info = await resolveArtistFolder(props.libraryId, props.folderPath)
        if (seq === refreshSeq) serverInfo.value = info
    } catch {
        if (seq === refreshSeq) serverInfo.value = null
    }
}
watch(
    [() => props.libraryId, () => props.folderPath, () => props.session.picturesSavedAt.value],
    refresh,
    { immediate: true }
)

const eligible = computed(() => serverInfo.value?.eligible === true)
const artist = computed(() => serverInfo.value?.artist ?? '')
const hasCurrent = computed(() => !!serverInfo.value?.current_image)
// The resolved artist folder — the selected folder itself, or an ancestor when an
// album/disc is selected. All staging, serving and writing key off THIS.
const folderKey = computed(() => serverInfo.value?.path ?? null)
const op = computed(() =>
    folderKey.value ? props.session.getArtistImageOp(folderKey.value) : undefined
)

const cellPreview = computed<string | null>(() => {
    const o = op.value
    if (o?.kind === 'set') return o.preview
    if (o?.kind === 'remove') return null
    if (hasCurrent.value && props.libraryId !== null && folderKey.value) {
        return getArtistImageUrl(props.libraryId, folderKey.value, props.session.picturesSavedAt.value)
    }
    return null
})

const targetLabel = computed(() => {
    const folder = folderKey.value ?? ''
    if (op.value?.kind === 'set') return `${folder}/artist.jpg`
    const name = serverInfo.value?.current_image || 'artist.jpg'
    return `${folder}/${name}`
})

const cellNote = computed(() => {
    const o = op.value
    if (o?.kind === 'set') return 'Pending — saves on Save'
    if (o?.kind === 'remove') return 'Will be removed on Save'
    if (hasCurrent.value) return serverInfo.value?.current_image ?? ''
    return 'No image'
})

const dialogVisible = ref(false)
function openPicker() {
    dialogVisible.value = true
}
function onPickerSelect(pick: ArtistImagePick) {
    if (folderKey.value) {
        props.session.stageArtistImageSet(folderKey.value, {
            file: null,
            mbid: pick.mbid,
            url: pick.url
        })
    }
}
function onPickerUpload(file: File) {
    if (folderKey.value) {
        props.session.stageArtistImageSet(folderKey.value, { file, mbid: null, url: null })
    }
}
function removeImage() {
    if (folderKey.value) props.session.stageArtistImageRemoval(folderKey.value)
}
function undo() {
    if (folderKey.value) props.session.discardArtistImageOp(folderKey.value)
}
</script>

<template>
    <CollapsibleSection
        v-if="eligible"
        title="Artist image"
        :help="
            'A portrait for this artist folder, written as artist.jpg alongside ' +
            'the albums. It is a fallback: a fetched or manually-set artist image ' +
            'in Aether still takes priority.'
        "
        data-test="artist-image-block"
    >
        <div
            class="artist-cell"
            :class="{ pending: op?.kind === 'set', removing: op?.kind === 'remove' }"
            data-test="artist-image-cell"
        >
            <div class="thumb">
                <img v-if="cellPreview" :src="cellPreview" class="thumb-img" alt="Artist image" />
                <div v-else class="thumb-empty"><i class="pi pi-user"></i></div>
            </div>
            <div class="cell-body">
                <div class="target" data-test="artist-image-target">{{ targetLabel }}</div>
                <div class="sub">artist “{{ artist }}”</div>
                <div class="cell-note">{{ cellNote }}</div>
                <div class="actions">
                    <Button
                        v-if="op"
                        icon="pi pi-undo"
                        label="Undo"
                        text
                        size="small"
                        data-test="artist-image-undo"
                        @click="undo"
                    />
                    <template v-else>
                        <Button
                            icon="pi pi-images"
                            :label="cellPreview ? 'Change' : 'Add image'"
                            text
                            size="small"
                            data-test="artist-image-change"
                            :disabled="libraryId === null"
                            @click="openPicker"
                        />
                        <Button
                            v-if="hasCurrent"
                            icon="pi pi-trash"
                            label="Remove"
                            text
                            size="small"
                            severity="danger"
                            data-test="artist-image-remove"
                            @click="removeImage"
                        />
                    </template>
                </div>
            </div>
        </div>

        <ArtistImageSearchDialog
            v-model:visible="dialogVisible"
            :artistName="artist"
            allow-upload
            @select="onPickerSelect"
            @upload="onPickerUpload"
        />
    </CollapsibleSection>
</template>

<style scoped>
.artist-cell {
    display: flex;
    gap: 0.75rem;
    align-items: flex-start;
    padding: 0.5rem;
    border: 1px solid var(--app-border);
    border-radius: 6px;
    text-align: left;
}
.artist-cell.pending,
.artist-cell.removing {
    border-color: var(--app-staged);
    background-color: var(--app-staged-soft);
}
.artist-cell.removing .cell-note {
    text-decoration: line-through;
}
.artist-cell.pending .cell-note,
.artist-cell.removing .cell-note {
    color: var(--app-staged);
}
.thumb {
    flex: 0 0 auto;
    width: 5rem;
    height: 5rem;
}
.thumb-img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: 6px;
    border: 1px solid var(--app-border);
    background: var(--app-bg-subtle, #f3f4f6);
    display: block;
}
.thumb-empty {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px dashed var(--app-border);
    border-radius: 6px;
    color: var(--app-text-secondary);
    font-size: 1.5rem;
}
.cell-body {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    min-width: 0;
}
.target {
    font-size: 0.85rem;
    font-weight: 600;
    word-break: break-all;
}
.sub {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}
.cell-note {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}
.actions {
    display: flex;
    gap: 0.25rem;
    margin-top: 0.25rem;
}
</style>
