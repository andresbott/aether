<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Checkbox from 'primevue/checkbox'
import Message from 'primevue/message'
import { useMusicBrainzSearch } from '@/composables/useMusicBrainzSearch'
import type { MusicBrainzCandidate, ArtistMatchPayload } from '@/types/artists'

const MBID_RE = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/

const props = defineProps<{
    visible: boolean
    artistName: string
    currentMbid: string
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    // A confirmed selection: only the fields the user left checked in the
    // preview. An empty-string mbid (the clear-match path) wipes the match.
    (e: 'select', payload: ArtistMatchPayload): void
}>()

const { results, loading: searching, error: searchError, search } = useMusicBrainzSearch()

// One combined box: an artist name triggers a search, a pasted MBID is detected
// and staged directly.
const query = ref('')
// The staged choice, applied only on OK. `name: ''` means "ID only, keep name".
const selected = ref<{ name: string; mbid: string } | null>(null)

const isMbidQuery = computed(() => MBID_RE.test(query.value.trim()))
const selectedMbid = computed(() => selected.value?.mbid ?? '')

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

// --- Preview ("Will apply") --------------------------------------------------
// One row per fillable field, showing current → new with a checkbox. Only
// checked rows end up in the emitted payload. A bare-MBID choice carries no
// name, so it only offers the ID row.

type FieldKey = 'name' | 'mbid'

interface PreviewRow {
    key: FieldKey
    label: string
    current: string
    next: string
    unchanged: boolean
    mono: boolean
}

const previewRows = computed<PreviewRow[]>(() => {
    const s = selected.value
    if (!s) return []
    const rows: PreviewRow[] = []
    if (s.name) {
        rows.push({
            key: 'name',
            label: 'Artist',
            current: props.artistName,
            next: s.name,
            unchanged: props.artistName === s.name,
            mono: false
        })
    }
    rows.push({
        key: 'mbid',
        label: 'MusicBrainz ID',
        current: props.currentMbid,
        next: s.mbid,
        unchanged: props.currentMbid === s.mbid,
        mono: true
    })
    return rows
})

const checked = ref<Record<FieldKey, boolean>>({ name: true, mbid: true })

// Picking a (different) candidate re-arms every checkbox.
watch(selected, () => {
    checked.value = { name: true, mbid: true }
})

const anyChecked = computed(() => previewRows.value.some((r) => checked.value[r.key]))
const canConfirm = computed(() => selected.value !== null && anyChecked.value)

function pick(c: MusicBrainzCandidate) {
    selected.value = { name: c.name, mbid: c.mbid }
}
function confirm() {
    const s = selected.value
    if (!s) return
    const payload: ArtistMatchPayload = {}
    for (const row of previewRows.value) {
        if (!checked.value[row.key]) continue
        if (row.key === 'name') payload.name = s.name
        else payload.mbid = s.mbid
    }
    emit('select', payload)
    emit('update:visible', false)
}
function cancel() {
    emit('update:visible', false)
}
function clearMatch() {
    emit('select', { mbid: '' })
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
            <a
                :href="`https://musicbrainz.org/artist/${currentMbid}`"
                target="_blank"
                rel="noopener"
                >{{ currentMbid }}</a
            >
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

        <div v-if="previewRows.length > 0" class="preview" data-test="artist-preview">
            <div class="preview-title">Will apply:</div>
            <div
                v-for="row in previewRows"
                :key="row.key"
                class="preview-row"
                :data-test="`preview-${row.key}`"
            >
                <Checkbox v-model="checked[row.key]" :binary="true" :inputId="`apply-${row.key}`" />
                <label :for="`apply-${row.key}`" class="preview-label">{{ row.label }}</label>
                <span class="preview-diff" :class="{ mono: row.mono }">
                    <template v-if="row.unchanged">
                        <span class="preview-value">{{ row.next }}</span>
                        <span class="preview-unchanged">(unchanged)</span>
                    </template>
                    <template v-else>
                        <span class="preview-value old">{{ row.current || '(empty)' }}</span>
                        <i class="pi pi-arrow-right preview-arrow"></i>
                        <span class="preview-value">{{ row.next }}</span>
                    </template>
                </span>
            </div>
        </div>

        <template #footer>
            <Button label="Cancel" text @click="cancel" />
            <Button label="OK" data-test="mbid-confirm" :disabled="!canConfirm" @click="confirm" />
        </template>
    </Dialog>
</template>

<style scoped>
.current-match {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.75rem;
    margin-bottom: 0.75rem;
    background: var(--app-bg-subtle, #f3f4f6);
    border-radius: 6px;
    font-size: 0.9rem;
}
.current-match-label {
    font-weight: 500;
    color: var(--app-text-secondary);
}
.clear-btn {
    margin-left: auto;
    background: none;
    border: none;
    cursor: pointer;
    color: var(--app-text-secondary);
    padding: 0.25rem;
}
.clear-btn:hover {
    color: #ef4444;
}
.picker-input {
    width: 100%;
}
.results {
    min-height: 6rem;
    max-height: 14rem;
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
.preview {
    border-top: 1px solid var(--app-border);
    padding-top: 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
}
.preview-title {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--app-text-secondary);
    margin-bottom: 0.25rem;
}
.preview-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.85rem;
    min-width: 0;
}
.preview-label {
    flex: 0 0 8rem;
    color: var(--app-text-secondary);
    cursor: pointer;
}
.preview-diff {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    min-width: 0;
    flex: 1;
}
.preview-diff.mono .preview-value {
    font-family: var(--font-mono, monospace);
    font-size: 0.75rem;
}
.preview-value {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
}
.preview-value.old {
    color: var(--app-text-secondary);
}
.preview-arrow {
    font-size: 0.7rem;
    color: var(--app-text-secondary);
    flex: 0 0 auto;
}
.preview-unchanged {
    color: var(--app-text-secondary);
    font-size: 0.75rem;
    flex: 0 0 auto;
}
</style>
