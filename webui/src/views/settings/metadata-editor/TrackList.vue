<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import type { Track } from '@/types/metadata'

const props = defineProps<{
    tracks: Track[]
    isLoading: boolean
    selection: Track[]
    // Paths of tracks with staged (unsaved) session edits; shown as a marker.
    stagedPaths?: ReadonlySet<string>
    // Library-relative path of the folder selected above the list. Track paths
    // are library-relative too, so we show each one relative to this folder —
    // just the file name for a flat album, or a subfolder-qualified name
    // (e.g. "CD2/02 - Song.mp3") for multi-disc ones. Null/empty means the
    // library root is selected, so the full library-relative path is shown.
    folderPath?: string | null
}>()
const emit = defineEmits<{
    (e: 'update:selection', sel: Track[]): void
}>()

const rows = computed(() => props.tracks)

// Selection mirrors the Now Playing queue (see useQueueEdit): a plain click
// selects just that row — clicking the row that is already the sole selection
// again clears it, so a re-click toggles off — Ctrl/Cmd click toggles a row, and Shift click extends a
// range from the anchor, unioned onto the committed base selection. The checkbox
// column behaves the same way — a bare toggle is additive, a Shift toggle extends
// the range — and its header select-all toggles every row. We drive all of this
// ourselves rather than through PrimeVue's built-in row selection, because
// PrimeVue's Shift range *replaces* the whole selection instead of extending it
// onto what was already selected.
const anchorIndex = ref<number | null>(null)
// The selection a Shift range is unioned onto. Committed by plain/Ctrl clicks and
// checkbox toggles; left untouched by Shift so a range re-drags off one pivot.
let baseSelection: Track[] = []
// Checkbox clicks never reach @row-click — PrimeVue's DataTable swallows clicks
// landing inside .p-checkbox — and the change event they do produce is a plain
// Event with no modifier keys. So remember whether Shift was down on the
// interaction that preceded the toggle.
let shiftHeld = false

const selectable = (t: Track): boolean => !t.error

function isRowSelected(t: Track): boolean {
    return props.selection.some((s) => s.path === t.path)
}

function rowClass(t: Track): string {
    if (t.error) return 'row-error'
    return isRowSelected(t) ? 'row-selected' : ''
}

// The path shown in the list, made relative to the selected folder by dropping
// its prefix. Falls back to the full library-relative path at the library root
// (no folder prefix) or if the path somehow lies outside the folder.
function displayPath(t: Track): string {
    const base = props.folderPath
    if (base && t.path.startsWith(base + '/')) return t.path.slice(base.length + 1)
    return t.path
}

function dedupe(tracks: Track[]): Track[] {
    const seen = new Set<string>()
    const out: Track[] = []
    for (const t of tracks) {
        if (!seen.has(t.path)) {
            seen.add(t.path)
            out.push(t)
        }
    }
    return out
}

function commit(sel: Track[], anchor: number | null): void {
    baseSelection = sel
    anchorIndex.value = anchor
    emit('update:selection', sel)
}

function rangeBetween(a: number, b: number): Track[] {
    const lo = Math.min(a, b)
    const hi = Math.max(a, b)
    const out: Track[] = []
    for (let i = lo; i <= hi; i++) {
        const t = rows.value[i]
        if (t && selectable(t)) out.push(t)
    }
    return out
}

interface RowClickEvent {
    originalEvent: Event
    data: Track
    index: number
}

function onRowClick(event: RowClickEvent): void {
    const track = event.data
    if (!selectable(track)) return
    const { ctrlKey, metaKey, shiftKey } = event.originalEvent as MouseEvent

    if (shiftKey && anchorIndex.value !== null) {
        // Shift extends the range onto the base; the anchor/base stay put so the
        // range can be re-dragged off the same pivot.
        emit(
            'update:selection',
            dedupe([...baseSelection, ...rangeBetween(anchorIndex.value, event.index)])
        )
        return
    }
    if (ctrlKey || metaKey) {
        const next = isRowSelected(track)
            ? props.selection.filter((t) => t.path !== track.path)
            : [...props.selection, track]
        commit(next, event.index)
        return
    }
    // Clicking the row that is already the sole selection clears it, so a plain
    // re-click toggles off. Clicking any other row — or a selected row while
    // several are selected — still collapses down to just that row.
    if (isRowSelected(track) && props.selection.length === 1) {
        commit([], null)
        return
    }
    commit([track], event.index)
}

