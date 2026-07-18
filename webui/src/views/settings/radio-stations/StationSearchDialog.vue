<script setup lang="ts">
import { ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { useRadioBrowserSearch } from '@/composables/useRadioBrowserSearch'
import type { RadioBrowserStation } from '@/types/radiobrowser'

const props = defineProps<{
    visible: boolean
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    (e: 'select', station: RadioBrowserStation): void
}>()

const { results, loading: searching, error: searchError, search } = useRadioBrowserSearch()

const query = ref('')
// The staged station, applied only on OK.
const selected = ref<RadioBrowserStation | null>(null)

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

// Reset to a clean slate every time the dialog opens.
watch(
    () => props.visible,
    (visible) => {
        if (!visible) return
        query.value = ''
        lastSearched = ''
        selected.value = null
        results.value = []
        searchError.value = null
    }
)

function pick(s: RadioBrowserStation) {
    selected.value = s
}
function confirm() {
    if (!selected.value) return
    emit('select', selected.value)
    emit('update:visible', false)
}
function cancel() {
    emit('update:visible', false)
}

function meta(s: RadioBrowserStation): string {
    const parts: string[] = []
    if (s.country) parts.push(s.country)
    if (s.bitrate) parts.push(`${s.bitrate} kbps`)
    if (s.codec) parts.push(s.codec)
    return parts.join(' · ')
}

function hideBrokenImage(e: Event) {
    ;(e.target as HTMLImageElement).style.display = 'none'
}
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="emit('update:visible', $event)"
        modal
        header="Search online radio stations"
        :style="{ width: '38rem' }"
    >
        <InputText
            v-model="query"
            data-test="rb-query"
            placeholder="Search by station name (e.g. BBC, Jazz FM)"
            class="picker-input"
            autofocus
        />

        <Message v-if="searchError" severity="error" :closable="false">{{ searchError }}</Message>

        <div class="results">
            <div v-if="searching" class="searching"><i class="pi pi-spin pi-spinner"></i></div>
            <ul v-else-if="results.length > 0" class="result-list">
                <li
                    v-for="s in results"
                    :key="s.uuid || s.streamUrl"
                    class="result-row"
                    :class="{ selected: s === selected }"
                    @click="pick(s)"
                >
                    <img
                        v-if="s.favicon"
                        :src="s.favicon"
                        class="result-favicon"
                        alt=""
                        @error="hideBrokenImage"
                    />
                    <div class="result-body">
                        <div class="result-name">{{ s.name }}</div>
                        <div class="result-meta">
                            <span v-if="meta(s)">{{ meta(s) }}</span>
                            <span v-if="s.tags" class="result-tags">{{ s.tags }}</span>
                        </div>
                    </div>
                </li>
            </ul>
            <p v-else-if="query.trim().length >= 2" class="no-results">No matches</p>
            <p v-else class="hint">Type a station name to search radio-browser.info.</p>
        </div>

        <template #footer>
            <Button label="Cancel" text @click="cancel" />
            <Button label="Add" data-test="rb-confirm" :disabled="!selected" @click="confirm" />
        </template>
    </Dialog>
</template>

<style scoped>
.picker-input {
    width: 100%;
}
.results {
    min-height: 6rem;
    max-height: 22rem;
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
    display: flex;
    align-items: center;
    gap: 0.75rem;
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
.result-favicon {
    width: 2rem;
    height: 2rem;
    border-radius: 4px;
    object-fit: cover;
    flex-shrink: 0;
    background: var(--app-bg-subtle, #f3f4f6);
}
.result-body {
    min-width: 0;
}
.result-name {
    font-weight: 500;
}
.result-meta {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    display: flex;
    gap: 0.75rem;
    min-width: 0;
}
.result-tags {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.no-results,
.hint {
    text-align: center;
    color: var(--app-text-secondary);
    padding: 1.5rem 0;
}
</style>
