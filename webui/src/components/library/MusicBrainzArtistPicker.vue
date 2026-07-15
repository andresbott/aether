<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { useMusicBrainzSearch } from '@/composables/useMusicBrainzSearch'
import type { MusicBrainzCandidate } from '@/types/artists'

const MBID_RE = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/

const props = defineProps<{
    visible: boolean
    artistName: string
    currentMbid: string
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    // A confirmed selection: the MBID plus the artist name to write. `name` is
    // omitted when only an ID was entered (a bare MBID carries no name), so the
    // caller leaves the existing artist name untouched.
    (e: 'select', mbid: string, name?: string): void
}>()

const { results, loading: searching, error: searchError, search } = useMusicBrainzSearch()

// One combined box: an artist name triggers a search, a pasted MBID is detected
// and staged directly.
const query = ref('')
// The staged choice, applied only on OK. `name: ''` means "ID only, keep name".
const selected = ref<{ name: string; mbid: string } | null>(null)

const isMbidQuery = computed(() => MBID_RE.test(query.value.trim()))
const selectedMbid = computed(() => selected.value?.mbid ?? '')
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
    if (isMbidQuery.value) {
        // A pasted/typed MBID is a complete choice on its own (no name).
        selected.value = { name: '', mbid: query.value.trim() }
        return
    }
    // Editing the name query invalidates any previously picked result.
    selected.value = null
    if (query.value === lastSearched) return
    scheduleSearch()
})

watch(
    () => props.visible,
    (visible) => {
        if (!visible) return
        query.value = props.artistName
        selected.value = null
        runSearch(query.value)
    },
    { immediate: true }
)

function pick(c: MusicBrainzCandidate) {
    selected.value = { name: c.name, mbid: c.mbid }
}
function confirm() {
    if (!selected.value) return
    emit('select', selected.value.mbid, selected.value.name || undefined)
    emit('update:visible', false)
}
function cancel() {
    emit('update:visible', false)
}
function clearMatch() {
    emit('select', '')
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
        header="Match MusicBrainz artist"
        :style="{ width: '34rem' }"
    >
        <div v-if="currentMbid" class="current-match">
            <span class="current-match-label">Linked:</span>
            <a :href="`https://musicbrainz.org/artist/${currentMbid}`" target="_blank" rel="noopener">{{ currentMbid }}</a>
            <button class="clear-btn" title="Clear match" @click="clearMatch">
                <i class="pi pi-times"></i>
            </button>
        </div>

        <InputText
            v-model="query"
            data-test="mbid-query"
            placeholder="Search by artist name, or paste a MusicBrainz ID"
            class="picker-input"
        />

        <Message v-if="searchError" severity="error" :closable="false">{{ searchError }}</Message>

        <Message v-if="isMbidQuery" severity="info" :closable="false">
            MusicBrainz ID detected — click OK to apply it (the artist name is left unchanged).
        </Message>

        <div v-else class="results">
            <div v-if="searching" class="searching"><i class="pi pi-spin pi-spinner"></i></div>
            <ul v-else-if="results.length > 0" class="result-list">
                <li
                    v-for="c in results"
                    :key="c.mbid"
                    class="result-row"
                    :class="{ selected: c.mbid === selectedMbid }"
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

        <template #footer>
            <Button label="Cancel" text @click="cancel" />
            <Button
                label="OK"
                data-test="mbid-confirm"
                :disabled="!canConfirm"
                @click="confirm"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.current-match { display: flex; align-items: center; gap: 0.5rem; padding: 0.5rem 0.75rem; margin-bottom: 0.75rem; background: var(--app-bg-subtle, #f3f4f6); border-radius: 6px; font-size: 0.9rem; }
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
.result-type { font-size: 0.75rem; font-weight: 400; color: var(--app-text-secondary); border: 1px solid var(--app-border, #e5e7eb); border-radius: 4px; padding: 0 0.35rem; }
.result-meta { font-size: 0.8rem; color: var(--app-text-secondary); display: flex; gap: 0.5rem; }
.no-results { text-align: center; color: var(--app-text-secondary); padding: 1.5rem 0; }
</style>
