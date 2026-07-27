<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Dropdown from 'primevue/dropdown'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import type {
    AlbumAssignment,
    AlbumIdentifyPick,
    AlbumOption,
    Track
} from '@/types/metadata'

// Sentinel slot values for the per-row re-point dropdown: keep the server's
// proposal, or drop the position entirely (album fields only).
const SLOT_KEEP = -1
const SLOT_CLEAR = 0

// Composite slot identity: a position is a (disc, track) pair, not a bare track number.
function slotKey(disc: number, track: number): string {
    return `${disc}-${track}`
}

const props = defineProps<{
    visible: boolean
    options: AlbumOption[]
    tracks: Track[]
    pathErrors: Array<{ path: string; error: string }>
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    (e: 'apply', picks: AlbumIdentifyPick[]): void
}>()

// Per-song review state. `slot` is the tracklist position the user settled on:
// SLOT_KEEP defers to the chosen album's own assignment, SLOT_CLEAR stages the
// album fields with no position, any other value is a slotKey string (disc-track).
interface RowState {
    included: boolean
    slot: number | string
}

const selectedMbid = ref('')
const rows = ref(new Map<string, RowState>())

const selectedOption = computed<AlbumOption | undefined>(() =>
    props.options.find((o) => o.release_mbid === selectedMbid.value)
)

// Switching albums invalidates every manual re-point: the positions belonged to
// the previous tracklist. Reset the rows rather than carry meaningless slots.
function resetRows() {
    const next = new Map<string, RowState>()
    for (const t of props.tracks) {
        next.set(t.path, { included: true, slot: SLOT_KEEP })
    }
    rows.value = next
}

watch(
    () => props.options,
    (options) => {
        selectedMbid.value = options[0]?.release_mbid ?? ''
        resetRows()
    },
    { immediate: true }
)

watch(selectedMbid, resetRows)

const trackByPath = computed(() => {
    const map = new Map<string, Track>()
    for (const t of props.tracks) map.set(t.path, t)
    return map
})

const assignmentByPath = computed(() => {
    const map = new Map<string, AlbumAssignment>()
    for (const a of selectedOption.value?.assignments ?? []) map.set(a.path, a)
    return map
})

const slotByKey = computed(() => {
    const map = new Map<string, { disc_number: number; track_number: number; title: string; recording_mbid: string }>()
    for (const s of selectedOption.value?.tracks ?? []) {
        map.set(slotKey(s.disc_number, s.track_number), {
            disc_number: s.disc_number,
            track_number: s.track_number,
            title: s.title,
            recording_mbid: s.recording_mbid
        })
    }
    return map
})

function rowState(path: string): RowState {
    return rows.value.get(path) ?? { included: true, slot: SLOT_KEEP }
}

// resolved is what the row actually stages: the album's assignment, the user's
// re-point, or nothing.
function resolved(path: string): AlbumAssignment | null {
    const state = rowState(path)
    const proposed = assignmentByPath.value.get(path) ?? null
    if (state.slot === SLOT_KEEP) {
        if (!proposed || proposed.source === 'none') return null
        return proposed
    }
    if (state.slot === SLOT_CLEAR) return null
    // state.slot is a slotKey string (disc-track).
    const slot = slotByKey.value.get(state.slot as string)
    if (!slot) return null
    return {
        path,
        // A hand-picked position is an assertion by the user, not an inference.
        source: 'fingerprint',
        title: slot.title,
        recording_mbid: slot.recording_mbid,
        artists: proposed?.artists ?? [],
        disc_number: slot.disc_number,
        track_number: slot.track_number,
        score: 0
    }
}

function badge(path: string): string {
    const state = rowState(path)
    if (state.slot !== SLOT_KEEP) return state.slot === SLOT_CLEAR ? 'none' : 'chosen'
    return assignmentByPath.value.get(path)?.source ?? 'none'
}

function rowError(path: string): string {
    return assignmentByPath.value.get(path)?.error ?? ''
}

function currentTitle(path: string): string {
    const t = trackByPath.value.get(path)
    return t?.title || t?.name || path
}

// Rows in tracklist order, then the unplaced ones — the order the album reads
// in, not the order the file system happened to list.
const orderedPaths = computed(() => {
    const paths = props.tracks.map((t) => t.path)
    return [...paths].sort((a, b) => {
        const ra = resolved(a)
        const rb = resolved(b)
        // Sort placed rows before unplaced, then by disc and track separately (no magic multiplier).
        if (!ra && !rb) return a.localeCompare(b)
        if (!ra) return 1
        if (!rb) return -1
        if (ra.disc_number !== rb.disc_number) return ra.disc_number - rb.disc_number
        if (ra.track_number !== rb.track_number) return ra.track_number - rb.track_number
        return a.localeCompare(b)
    })
})

