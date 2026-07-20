<script setup lang="ts">
import { computed, ref } from 'vue'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import Button from 'primevue/button'
import { useRawTags } from '@/composables/useMetadataEditor'
import type { EditSession } from '@/composables/useEditSession'
import { isManagedTag } from '@/types/metadata'
import type { Track } from '@/types/metadata'

const props = defineProps<{
    selection: Track[]
    libraryId: number | null
    session: EditSession
}>()

const selectionPaths = computed(() => props.selection.map((t) => t.path))

const rawQuery = useRawTags(
    () => props.libraryId,
    () => selectionPaths.value,
    () => true
)

const results = computed(() => rawQuery.data.value ?? [])
const readErrors = computed(() => results.value.filter((r) => r.error))

// originalsFor collects each track's on-disk values for one key (absent key =
// empty list), the reference stageRawKey normalizes against.
function originalsFor(key: string): Map<string, string[]> {
    const map = new Map<string, string[]>()
    for (const r of results.value) map.set(r.path, r.tags[key] ?? [])
    return map
}

// One display row per distinct key across the selection, with the structured
// editor's shared/mixed semantics: a key differing between tracks (or missing
// on some) is "mixed" and shows a placeholder until overwritten.
interface RawRow {
    key: string
    managed: boolean
    shared: boolean
    // Effective values (staged overlay ?? original) when shared; [] when mixed.
    values: string[]
    staged: boolean
    // True when the staged edit is a delete (empty values on every track).
    stagedDelete: boolean
}

function effectiveValues(path: string, key: string): string[] {
    return props.session.stagedRawValue(path, key) ?? originalsFor(key).get(path) ?? []
}

const rows = computed<RawRow[]>(() => {
    const keys = new Set<string>()
    for (const r of results.value) {
        for (const k of Object.keys(r.tags)) keys.add(k)
    }
    // Staged-but-new keys (added via "Add tag") must show too.
    for (const p of selectionPaths.value) {
        const raw = props.session.overlays.value.get(p)?.raw
        if (raw) for (const k of Object.keys(raw)) keys.add(k)
    }
    const out: RawRow[] = []
    for (const key of [...keys].sort()) {
        const perTrack = selectionPaths.value.map((p) => effectiveValues(p, key))
        const first = perTrack[0] ?? []
        const shared = perTrack.every(
            (v) => v.length === first.length && v.every((x, i) => x === first[i])
        )
        const staged = props.session.isRawKeyStaged(selectionPaths.value, key)
        out.push({
            key,
            managed: isManagedTag(key),
            shared,
            values: shared ? first : [],
            staged,
            stagedDelete: staged && shared && first.length === 0
        })
    }
    return out
})

// Value editing: one value per line. Buffered per key so typing does not
// re-render against the recomputed row mid-edit.
const editBuffers = ref(new Map<string, string>())

function displayValue(row: RawRow): string {
    return editBuffers.value.get(row.key) ?? row.values.join('\n')
}

function onValueInput(row: RawRow, text: string) {
    editBuffers.value.set(row.key, text)
    const values = text
        .split('\n')
        .map((v) => v.trim())
        .filter((v) => v !== '')
    props.session.stageRawKey(selectionPaths.value, row.key, values, originalsFor(row.key))
}

function deleteKey(row: RawRow) {
    editBuffers.value.delete(row.key)
    props.session.stageRawKey(selectionPaths.value, row.key, [], originalsFor(row.key))
}

function revertKey(row: RawRow) {
    editBuffers.value.delete(row.key)
    props.session.unstageRawKey(selectionPaths.value, row.key)
}

function revertTooltip(row: RawRow): string {
    const originals = originalsFor(row.key)
    const perTrack = selectionPaths.value.map((p) => originals.get(p) ?? [])
    const first = perTrack[0] ?? []
    const shared = perTrack.every(
        (v) => v.length === first.length && v.every((x, i) => x === first[i])
    )
    if (!shared) return 'Revert to each track’s own value'
    if (first.length === 0) return 'Revert to empty'
    return `Revert to “${first.join(', ')}”`
}

// ----- Hidden frames (unsupported data) -----
// Frames the tag map cannot represent (ID3v2 PRIV/GEOB/POPM, unknown binary
// frames). They can only be deleted, not viewed or edited: descriptors are
// identifiers without payload.

// unsupportedByPath maps each track to its hidden-frame descriptors.
const unsupportedByPath = computed(() => {
    const map = new Map<string, string[]>()
    for (const r of results.value) map.set(r.path, r.unsupported ?? [])
    return map
})

interface HiddenFrameRow {
    descriptor: string
    // How many of the selected tracks carry this frame.
    count: number
    staged: boolean
}

