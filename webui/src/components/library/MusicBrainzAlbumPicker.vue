<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Checkbox from 'primevue/checkbox'
import Message from 'primevue/message'
import { useMusicBrainzReleaseSearch } from '@/composables/useMusicBrainzReleaseSearch'
import { getReleaseGroupGenres } from '@/lib/api/Artists'
import type {
    MusicBrainzReleaseCandidate,
    AlbumMatchPayload,
    ReleaseArtistCredit
} from '@/types/artists'

const props = defineProps<{
    visible: boolean
    albumName: string
    currentReleaseMbid: string
    currentReleaseGroupMbid: string
    currentYear: number
    currentAlbumArtists: ReleaseArtistCredit[]
    currentGenres: string[]
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    // A confirmed selection: only the fields the user left checked in the
    // preview. Empty-string IDs (the clear-match path) wipe the match.
    (e: 'select', payload: AlbumMatchPayload): void
}>()

const { results, loading: searching, error: searchError, search } = useMusicBrainzReleaseSearch()

// Unlike the artist picker, this is a search-only helper: manual entry of an
// individual ID is done in the edit panel's text fields, so there is no
// paste-a-bare-ID shortcut here (a single UUID could not disambiguate between
// the release and release-group fields).
const query = ref('')
const selected = ref<MusicBrainzReleaseCandidate | null>(null)

const selectedMbid = computed(() => selected.value?.releaseMbid ?? '')

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

// --- Preview ("Will apply") --------------------------------------------------
// One row per fillable field, showing current → new with a checkbox. Only
// checked rows end up in the emitted payload.

type FieldKey = 'album' | 'year' | 'albumArtists' | 'genres' | 'mbReleaseId' | 'mbReleaseGroupId'

interface PreviewRow {
    key: FieldKey
    label: string
    current: string
    next: string
    unchanged: boolean
    mono: boolean
}

// yearOf extracts the leading YYYY of a MusicBrainz date ("1997-05-21" → 1997).
function yearOf(date: string): number | null {
    const m = /^(\d{4})/.exec(date)
    return m ? Number(m[1]) : null
}

function creditNames(credits: ReleaseArtistCredit[]): string {
    return credits.map((c) => c.name).join(', ')
}

const previewRows = computed<PreviewRow[]>(() => {
    const c = selected.value
    if (!c) return []
    const rows: PreviewRow[] = []
    const push = (key: FieldKey, label: string, current: string, next: string, mono = false) => {
        rows.push({ key, label, current, next, unchanged: current === next, mono })
    }
    push('album', 'Album', props.albumName, c.title)
    const year = yearOf(c.date)
    if (year !== null) {
        push('year', 'Year', props.currentYear ? String(props.currentYear) : '', String(year))
    }
    if (c.artists.length > 0) {
        push(
            'albumArtists',
            'Album artists',
            creditNames(props.currentAlbumArtists),
            creditNames(c.artists)
        )
    }
    if (selectedGenres.value.length > 0) {
        push('genres', 'Genres', props.currentGenres.join(', '), selectedGenres.value.join(', '))
    }
    push('mbReleaseId', 'Release ID', props.currentReleaseMbid, c.releaseMbid, true)
    push(
        'mbReleaseGroupId',
        'Release-group ID',
        props.currentReleaseGroupMbid,
        c.releaseGroupMbid,
        true
    )
    return rows
})

const checked = ref<Record<FieldKey, boolean>>({
    album: true,
    year: true,
    albumArtists: true,
    genres: true,
    mbReleaseId: true,
    mbReleaseGroupId: true
})

// Picking a (different) candidate re-arms every checkbox.
watch(selected, () => {
    checked.value = {
        album: true,
        year: true,
        albumArtists: true,
        genres: true,
        mbReleaseId: true,
        mbReleaseGroupId: true
    }
})

// The release *search* response carries no genres; they come from a separate
// release-group lookup fired when a candidate is picked. Empty (also on
// lookup failure) hides the preview row.
const selectedGenres = ref<string[]>([])
watch(selected, async (c) => {
    selectedGenres.value = []
    if (!c || !c.releaseGroupMbid) return
    const mbid = c.releaseGroupMbid
    try {
        const genres = await getReleaseGroupGenres(mbid)
        // Guard against a stale response after the user picked another row.
        if (selected.value?.releaseGroupMbid === mbid) selectedGenres.value = genres
    } catch {
        // Genres are a nice-to-have; a failed lookup just hides the row.
    }
})

const anyChecked = computed(() => previewRows.value.some((r) => checked.value[r.key]))
const canConfirm = computed(() => selected.value !== null && anyChecked.value)

function pick(c: MusicBrainzReleaseCandidate) {
    selected.value = c
}
function confirm() {
    const c = selected.value
    if (!c) return
    const payload: AlbumMatchPayload = {}
    for (const row of previewRows.value) {
        if (!checked.value[row.key]) continue
        switch (row.key) {
            case 'album':
                payload.album = c.title
                break
            case 'year': {
                const y = yearOf(c.date)
                if (y !== null) payload.year = y
                break
            }
            case 'albumArtists':
                payload.albumArtists = c.artists.map((a) => ({ name: a.name, mbid: a.mbid }))
                break
            case 'genres':
                payload.genres = [...selectedGenres.value]
                break
            case 'mbReleaseId':
                payload.mbReleaseId = c.releaseMbid
                break
            case 'mbReleaseGroupId':
                payload.mbReleaseGroupId = c.releaseGroupMbid
                break
        }
    }
    emit('select', payload)
    emit('update:visible', false)
}
function cancel() {
    emit('update:visible', false)
}
function clearMatch() {
    emit('select', { mbReleaseId: '', mbReleaseGroupId: '' })
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
                <a
                    :href="`https://musicbrainz.org/release/${currentReleaseMbid}`"
                    target="_blank"
                    rel="noopener"
                    >{{ currentReleaseMbid }}</a
                >
            </div>
            <div v-if="currentReleaseGroupMbid" class="current-match-line">
                <span class="current-match-label">Release group:</span>
                <a
                    :href="`https://musicbrainz.org/release-group/${currentReleaseGroupMbid}`"
                    target="_blank"
                    rel="noopener"
                    >{{ currentReleaseGroupMbid }}</a
                >
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

        <div v-if="previewRows.length > 0" class="preview" data-test="album-preview">
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
.current-match {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 0.25rem 0.5rem;
    padding: 0.5rem 0.75rem;
    margin-bottom: 0.75rem;
    background: var(--app-bg-subtle, #f3f4f6);
    border-radius: 6px;
    font-size: 0.9rem;
}
.current-match-line {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
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
.result-artist {
    font-size: 0.8rem;
    font-weight: 400;
    color: var(--app-text-secondary);
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
