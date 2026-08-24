<script setup lang="ts">
import { ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { useMusicBrainzSearch } from '@/composables/useMusicBrainzSearch'
import { useArtistImageCandidates } from '@/composables/useArtistImageCandidates'
import type { ArtistImagePick, ArtistImageCandidate, MusicBrainzCandidate } from '@/types/artists'

const props = defineProps<{
    visible: boolean
    artistName: string
    // When true, also offer a local file upload alongside the online search
    // (used by the metadata editor's artist-folder image). Off for the artist
    // page, which only stores online picks.
    allowUpload?: boolean
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    // The user's confirmed pick. Nothing is written here — the parent stages it
    // and commits on its own Save, so Cancel discards it like any other edit.
    (e: 'select', pick: ArtistImagePick): void
    // A locally uploaded file (only when allowUpload); staged like a pick.
    (e: 'upload', file: File): void
}>()

const fileInput = ref<HTMLInputElement | null>(null)
function triggerUpload(): void {
    fileInput.value?.click()
}
function onFileChange(e: Event): void {
    const input = e.target as HTMLInputElement
    const file = input.files?.[0]
    input.value = '' // let the same file be re-picked after a cancel
    if (!file) return
    emit('upload', file)
    emit('update:visible', false)
}

// Same MusicBrainz artist search the MBID picker and the auto-fetch job use — the
// image comes from the same provider chain, driven by the user's pick instead of
// the artist's stored match.
const { results, loading: searching, error: searchError, search } = useMusicBrainzSearch()
// The candidate portraits for whichever MusicBrainz artist is picked below.
const { candidates, loading: loadingImages, error: imagesError, load, reset } = useArtistImageCandidates()

const query = ref('')
const pickedArtist = ref<MusicBrainzCandidate | null>(null)
const pickedImage = ref<ArtistImageCandidate | null>(null)

let debounceTimer: ReturnType<typeof setTimeout> | undefined
watch(query, () => {
    // Editing the query invalidates both picks — the grid below is stale.
    pickedArtist.value = null
    pickedImage.value = null
    reset()
    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => search(query.value), 400)
})

watch(
    () => props.visible,
    (visible) => {
        if (!visible) return
        query.value = props.artistName
        pickedArtist.value = null
        pickedImage.value = null
        reset()
        search(props.artistName)
    },
    { immediate: true }
)

function pickArtist(c: MusicBrainzCandidate): void {
    pickedArtist.value = c
    pickedImage.value = null
    void load(c.mbid)
}

function pickImage(c: ArtistImageCandidate): void {
    pickedImage.value = c
}

function confirm(): void {
    const a = pickedArtist.value
    const img = pickedImage.value
    if (!a || !img) return
    emit('select', { mbid: a.mbid, name: a.name, url: img.url, previewUrl: img.thumbUrl })
    emit('update:visible', false)
}

function cancel(): void {
    emit('update:visible', false)
}