const hiddenFrameRows = computed<HiddenFrameRow[]>(() => {
    const counts = new Map<string, number>()
    for (const r of results.value) {
        for (const d of r.unsupported ?? []) {
            counts.set(d, (counts.get(d) ?? 0) + 1)
        }
    }
    return [...counts.entries()].sort(([a], [b]) => a.localeCompare(b)).map(([descriptor, count]) => ({
        descriptor,
        count,
        staged: props.session.isUnsupportedRemovalStaged(selectionPaths.value, descriptor)
    }))
})

function deleteHiddenFrame(row: HiddenFrameRow) {
    props.session.stageUnsupportedRemoval(
        selectionPaths.value,
        row.descriptor,
        unsupportedByPath.value
    )
}

function revertHiddenFrame(row: HiddenFrameRow) {
    props.session.unstageUnsupportedRemoval(selectionPaths.value, row.descriptor)
}

// hiddenFrameLabel renders a descriptor with its frame type spelled out where
// the id is a well-known junk carrier.
const FRAME_LABELS: Record<string, string> = {
    PRIV: 'private data',
    GEOB: 'binary object',
    POPM: 'rating / play count',
    UFID: 'file identifier',
    SYLT: 'synced lyrics',
    ETCO: 'event timing codes',
    UNKNOWN: 'unknown frame'
}

function hiddenFrameHint(descriptor: string): string {
    const id = descriptor.split('/')[0]
    return FRAME_LABELS[id] ?? ''
}

// ----- Add tag -----
const newKey = ref('')
const newValue = ref('')
const addError = computed(() => {
    const key = newKey.value.trim().toUpperCase()
    if (key === '') return ''
    if (isManagedTag(key)) return 'This tag is managed by the metadata editor.'
    if (rows.value.some((r) => r.key.toUpperCase() === key)) return 'Tag already exists.'
    return ''
})

function addTag() {
    const key = newKey.value.trim().toUpperCase()
    if (key === '' || addError.value !== '') return
    const values = newValue.value
        .split('\n')
        .map((v) => v.trim())
        .filter((v) => v !== '')
    if (values.length === 0) return
    props.session.stageRawKey(selectionPaths.value, key, values, originalsFor(key))
    newKey.value = ''
    newValue.value = ''
}
</script>

<template>
    <div class="raw-panel">
        <div v-if="rawQuery.isLoading.value" class="raw-loading">
            <i class="pi pi-spin pi-spinner"></i>
        </div>
        <template v-else>
            <small v-for="err in readErrors" :key="err.path" class="raw-error">
                {{ err.path }}: {{ err.error }}
            </small>

            <div v-if="rows.length === 0 && readErrors.length === 0" class="raw-empty">
                No tags found.
            </div>

            <div
                v-for="row in rows"
                :key="row.key"
                class="raw-row"
                :class="{ 'raw-dirty': row.staged, 'raw-deleted': row.stagedDelete }"
                :data-test="`raw-row-${row.key}`"
            >
                <div class="raw-key">
                    <span class="raw-key-name">{{ row.key }}</span>
                    <i
                        v-if="row.managed"
                        class="pi pi-lock raw-managed"
                        v-tooltip.right="
                            'Managed by the metadata editor — edit this field in the form view.'
                        "
                        data-test="raw-managed"
                    ></i>
                </div>

                <span v-if="row.stagedDelete" class="raw-delete-note" data-test="raw-delete-note">
                    deleted on Save
                </span>
                <Textarea
                    v-else
                    class="raw-value"
                    autoResize
                    rows="1"
                    :modelValue="displayValue(row)"
                    :placeholder="row.shared ? '' : '(multiple values — typing overwrites all)'"
                    :disabled="row.managed"
                    @update:modelValue="(v: string) => onValueInput(row, v)"
                />

                <div class="raw-actions">
                    <Button
                        v-if="row.staged"
                        icon="pi pi-undo"
                        text
                        size="small"
                        :aria-label="`Revert ${row.key}`"
                        v-tooltip.left="revertTooltip(row)"
                        :data-test="`raw-undo-${row.key}`"
                        @click="revertKey(row)"
                    />
                    <Button
                        v-if="!row.managed && !row.stagedDelete"
                        icon="pi pi-trash"
                        text
                        size="small"
                        severity="danger"
                        :aria-label="`Delete ${row.key}`"
                        :data-test="`raw-delete-${row.key}`"
                        @click="deleteKey(row)"
                    />
                </div>
            </div>

            <div v-if="hiddenFrameRows.length > 0" class="raw-hidden" data-test="raw-hidden">
                <div class="raw-hidden-header">
                    <span class="raw-hidden-title">Hidden frames</span>
                    <i
                        class="pi pi-question-circle raw-hidden-help"
                        v-tooltip.right="
                            'Binary or non-text metadata (private data, ratings, embedded objects) that cannot be shown as tags. It can only be deleted.'
                        "
                    ></i>
                </div>
                <div
                    v-for="row in hiddenFrameRows"
                    :key="row.descriptor"
                    class="raw-row"
                    :class="{ 'raw-dirty': row.staged }"
                    :data-test="`raw-hidden-row-${row.descriptor}`"
                >
                    <div class="raw-key">
                        <span class="raw-key-name">{{ row.descriptor }}</span>
                    </div>
                    <div class="raw-hidden-desc">
                        <span v-if="row.staged" class="raw-delete-note" data-test="raw-hidden-delete-note">
                            deleted on Save
                        </span>
                        <template v-else>
                            <span v-if="hiddenFrameHint(row.descriptor)">
                                {{ hiddenFrameHint(row.descriptor) }}
                            </span>
                            <span v-if="selectionPaths.length > 1" class="raw-hidden-count">
                                on {{ row.count }} of {{ selectionPaths.length }} tracks
                            </span>
                        </template>
                    </div>
                    <div class="raw-actions">
                        <Button
                            v-if="row.staged"
                            icon="pi pi-undo"
                            text
                            size="small"
                            :aria-label="`Revert ${row.descriptor}`"
                            v-tooltip.left="'Keep this frame'"
                            :data-test="`raw-hidden-undo-${row.descriptor}`"
                            @click="revertHiddenFrame(row)"
                        />
                        <Button
                            v-else
                            icon="pi pi-trash"
                            text
                            size="small"
                            severity="danger"
                            :aria-label="`Delete ${row.descriptor}`"
                            :data-test="`raw-hidden-delete-${row.descriptor}`"
                            @click="deleteHiddenFrame(row)"
                        />
                    </div>
                </div>
            </div>

            <div class="raw-add" data-test="raw-add">
                <InputText
                    class="raw-add-key"
                    v-model="newKey"
                    placeholder="NEW_TAG_NAME"
                    data-test="raw-add-key"
                />
                <Textarea
                    class="raw-add-value"
                    autoResize
                    rows="1"
                    v-model="newValue"
                    placeholder="Value (one per line)"
                    data-test="raw-add-value"
                />
                <Button
                    icon="pi pi-plus"
                    label="Add tag"
                    text
                    size="small"
                    :disabled="newKey.trim() === '' || newValue.trim() === '' || addError !== ''"
                    data-test="raw-add-button"
                    @click="addTag"
                />
            </div>
            <small v-if="addError" class="raw-error" data-test="raw-add-error">
                {{ addError }}
            </small>
        </template>
    </div>
