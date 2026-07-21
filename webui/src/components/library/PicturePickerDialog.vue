<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import Message from 'primevue/message'
import { useCoverArtSearch } from '@/composables/useCoverArtSearch'
import {
    PICTURE_SLOT_LABELS,
    candidateMatchesType,
    pictureTypeLabel,
    sortCandidatesForType
} from '@/lib/pictureTypes'
import type { CoverCandidate, PictureSlot, StagedPictureSource } from '@/types/metadata'

const props = defineProps<{
    visible: boolean
    // The picture type + storage slot this picker fills — chosen before the
    // dialog opens, shown in the header. (`slot` is reserved in Vue templates,
    // hence pictureSlot.)
    pictureType: string
    pictureSlot: PictureSlot
    releaseMbid: string
    releaseGroupMbid: string
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    // Emitted when the user confirms a choice. The image is NOT persisted here;
    // the parent stages it and writes it on Save.
    (e: 'select', source: StagedPictureSource): void
}>()

const { candidates, loading: searching, error: searchError, search } = useCoverArtSearch()

const selectedCandidate = ref<CoverCandidate | null>(null)
const uploadFile = ref<File | null>(null)
const uploadPreview = ref<string | null>(null)
// The dialog never searches on open; the user triggers it explicitly.
const searched = ref(false)

const header = computed(
    () =>
        `Change ${pictureTypeLabel(props.pictureType).toLowerCase()} — ` +
        PICTURE_SLOT_LABELS[props.pictureSlot]
)
const canSearch = computed(
    () => props.releaseMbid.trim() !== '' || props.releaseGroupMbid.trim() !== ''
)
const hasSource = computed(() => selectedCandidate.value !== null || uploadFile.value !== null)
// Candidates depicting the requested type sort first.
const sortedCandidates = computed(() =>
    sortCandidatesForType(candidates.value, props.pictureType)
)

function resetState() {
    selectedCandidate.value = null
    clearUpload()
    searched.value = false
    candidates.value = []
}

watch(
    () => props.visible,
    (visible) => {
        if (visible) resetState()
    },
    { immediate: true }
)

function runSearch() {
    searched.value = true
    search(props.releaseMbid, props.releaseGroupMbid)
}

function pickCandidate(c: CoverCandidate) {
    selectedCandidate.value = c
    clearUpload()
}

// coverDescription summarises what an image depicts, from its Cover Art Archive
// types (e.g. "Front", "Back", "Booklet, Medium"), falling back to "Front"/"Cover".
function coverDescription(c: CoverCandidate): string {
    if (c.types && c.types.length > 0) return c.types.join(', ')
    if (c.isFront) return 'Front'
    return 'Cover'
}

function onFileChange(e: Event) {
    const input = e.target as HTMLInputElement
    const file = input.files?.[0] ?? null
    if (!file) return
    selectedCandidate.value = null
    uploadFile.value = file
    if (uploadPreview.value) URL.revokeObjectURL(uploadPreview.value)
    uploadPreview.value = URL.createObjectURL(file)
}

function clearUpload() {
    uploadFile.value = null
    if (uploadPreview.value) {
        URL.revokeObjectURL(uploadPreview.value)
        uploadPreview.value = null
    }
}

function confirmSelection() {
    if (!hasSource.value) return
    emit('select', {
        file: uploadFile.value,
        imageUrl: uploadFile.value ? null : (selectedCandidate.value?.imageUrl ?? null)
    })
    emit('update:visible', false)
}

function cancel() {
    emit('update:visible', false)
}
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="emit('update:visible', $event)"
        modal
        :header="header"
        :style="{ width: '48rem' }"
    >
        <section class="picker-section">
            <h4>From Cover Art Archive</h4>
            <Button
                label="Search Cover Art Archive"
                icon="pi pi-search"
                size="small"
                outlined
                data-test="picture-search"
                :disabled="!canSearch"
                :loading="searching"
                v-tooltip.right="
                    canSearch ? '' : 'Set a release or release-group MusicBrainz ID to search'
                "
                @click="runSearch"
            />
            <Message v-if="searchError" severity="error" :closable="false">{{
                searchError
            }}</Message>
            <ul v-if="!searching && sortedCandidates.length > 0" class="cover-list">
                <li
                    v-for="c in sortedCandidates"
                    :key="c.id"
                    class="cover-row"
                    :class="{ selected: selectedCandidate?.id === c.id }"
                    @click="pickCandidate(c)"
                >
                    <img class="cover-thumb" :src="c.thumbUrl" :alt="coverDescription(c)" />
                    <div class="cover-info">
                        <span class="cover-desc">
                            {{ coverDescription(c) }}
                            <span
                                v-if="candidateMatchesType(c, pictureType)"
                                class="type-match"
                                data-test="type-match"
                            >
                                matches {{ pictureTypeLabel(pictureType).toLowerCase() }}
                            </span>
                        </span>
                        <span v-if="c.comment" class="cover-comment">{{ c.comment }}</span>
                    </div>
                    <i v-if="selectedCandidate?.id === c.id" class="pi pi-check cover-check"></i>
                </li>
            </ul>
            <p v-else-if="searched && !searching && !searchError" class="no-results">
                No images found for this release.
            </p>
        </section>

        <section class="picker-section">
            <h4>Or upload a file</h4>
            <input
                type="file"
                accept="image/png,image/jpeg"
                data-test="picture-upload"
                @change="onFileChange"
            />
            <div v-if="uploadPreview" class="upload-preview">
                <img :src="uploadPreview" alt="Upload preview" />
            </div>
        </section>

        <template #footer>
            <Button label="Cancel" text @click="cancel" />
            <Button
                label="Select"
                data-test="picture-select"
                :disabled="!hasSource"
                @click="confirmSelection"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.picker-section {
    margin-bottom: 1rem;
}
.picker-section h4 {
    margin: 0 0 0.5rem;
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--app-text-secondary);
}
.cover-list {
    list-style: none;
    margin: 0.75rem 0 0;
    padding: 0;
    max-height: 24rem;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
}
.cover-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.5rem;
    border: 1px solid transparent;
    border-radius: 6px;
    cursor: pointer;
}
.cover-row:hover {
    background: var(--app-bg-subtle, #f3f4f6);
}
.cover-row.selected {
    border-color: var(--app-accent);
    background: var(--app-accent-soft);
}
.cover-thumb {
    width: 4rem;
    height: 4rem;
    object-fit: cover;
    border-radius: 4px;
    flex: 0 0 auto;
    background: var(--app-bg-subtle, #f3f4f6);
}
.cover-info {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    min-width: 0;
}
.cover-desc {
    font-weight: 500;
}
.type-match {
    font-size: 0.7rem;
    font-weight: 600;
    color: var(--app-accent);
    border: 1px solid var(--app-accent);
    border-radius: 999px;
    padding: 0.05rem 0.45rem;
    margin-left: 0.35rem;
    vertical-align: middle;
}
.cover-comment {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.cover-check {
    margin-left: auto;
    color: var(--app-accent);
}
.no-results {
    text-align: center;
    color: var(--app-text-secondary);
    padding: 1rem 0;
    margin: 0.5rem 0 0;
}
.upload-preview {
    margin-top: 0.5rem;
}
.upload-preview img {
    max-width: 6rem;
    max-height: 6rem;
    border-radius: 6px;
}
</style>