function lifeSpan(c: MusicBrainzCandidate): string {
    if (!c.lifeSpanBegin && !c.lifeSpanEnd) return ''
    return `${c.lifeSpanBegin || '?'}–${c.lifeSpanEnd || 'present'}`
}
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="emit('update:visible', $event)"
        modal
        header="Search artist image online"
        :style="{ width: '34rem' }"
    >
        <InputText
            v-model="query"
            data-test="image-search-query"
            placeholder="Search by artist name"
            class="picker-input"
        />

        <Message v-if="searchError" severity="error" :closable="false">{{ searchError }}</Message>

        <div class="results">
            <div v-if="searching" class="searching"><i class="pi pi-spin pi-spinner"></i></div>
            <ul v-else-if="results.length > 0" class="result-list">
                <li
                    v-for="c in results"
                    :key="c.mbid"
                    class="result-row"
                    :class="{ selected: c.mbid === pickedArtist?.mbid }"
                    @click="pickArtist(c)"
                >
                    <div class="result-name">
                        {{ c.name }}
                        <span v-if="c.type" class="result-type">{{ c.type }}</span>
                    </div>
                    <div class="result-meta">
                        <span v-if="c.disambiguation">{{ c.disambiguation }}</span>
                        <span v-if="lifeSpan(c)">{{ lifeSpan(c) }}</span>
                    </div>
                </li>
            </ul>
            <p v-else class="no-results">No matches</p>
        </div>

        <!-- The grid loads each thumbnail straight from the provider CDN — no
             endpoint of ours is hit to preview it, only to list the URLs. -->
        <div v-if="pickedArtist" class="image-section">
            <div class="preview-title">Choose an image for this artist:</div>
            <div v-if="loadingImages" class="searching"><i class="pi pi-spin pi-spinner"></i></div>
            <Message v-else-if="imagesError" severity="error" :closable="false">{{ imagesError }}</Message>
            <div v-else-if="candidates.length > 0" class="image-grid">
                <img
                    v-for="c in candidates"
                    :key="c.url"
                    data-test="candidate-thumb"
                    :src="c.thumbUrl"
                    loading="lazy"
                    alt="Artist image candidate"
                    class="candidate"
                    :class="{ selected: c.url === pickedImage?.url }"
                    @click="pickImage(c)"
                />
            </div>
            <p v-else class="no-results">No images available for this artist. Try another match.</p>
        </div>

        <template #footer>
            <input
                v-if="allowUpload"
                ref="fileInput"
                type="file"
                accept="image/*"
                class="hidden-file"
                data-test="image-upload-input"
                @change="onFileChange"
            />
            <Button
                v-if="allowUpload"
                label="Upload file…"
                icon="pi pi-upload"
                text
                data-test="image-upload"
                @click="triggerUpload"
            />
            <span class="footer-spacer"></span>
            <Button label="Cancel" text data-test="image-search-cancel" @click="cancel" />
            <!-- "Use this image", not "Save": the write happens on the editor's
                 own Save, so this only stages the pick. -->
            <Button
                label="Use this image"
                data-test="image-search-save"
                :disabled="!pickedImage"
                @click="confirm"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.picker-input {
    width: 100%;
}
.results {
    min-height: 6rem;
    max-height: 12rem;
    overflow-y: auto;
    margin: 0.75rem 0;
}
.searching {
    display: flex;
    justify-content: center;
    padding: 2rem;
    color: var(--app-text-secondary);
}
.result-list {
    list-style: none;
    margin: 0;
    padding: 0;
}
.result-row {
    padding: 0.5rem 0.75rem;
    border-radius: 6px;
    cursor: pointer;
}
.result-row:hover {
    background: var(--app-bg-subtle, #f3f4f6);
}
.result-row.selected {
    background: var(--app-accent-soft);
}
.result-name {
    font-weight: 500;
    display: flex;
    align-items: center;
    gap: 0.5rem;
}
.result-type {
    font-size: 0.75rem;
    font-weight: 400;
    color: var(--app-text-secondary);
    border: 1px solid var(--app-border, #e5e7eb);
    border-radius: 4px;
    padding: 0 0.35rem;
}
.result-meta {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    display: flex;
    gap: 0.5rem;
}
.no-results {
    text-align: center;
    color: var(--app-text-secondary);
    padding: 1.5rem 0;
}
.image-section {
    border-top: 1px solid var(--app-border);
    padding-top: 0.75rem;
}
.preview-title {
    align-self: flex-start;
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--app-text-secondary);
    margin-bottom: 0.5rem;
}
.image-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(96px, 1fr));
    gap: 0.5rem;
    max-height: 16rem;
    overflow-y: auto;
}
.candidate {
    width: 100%;
    aspect-ratio: 1;
    object-fit: cover;
    border-radius: var(--app-radius);
    border: 2px solid transparent;
    cursor: pointer;
}
.candidate:hover {
    border-color: var(--app-border);
}
.candidate.selected {
    border-color: var(--app-accent);
}
.hidden-file {
    display: none;
}
/* Push Cancel/Use to the right, leaving the upload button on the left. */
.footer-spacer {
    flex: 1;
}
</style>