</template>

<style scoped>
.raw-panel {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
}
.raw-loading,
.raw-empty {
    padding: 1.5rem;
    text-align: center;
    color: var(--app-text-secondary);
}
.raw-error {
    color: var(--p-red-600, #dc2626);
}
.raw-row {
    display: grid;
    grid-template-columns: 14rem 1fr auto;
    align-items: start;
    gap: 0.5rem;
    padding: 0.3rem 0.4rem;
    border-radius: 4px;
}
.raw-row.raw-dirty {
    background-color: var(--app-staged-soft);
}
.raw-key {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    min-width: 0;
    padding-top: 0.45rem;
}
.raw-key-name {
    font-family: var(--font-mono, monospace);
    font-size: 0.8rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.raw-dirty .raw-key-name {
    color: var(--app-staged);
    font-weight: 600;
}
.raw-managed {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
    cursor: help;
    flex: 0 0 auto;
}
.raw-value {
    width: 100%;
    font-family: var(--font-mono, monospace);
    font-size: 0.8rem;
}
.raw-dirty :deep(.raw-value) {
    border-color: var(--app-staged);
}
.raw-delete-note {
    padding-top: 0.45rem;
    font-size: 0.8rem;
    color: var(--app-staged);
    text-decoration: line-through;
}
.raw-actions {
    display: flex;
    align-items: center;
    gap: 0.15rem;
}
.raw-hidden {
    margin-top: 0.75rem;
    padding-top: 0.75rem;
    border-top: 1px solid var(--app-border);
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
}
.raw-hidden-header {
    display: flex;
    align-items: center;
    gap: 0.35rem;
}
.raw-hidden-title {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--app-text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
}
.raw-hidden-help {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    cursor: help;
}
.raw-hidden-desc {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding-top: 0.45rem;
    font-size: 0.8rem;
    color: var(--app-text-secondary);
}
.raw-hidden-count {
    font-style: italic;
}
.raw-add {
    display: grid;
    grid-template-columns: 14rem 1fr auto;
    align-items: start;
    gap: 0.5rem;
    margin-top: 0.75rem;
    padding-top: 0.75rem;
    border-top: 1px solid var(--app-border);
}
.raw-add-key {
    font-family: var(--font-mono, monospace);
    font-size: 0.8rem;
    text-transform: uppercase;
}
.raw-add-value {
    font-family: var(--font-mono, monospace);
    font-size: 0.8rem;
}
</style>
