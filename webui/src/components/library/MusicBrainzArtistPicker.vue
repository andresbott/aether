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
    (e: 'select', mbid: string): void
}>()

const { results, loading: searching, error: searchError, search } = useMusicBrainzSearch()
const query = ref('')
const paste = ref('')

const pasteValid = computed(() => MBID_RE.test(paste.value.trim()))

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
    if (query.value === lastSearched) return
    scheduleSearch()
})

watch(
    () => props.visible,
    (visible) => {
        if (!visible) return
        query.value = props.artistName
        paste.value = ''
        runSearch(query.value)
    },
    { immediate: true }
)

function choose(mbid: string) {
    emit('select', mbid)
    emit('update:visible', false)
}
function applyPaste() {
    if (pasteValid.value) choose(paste.value.trim())
}
function clearMatch() {
    choose('')
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

        <InputText v-model="query" placeholder="Search MusicBrainz for an artist" class="picker-input" />

        <Message v-if="searchError" severity="error" :closable="false">{{ searchError }}</Message>

        <div class="results">
            <div v-if="searching" class="searching"><i class="pi pi-spin pi-spinner"></i></div>
            <ul v-else-if="results.length > 0" class="result-list">
                <li
                    v-for="c in results"
                    :key="c.mbid"
                    class="result-row"
                    :class="{ selected: c.mbid === currentMbid }"
                    @click="choose(c.mbid)"
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

        <div class="paste-row">
            <InputText
                v-model="paste"
                data-test="mbid-paste"
                placeholder="…or paste an MBID"
                class="picker-input"
            />
            <Button
                label="Apply"
                data-test="mbid-apply"
                :disabled="!pasteValid"
                @click="applyPaste"
            />
        </div>
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
.result-row.selected { background: var(--app-primary-subtle, #e0e7ff); }
.result-name { font-weight: 500; display: flex; align-items: center; gap: 0.5rem; }
.result-type { font-size: 0.75rem; font-weight: 400; color: var(--app-text-secondary); border: 1px solid var(--app-border, #e5e7eb); border-radius: 4px; padding: 0 0.35rem; }
.result-meta { font-size: 0.8rem; color: var(--app-text-secondary); display: flex; gap: 0.5rem; }
.no-results { text-align: center; color: var(--app-text-secondary); padding: 1.5rem 0; }
.paste-row { display: flex; gap: 0.5rem; align-items: center; }
</style>