// Checkbox toggles and the header select-all arrive through PrimeVue's
// update:selection. Treat them as an additive commit: strip error rows and move
// the anchor to the toggled row so a following Shift extends from it. With Shift
// held on a single-row toggle, behave exactly like a Shift row click instead.
function onCheckboxSelection(value: Track[]): void {
    const next = value.filter(selectable)
    const before = new Set(props.selection.map((t) => t.path))
    const after = new Set(next.map((t) => t.path))
    const touched = [
        ...next.filter((t) => !before.has(t.path)),
        ...props.selection.filter((t) => !after.has(t.path))
    ]

    if (shiftHeld && anchorIndex.value !== null && touched.length === 1) {
        const index = rows.value.findIndex((t) => t.path === touched[0].path)
        if (index !== -1) {
            emit(
                'update:selection',
                dedupe([...baseSelection, ...rangeBetween(anchorIndex.value, index)])
            )
            return
        }
    }

    const changed = touched[0]
    const anchor = changed ? rows.value.findIndex((t) => t.path === changed.path) : anchorIndex.value
    commit(next, anchor)
}

// A new folder (or reload) resets the pivot; the parent clears the selection.
watch(rows, () => {
    anchorIndex.value = null
    baseSelection = []
})

// Runs before the checkbox's change event, so it records the modifier state of
// the interaction that is about to toggle a row.
function onModifierProbe(event: MouseEvent | KeyboardEvent): void {
    shiftHeld = event.shiftKey
}

// Escape clears the whole selection. Arrow keys move a single-row selection
// up/down (skipping error rows), and are only active with exactly one selected
// track — with a multi-selection the "move" intent is ambiguous, so those keys
// are left alone.
function onKeydown(event: KeyboardEvent): void {
    if (event.key === 'Escape') {
        if (props.selection.length === 0) return
        event.preventDefault()
        commit([], null)
        return
    }
    if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return
    if (props.selection.length !== 1) return
    const current = rows.value.findIndex((t) => t.path === props.selection[0].path)
    if (current === -1) return
    const step = event.key === 'ArrowDown' ? 1 : -1
    for (let i = current + step; i >= 0 && i < rows.value.length; i += step) {
        const t = rows.value[i]
        if (!selectable(t)) continue
        event.preventDefault()
        commit([t], i)
        scrollRowIntoView(i)
        return
    }
    // At the edge of the list: swallow the key so the page doesn't scroll.
    event.preventDefault()
}

function scrollRowIntoView(index: number): void {
    const row = wrapperEl.value?.querySelectorAll('tbody tr')[index]
    row?.scrollIntoView({ block: 'nearest' })
}

const wrapperEl = ref<HTMLElement | null>(null)
</script>

<template>
    <div class="track-list">
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 1.5rem"></i>
        </div>
        <div v-else-if="rows.length === 0" class="empty">No audio files in this folder.</div>

        <!-- tabindex makes the wrapper focusable: clicking a row focuses it, so
             arrow keys can then move a single-row selection. -->
        <div
            v-else
            class="table-wrapper"
            ref="wrapperEl"
            tabindex="0"
            @keydown="onKeydown"
            @mousedown.capture="onModifierProbe"
            @keydown.capture="onModifierProbe"
        >
        <DataTable
            :value="rows"
            :selection="selection"
            dataKey="path"
            :rowClass="rowClass"
            @row-click="onRowClick"
            @update:selection="onCheckboxSelection"
        >
            <Column selectionMode="multiple" style="width: 3rem" />
            <Column style="width: 1.5rem">
                <template #body="{ data }">
                    <i
                        v-if="stagedPaths?.has((data as Track).path)"
                        class="pi pi-circle-fill staged-dot"
                        v-tooltip.right="'Unsaved changes'"
                        data-test="staged-marker"
                    ></i>
                </template>
            </Column>
            <Column field="path" header="Path">
                <template #body="{ data }">{{ displayPath(data as Track) }}</template>
            </Column>
            <Column header="" style="width: 10rem">
                <template #body="{ data }">
                    <span v-if="(data as Track).error" class="err" :title="(data as Track).error">
                        read error
                    </span>
                </template>
            </Column>
        </DataTable>
        </div>
    </div>
</template>

<style scoped>
.track-list {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
}

.table-wrapper {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
    outline: none;
}
.loading,
.empty {
    padding: 2rem;
    text-align: center;
    color: var(--app-text-secondary);
}
:deep(tbody tr) {
    cursor: pointer;
}
:deep(tr.row-selected) {
    background-color: var(--app-accent-soft);
}
:deep(.row-error) {
    opacity: 0.5;
    cursor: default;
}
.err {
    color: var(--p-red-600, #dc2626);
    font-size: 0.8rem;
}
.staged-dot {
    color: var(--app-staged);
    font-size: 0.5rem;
    vertical-align: middle;
}
:deep(tr:has(.staged-dot)) {
    background-color: var(--app-staged-soft);
}
</style>
