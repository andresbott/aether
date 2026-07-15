<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { useMusicBrainzReleaseSearch } from '@/composables/useMusicBrainzReleaseSearch'
import type { MusicBrainzReleaseCandidate } from '@/types/artists'

const props = defineProps<{
    visible: boolean
    albumName: string
    currentReleaseMbid: string
    currentReleaseGroupMbid: string
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    // A confirmed selection: the release MBID, its parent release-group MBID,
    // and the release title to write into the album name field.
    (e: 'select', releaseMbid: string, releaseGroupMbid: string, title?: string): void
}>()

const { results, loading: searching, error: searchError, search } = useMusicBrainzReleaseSearch()

// Unlike the artist picker, this is a search-only helper: manual entry of an
// individual ID is done in the edit panel's text fields, so there is no
// paste-a-bare-ID shortcut here (a single UUID could not disambiguate between
// the release and release-group fields).
const query = ref('')
const selected = ref<MusicBrainzReleaseCandidate | null>(null)

const selectedMbid = computed(() => selected.value?.releaseMbid ?? '')
const canConfirm = computed(() => selected.value !== null)

let lastSearched = ''
function runSearch(q: string) {
    lastSearched = q
    search(q)
}

let debounceTimer: ReturnType<typeof setTimeout> | undefined
function scheduleSearch() {
    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => runSearch(query.value), 400)
}

watch(query, () => {
    // Editing the query invalidates any previously picked result.
    selected.value = null
    if (query.value === lastSearched) return
    scheduleSearch()
})

watch(
    () => props.visible,
    (visible) => {
        if (!visible) return
        query.value = props.albumName
        selected.value = null
        runSearch(query.value)
    },
    { immediate: true }
)

function pick(c: MusicBrainzReleaseCandidate) {
    selected.value = c
}
function confirm() {
    if (!selected.value) return
    emit('select', selected.value.releaseMbid, selected.value.releaseGroupMbid, selected.value.title)
    emit('update:visible', false)
}
function cancel() {
    emit('update:visible', false)
}
function clearMatch() {
    emit('select', '', '')
    emit('update:visible', false)
}
function releaseMeta(c: MusicBrainzReleaseCandidate): string {
    const parts: string[] = []
    if (c.date) parts.push(c.date)
    if (c.country) parts.push(c.country)
    if (c.trackCount) parts.push(`${c.trackCount} tracks`)
    return parts.join(' · ')
}
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="emit('update:visible', $event)"
        modal
        header="Match MusicBrainz album"
        :style="{ width: '34rem' }"
    >
        <div v-if="currentReleaseMbid || currentReleaseGroupMbid" class="current-match">
            <div v-if="currentReleaseMbid" class="current-match-line">
                <span class="current-match-label">Release:</span>
                <a :href="`https://musicbrainz.org/release/${currentReleaseMbid}`" target="_blank" rel="noopener">{{ currentReleaseMbid }}</a>
            </div>
            <div v-if="currentReleaseGroupMbid" class="current-match-line">
                <span class="current-match-label">Release group:</span>
                <a :href="`https://musicbrainz.org/release-group/${currentReleaseGroupMbid}`" target="_blank" rel="noopener">{{ currentReleaseGroupMbid }}</a>
            </div>
            <button class="clear-btn" title="Clear match" @click="clearMatch">
                <i class="pi pi-times"></i>
            </button>
        </div>

        <InputText
            v-model="query"
            data-test="album-mbid-query"
            placeholder="Search by album title"
            class="picker-input"
        />

        <Message v-if="searchError" severity="error" :closable="false">{{ searchError }}</Message>

        <div class="results">
            <div v-if="searching" class="searching"><i class="pi pi-spin pi-spinner"></i></div>
            <ul v-else-if="results.length > 0" class="result-list">
                <li
                    v-for="c in results"
                    :key="c.releaseMbid"
                    class="result-row"
                    :class="{ selected: c.releaseMbid === selectedMbid }"
                    @click="pick(c)"
                >
                    <div class="result-name">
                        {{ c.title }}
                        <span v-if="c.artist" class="result-artist">{{ c.artist }}</span>
                    </div>
                    <div class="result-meta">
                        <span v-if="c.disambiguation">{{ c.disambiguation }}</span>
                        <span v-if="releaseMeta(c)">{{ releaseMeta(c) }}</span>
                    </div>
                </li>
            </ul>
            <p v-else class="no-results">No matches</p>
        </div>

        <template #footer>
            <Button label="Cancel" text @click="cancel" />
            <Button
                label="OK"
                data-test="album-mbid-confirm"
                :disabled="!canConfirm"
                @click="confirm"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.current-match { display: flex; flex-wrap: wrap; align-items: center; gap: 0.25rem 0.5rem; padding: 0.5rem 0.75rem; margin-bottom: 0.75rem; background: var(--app-bg-subtle, #f3f4f6); border-radius: 6px; font-size: 0.9rem; }
.current-match-line { display: flex; align-items: center; gap: 0.5rem; width: 100%; }
.current-match-label { font-weight: 500; color: var(--app-text-secondary); }
.clear-btn { margin-left: auto; background: none; border: none; cursor: pointer; color: var(--app-text-secondary); padding: 0.25rem; }
.clear-btn:hover { color: #ef4444; }
.picker-input { width: 100%; }
.results { min-height: 6rem; max-height: 18rem; overflow-y: auto; margin: 0.75rem 0; }
.searching { display: flex; justify-content: center; padding: 2rem; color: var(--app-text-secondary); }
.result-list { list-style: none; margin: 0; padding: 0; }
.result-row { padding: 0.5rem 0.75rem; border-radius: 6px; cursor: pointer; }
.result-row:hover { background: var(--app-bg-subtle, #f3f4f6); }
.result-row.selected { background: var(--app-accent-soft); }
.result-name { font-weight: 500; display: flex; align-items: center; gap: 0.5rem; }
.result-artist { font-size: 0.8rem; font-weight: 400; color: var(--app-text-secondary); }
.result-meta { font-size: 0.8rem; color: var(--app-text-secondary); display: flex; gap: 0.5rem; }
.no-results { text-align: center; color: var(--app-text-secondary); padding: 1.5rem 0; }
</style>
