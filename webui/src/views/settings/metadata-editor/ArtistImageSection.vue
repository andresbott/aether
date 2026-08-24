<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import ArtistImageSearchDialog from '@/components/library/ArtistImageSearchDialog.vue'
import CollapsibleSection from './CollapsibleSection.vue'
import PictureCell from './PictureCell.vue'
import { resolveArtistFolder, getArtistImageUrl } from '@/lib/api/Metadata'
import { formatImageMeta } from '@/lib/imageMeta'
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

// The stored image's size/dimensions/format, shown only when it is the image on
// display: a pending change replaces it, and its preview showed its own metadata
// in the picker.
const currentMeta = computed<string | null>(() => {
    if (op.value) return null
    const m = serverInfo.value?.current_image_meta
    return m ? formatImageMeta(m) : null
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
            url: pick.url,
            // Reuse the picker's thumbnail for the staged preview; the full url is
            // downloaded server-side on save.
            previewUrl: pick.previewUrl
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
        <PictureCell
            data-test="artist-image-cell"
            :image="cellPreview"
            alt="Artist image"
            :title="targetLabel"
            :subtitle="`artist “${artist}”`"
            :note="cellNote"
            :meta="currentMeta"
            :staged="!!op"
            :pending="op?.kind === 'set'"
            :removing="op?.kind === 'remove'"
            :can-remove="hasCurrent"
            :disabled="libraryId === null"
            change-test-id="artist-image-change"
            remove-test-id="artist-image-remove"
            undo-test-id="artist-image-undo"
            meta-test-id="artist-image-meta"
            @change="openPicker"
            @remove="removeImage"
            @undo="undo"
        />

        <ArtistImageSearchDialog
            v-model:visible="dialogVisible"
            :artistName="artist"
            allow-upload
            @select="onPickerSelect"
            @upload="onPickerUpload"
        />
    </CollapsibleSection>
</template>