const albumChoices = computed(() =>
    props.options.map((o) => ({
        value: o.release_mbid,
        label: albumLabel(o),
        detail: albumDetail(o)
    }))
)

function albumLabel(o: AlbumOption): string {
    const artist = o.artists.map((a) => a.name).join(', ')
    const year = o.year > 0 ? ` (${o.year})` : ''
    return artist ? `${o.album} — ${artist}${year}` : `${o.album}${year}`
}

function albumDetail(o: AlbumOption): string {
    const parts = [`${o.matched_count} of ${props.tracks.length} songs matched`]
    if (o.enriched) {
        parts.push(`${o.track_count} track${o.track_count === 1 ? '' : 's'}`)
        if (o.disc_count > 1) parts.push(`${o.disc_count} discs`)
    } else {
        parts.push('track list unavailable')
    }
    return parts.join(' · ')
}

const selectedDetail = computed(() =>
    selectedOption.value ? albumDetail(selectedOption.value) : ''
)

const includedPaths = computed(() =>
    props.tracks.map((t) => t.path).filter((p) => rowState(p).included)
)

// Positions already claimed by INCLUDED rows, so a re-point dropdown only offers free slots.
// Unchecking a row frees its position.
const takenPositions = computed(() => {
    const taken = new Set<string>()
    for (const path of includedPaths.value) {
        const r = resolved(path)
        if (r && r.track_number > 0) taken.add(slotKey(r.disc_number, r.track_number))
    }
    return taken
})

function slotChoices(path: string): Array<{ value: number | string; label: string }> {
    const mine = resolved(path)
    const mineKey = mine ? slotKey(mine.disc_number, mine.track_number) : null
    const isMultiDisc = (selectedOption.value?.disc_count ?? 0) > 1
    const choices: Array<{ value: number | string; label: string }> = [
        { value: SLOT_KEEP, label: 'Keep proposed' },
        { value: SLOT_CLEAR, label: 'No position' }
    ]
    for (const s of selectedOption.value?.tracks ?? []) {
        const key = slotKey(s.disc_number, s.track_number)
        if (takenPositions.value.has(key) && mineKey !== key) {
            continue
        }
        const label = isMultiDisc
            ? `${s.disc_number}-${s.track_number}. ${s.title}`
            : `${s.track_number}. ${s.title}`
        choices.push({ value: key, label })
    }
    return choices
}

// Two included songs on one position (same disc AND track number) would write
// the same track number twice; the server's gap-fill avoids it, but a stale
// option or a manual pick could still collide.
const conflictingPositions = computed(() => {
    const seen = new Map<string, number>()
    for (const path of includedPaths.value) {
        const r = resolved(path)
        if (!r || r.track_number <= 0) continue
        const key = slotKey(r.disc_number, r.track_number)
        seen.set(key, (seen.get(key) ?? 0) + 1)
    }
    return [...seen.entries()].filter(([, n]) => n > 1).map(([key]) => key)
})

function isConflicting(path: string): boolean {
    const r = resolved(path)
    if (!r) return false
    return conflictingPositions.value.includes(slotKey(r.disc_number, r.track_number))
}

const canApply = computed(
    () =>
        selectedOption.value !== undefined &&
        includedPaths.value.length > 0 &&
        conflictingPositions.value.length === 0
)

