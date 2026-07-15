<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import { useMusicBrainzSearch } from '@/composables/useMusicBrainzSearch'
import { useSetArtistMBID } from '@/composables/useArtistMbid'
import { getArtistMBID, parseArtistNumericId } from '@/lib/api/Artists'
import type { MusicBrainzCandidate } from '@/types/artists'

const props = defineProps<{
    visible: boolean
    artistId: string
    artistName: string
}>()

const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    (e: 'saved'): void
}>()

const numericId = computed(() => parseArtistNumericId(props.artistId))

const query = ref('')
const currentMbid = ref('')
const stagedMbid = ref<string | null>(null)

const { results, loading: searching, error: searchError, search } = useMusicBrainzSearch()
const setMbid = useSetArtistMBID()

const pendingMbid = computed(() => stagedMbid.value ?? currentMbid.value)
const canSave = computed(() => stagedMbid.value !== null && stagedMbid.value !== currentMbid.value)

let debounceTimer: ReturnType<typeof setTimeout> | undefined

function scheduleSearch() {
    if (debounceTimer) clearTimeout(debounceTimer)
    debounceTimer = setTimeout(() => search(query.value), 400)
}

watch(query, scheduleSearch)

watch(
    () => props.visible,
    async (visible) => {
        if (!visible) return
        query.value = props.artistName
        stagedMbid.value = null
        currentMbid.value = await getArtistMBID(numericId.value)
        search(query.value)
    },
    { immediate: true }
)

function selectCandidate(c: MusicBrainzCandidate) {
    stagedMbid.value = c.mbid
}

function clearMatch() {
    stagedMbid.value = ''
}

function onSave() {
    if (!canSave.value) return
    setMbid.mutate(
        { numericId: numericId.value, mbid: pendingMbid.value },
        {
            onSuccess: () => {
                emit('saved')
                emit('update:visible', false)
            }
        }
    )
}

function onCancel() {
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
        <Message severity="warn" :closable="false" class="deprecation-note">
            Deprecated — prefer the Metadata Editor (Settings → Metadata Editor),
            which writes the MusicBrainz ID into the file tags.
        </Message>

        <div v-if="pendingMbid" class="current-match">
            <span class="current-match-label">Linked:</span>
            <a
                :href="`https://musicbrainz.org/artist/${pendingMbid}`"
                target="_blank"
                rel="noopener"
            >{{ pendingMbid }}</a>
            <button class="clear-btn" title="Clear match" @click="clearMatch">
                <i class="pi pi-times"></i>
            </button>
        </div>

        <InputText
            v-model="query"
            placeholder="Search MusicBrainz for an artist"
            class="search-input"
        />

        <Message v-if="searchError" severity="error" :closable="false">{{ searchError }}</Message>

        <div class="results">
            <div v-if="searching" class="searching">
                <i class="pi pi-spin pi-spinner"></i>
            </div>
            <ul v-else-if="results.length > 0" class="result-list">
                <li
                    v-for="c in results"
                    :key="c.mbid"
                    class="result-row"
                    :class="{ selected: c.mbid === pendingMbid }"
                    @click="selectCandidate(c)"
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
            <Button label="Cancel" text @click="onCancel" />
            <Button
                label="Save"
                :loading="setMbid.isPending.value"
                :disabled="!canSave"
                @click="onSave"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.deprecation-note {
    margin-bottom: 0.75rem;
}
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
.search-input {
    width: 100%;
    margin-bottom: 0.75rem;
}
.results {
    min-height: 8rem;
    max-height: 20rem;
    overflow-y: auto;
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
    padding: 2rem 0;
}
</style>
