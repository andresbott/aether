<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import Message from 'primevue/message'
import RadioButton from 'primevue/radiobutton'
import { useCoverArtSearch } from '@/composables/useCoverArtSearch'
import type { CoverCandidate, CoverTarget, StagedCover } from '@/types/metadata'

const props = defineProps<{
    visible: boolean
    albumName: string
    releaseMbid: string
    releaseGroupMbid: string
    libraryId: number
    // Selected track paths (library-relative). Cover is resolved to the album
    // via these; embedded writes target exactly these tracks.
    paths: string[]
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    // Emitted when the user confirms a choice. The cover is NOT persisted here;
    // the parent stages it and writes it on Save.
    (e: 'select', cover: StagedCover): void
}>()

const { candidates, loading: searching, error: searchError, search } = useCoverArtSearch()

const selectedCandidate = ref<CoverCandidate | null>(null)
const uploadFile = ref<File | null>(null)
const uploadPreview = ref<string | null>(null)
const target = ref<CoverTarget>('db')

const hasSelection = computed(() => props.paths.length > 0)
const hasSource = computed(() => selectedCandidate.value !== null || uploadFile.value !== null)
const embeddedDisabled = computed(() => !hasSelection.value)
const canConfirm = computed(() => hasSource.value && hasSelection.value)

function resetState() {
    selectedCandidate.value = null
    clearUpload()
    target.value = 'db'
}

watch(
    () => props.visible,
    (visible) => {
        if (!visible) return
        resetState()
        search(props.releaseMbid, props.releaseGroupMbid)
    },
    { immediate: true }
)

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
    if (!canConfirm.value) return
    emit('select', {
        target: target.value,
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
        header="Change album cover"
        :style="{ width: '48rem' }"
    >
        <p v-if="!hasSelection" class="hint">Select one or more tracks to save a cover.</p>

        <section class="picker-section">
            <h4>From Cover Art Archive</h4>
            <Message v-if="searchError" severity="error" :closable="false">{{
                searchError
            }}</Message>
            <div v-if="searching" class="searching"><i class="pi pi-spin pi-spinner"></i></div>
            <ul v-else-if="candidates.length > 0" class="cover-list">
                <li
                    v-for="c in candidates"
                    :key="c.id"
                    class="cover-row"
                    :class="{ selected: selectedCandidate?.id === c.id }"
                    @click="pickCandidate(c)"
                >
                    <img class="cover-thumb" :src="c.thumbUrl" :alt="coverDescription(c)" />
                    <div class="cover-info">
                        <span class="cover-desc">{{ coverDescription(c) }}</span>
                        <span v-if="c.comment" class="cover-comment">{{ c.comment }}</span>
                    </div>
                    <i v-if="selectedCandidate?.id === c.id" class="pi pi-check cover-check"></i>
                </li>
            </ul>
            <p v-else class="no-results">No covers found for this release.</p>
        </section>

        <section class="picker-section">
            <h4>Or upload a file</h4>
            <input
                type="file"
                accept="image/png,image/jpeg"
                data-test="cover-upload"
                @change="onFileChange"
            />
            <div v-if="uploadPreview" class="upload-preview">
                <img :src="uploadPreview" alt="Upload preview" />
            </div>
        </section>

        <section class="picker-section">
            <h4>Save to</h4>
            <div class="target-option">
                <RadioButton v-model="target" inputId="cover-target-db" value="db" />
                <label for="cover-target-db">
                    aether store <small>— keeps your music files untouched</small>
                </label>
            </div>
            <div class="target-option">
                <RadioButton v-model="target" inputId="cover-target-folder" value="folder" />
                <label for="cover-target-folder">
                    Album folder <small>— writes cover.jpg / cover.png</small>
                </label>
            </div>
            <div class="target-option">
                <RadioButton
                    v-model="target"
                    inputId="cover-target-embedded"
                    value="embedded"
                    :disabled="embeddedDisabled"
                />
                <label for="cover-target-embedded" :class="{ disabled: embeddedDisabled }">
                    Embed in tags <small>— applies to {{ paths.length }} selected track(s)</small>
                </label>
            </div>
        </section>

        <template #footer>
            <Button label="Cancel" text @click="cancel" />
            <Button
                label="Select"
                data-test="cover-select"
                :disabled="!canConfirm"
                @click="confirmSelection"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.hint {
    color: var(--app-text-secondary);
    font-size: 0.9rem;
    margin: 0 0 0.75rem;
}
.picker-section {
    margin-bottom: 1rem;
}
.picker-section h4 {
    margin: 0 0 0.5rem;
    font-size: 0.9rem;
    font-weight: 600;
    color: var(--app-text-secondary);
}
.searching {
    display: flex;
    justify-content: center;
    padding: 1.5rem;
    color: var(--app-text-secondary);
}
.cover-list {
    list-style: none;
    margin: 0;
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
}
.upload-preview {
    margin-top: 0.5rem;
}
.upload-preview img {
    max-width: 6rem;
    max-height: 6rem;
    border-radius: 6px;
}
.target-option {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-bottom: 0.4rem;
}
.target-option label {
    cursor: pointer;
}
.target-option label.disabled {
    color: var(--app-text-secondary);
    cursor: not-allowed;
}
.target-option small {
    color: var(--app-text-secondary);
}
</style>