function apply() {
    const option = selectedOption.value
    if (!option) return
    const picks: AlbumIdentifyPick[] = includedPaths.value.map((path) => ({
        path,
        option,
        assignment: resolved(path)
    }))
    emit('apply', picks)
}
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="(v) => emit('update:visible', v)"
        header="Identify album"
        modal
        :style="{ width: '72rem', maxWidth: '95vw' }"
    >
        <div
            v-if="pathErrors.length > 0"
            class="album-path-errors"
            data-test="album-path-errors"
        >
            <p class="path-errors-header">Some files could not be fingerprinted:</p>
            <ul class="path-errors-list">
                <li v-for="e in pathErrors" :key="e.path" class="path-error-item">
                    <span class="path-error-path">{{ e.path }}</span>
                    <span class="path-error-message">{{ e.error }}</span>
                </li>
            </ul>
        </div>

        <div v-if="options.length === 0 && pathErrors.length === 0" class="album-empty" data-test="album-empty">
            None of these songs matched a known release.
        </div>

        <div v-else-if="options.length === 0 && pathErrors.length > 0" class="album-empty" data-test="album-empty">
            No songs matched a known release.
        </div>

        <template v-else>
            <div class="album-pick">
                <label for="album-select">Album</label>
                <Dropdown
                    inputId="album-select"
                    data-test="album-select"
                    class="album-select"
                    :modelValue="selectedMbid"
                    @update:modelValue="(v: string) => (selectedMbid = v)"
                    :options="albumChoices"
                    optionLabel="label"
                    optionValue="value"
                />
                <small class="album-detail">{{ selectedDetail }}</small>
            </div>

            <p v-if="conflictingPositions.length > 0" class="album-conflict" data-test="album-conflict">
                Two songs are on the same track position. Change one before staging.
            </p>

            <div class="album-rows">
                <div
                    v-for="path in orderedPaths"
                    :key="path"
                    class="album-row"
                    :class="{ conflicting: isConflicting(path) }"
                    :data-test="`album-row-${path}`"
                >
                    <Checkbox
                        :modelValue="rowState(path).included"
                        @update:modelValue="(v: boolean) => (rowState(path).included = v)"
                        :binary="true"
                        :inputId="`album-include-${path}`"
                    />
                    <label :for="`album-include-${path}`" class="row-titles">
                        <span class="row-proposed">
                            {{ resolved(path)?.title || '(no title change)' }}
                        </span>
                        <span class="row-current">current: {{ currentTitle(path) }}</span>
                        <small v-if="rowError(path)" class="row-error">{{ rowError(path) }}</small>
                    </label>
                    <span class="row-position">
                        <template v-if="resolved(path)">
                            {{ resolved(path)!.disc_number }}-{{ resolved(path)!.track_number }}
                        </template>
                        <template v-else>—</template>
                    </span>
                    <span
                        class="row-badge"
                        :class="`badge-${badge(path)}`"
                        :data-test="`album-badge-${path}`"
                    >
                        {{ badge(path) }}
                    </span>
                    <Dropdown
                        class="row-slot"
                        :data-test="`album-slot-${path}`"
                        :modelValue="rowState(path).slot"
                        @update:modelValue="(v: number | string) => (rowState(path).slot = v)"
                        :options="slotChoices(path)"
                        optionLabel="label"
                        optionValue="value"
                    />
                </div>
            </div>
        </template>

        <template #footer>
            <Button label="Cancel" text @click="emit('update:visible', false)" />
            <Button
                v-if="options.length > 0"
                :label="`Stage ${includedPaths.length} song${includedPaths.length === 1 ? '' : 's'}`"
                icon="pi pi-check"
                data-test="album-apply"
                :disabled="!canApply"
                @click="apply"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.album-empty {
    padding: 1rem 0;
    color: var(--app-text-secondary);
}
.album-path-errors {
    margin-bottom: 1rem;
    padding: 0.75rem;
    border: 1px solid var(--p-red-300, #fca5a5);
    border-radius: 6px;
    background: var(--p-red-50, #fef2f2);
}
.path-errors-header {
    margin: 0 0 0.5rem;
    font-weight: 600;
    color: var(--p-red-800, #991b1b);
}
.path-errors-list {
    margin: 0;
    padding-left: 1.25rem;
    list-style: disc;
}
.path-error-item {
    margin-bottom: 0.25rem;
    font-size: 0.875rem;
}
.path-error-path {
    font-family: var(--p-font-mono, 'Courier New', monospace);
    font-weight: 600;
    color: var(--p-red-900, #7f1d1d);
}
.path-error-message {
    margin-left: 0.5rem;
    color: var(--p-red-700, #b91c1c);
}
.album-pick {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding-bottom: 0.8rem;
}
.album-select {
    min-width: 28rem;
    max-width: 100%;
}
.album-detail {
    color: var(--app-text-secondary);
}
.album-conflict {
    margin: 0 0 0.6rem;
    color: var(--p-red-600, #dc2626);
}
.album-rows {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    max-height: 65vh;
    overflow-y: auto;
}
.album-row {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.4rem 0.6rem;
    border: 1px solid var(--app-border);
    border-radius: 6px;
}
.album-row.conflicting {
    border-color: var(--p-red-600, #dc2626);
}
.row-titles {
    display: flex;
    flex-direction: column;
    min-width: 0;
    flex: 1;
    cursor: pointer;
}
.row-proposed {
    font-size: 0.9rem;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.row-current,
.row-error {
    font-size: 0.75rem;
}
.row-current {
    color: var(--app-text-secondary);
}
.row-error {
    color: var(--p-red-600, #dc2626);
}
.row-position {
    font-variant-numeric: tabular-nums;
    color: var(--app-text-secondary);
    min-width: 3.5rem;
    text-align: right;
}
.row-badge {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    min-width: 6.5rem;
    text-align: center;
}
.badge-fingerprint,
.badge-chosen {
    background: var(--p-green-100, #dcfce7);
    color: var(--p-green-800, #166534);
}
.badge-inferred {
    background: var(--p-yellow-100, #fef9c3);
    color: var(--p-yellow-800, #854d0e);
}
.badge-none {
    background: var(--app-surface-alt, #f1f5f9);
    color: var(--app-text-secondary);
}
.row-slot {
    max-width: 14rem;
}
</style>
