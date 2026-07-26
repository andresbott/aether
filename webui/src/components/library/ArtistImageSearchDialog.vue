<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { useMusicBrainzSearch } from '@/composables/useMusicBrainzSearch'
import {
    artistImagePreviewUrl,
    parseArtistNumericId,
    setArtistImageFromSearch
} from '@/lib/api/Artists'
import { apiErrorMessage } from '@/lib/apiError'
import type { MusicBrainzCandidate } from '@/types/artists'

const props = defineProps<{
    visible: boolean
    artistId: string
    artistName: string
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    // The image was stored; the caller should bust its cover URL and refetch.
    (e: 'saved'): void
}>()

// Same MusicBrainz artist search the MBID picker and the auto-fetch job use — the
// image comes from the same provider chain, driven by the user's pick instead of
// the artist's stored match.
const { results, loading: searching, error: searchError, search } = useMusicBrainzSearch()

const query = ref('')
const picked = ref<MusicBrainzCandidate | null>(null)
// Set when the preview <img> fails: the providers hold no image for this
// candidate, so there is nothing to save.
const previewFailed = ref(false)
const saveError = ref<string | null>(null)
const saving = ref(false)

const previewUrl = computed(() =>
    picked.value ? artistImagePreviewUrl(picked.value.mbid) : null
)
const canSave = computed(() => picked.value !== null && !previewFailed.value && !saving.value)

let debounceTimer: ReturnType<typeof setTimeout> | undefined
watch(query, () => {
    // Editing the query invalidates the pick — the preview below it is stale.
    picked.value = null
    previewFailed.value = false
    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => search(query.value), 400)
})

watch(
    () => props.visible,
    (visible) => {
        if (!visible) return
        query.value = props.artistName
        picked.value = null
        previewFailed.value = false
        saveError.value = null
        search(props.artistName)
    },
    { immediate: true }
)

function pick(c: MusicBrainzCandidate): void {
    picked.value = c
    previewFailed.value = false
    saveError.value = null
}

async function save(): Promise<void> {
    if (!picked.value || previewFailed.value) return
    saving.value = true
    saveError.value = null
    try {
        await setArtistImageFromSearch(parseArtistNumericId(props.artistId), picked.value.mbid)
        emit('saved')
        emit('update:visible', false)
    } catch (err) {
        saveError.value = apiErrorMessage(err)
    } finally {
        saving.value = false
    }
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
        <Message v-if="saveError" severity="error" :closable="false">{{ saveError }}</Message>

        <div class="results">
            <div v-if="searching" class="searching"><i class="pi pi-spin pi-spinner"></i></div>
            <ul v-else-if="results.length > 0" class="result-list">
                <li
                    v-for="c in results"
                    :key="c.mbid"
                    class="result-row"
                    :class="{ selected: c.mbid === picked?.mbid }"
                    @click="pick(c)"
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

        <!-- The browser does the fetching: `src` hits the preview endpoint, which
             runs the provider chain but stores nothing. A load error means the
             providers have no image for this candidate. -->
        <div v-if="previewUrl" class="image-preview">
            <div class="preview-title">Image for this artist:</div>
            <img
                :src="previewUrl"
                alt="Artist image preview"
                @error="previewFailed = true"
                @load="previewFailed = false"
            />
            <p v-if="previewFailed" class="preview-error">
                No image available for this artist. Try another match.
            </p>
        </div>

        <template #footer>
            <Button label="Cancel" text data-test="image-search-cancel" @click="cancel" />
            <Button
                label="Save"
                data-test="image-search-save"
                :disabled="!canSave"
                :loading="saving"
                @click="save"
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
.image-preview {
    border-top: 1px solid var(--app-border);
    padding-top: 0.75rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
}
.preview-title {
    align-self: flex-start;
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--app-text-secondary);
}
.image-preview img {
    max-width: 200px;
    max-height: 200px;
    border-radius: var(--app-radius);
    object-fit: cover;
}
.preview-error {
    margin: 0;
    color: #ef4444;
    font-size: 0.85rem;
}
</style>
