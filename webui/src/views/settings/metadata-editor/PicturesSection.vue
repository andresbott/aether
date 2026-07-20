<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Button from 'primevue/button'
import Menu from 'primevue/menu'
import PicturePickerDialog from '@/components/library/PicturePickerDialog.vue'
import CollapsibleSection from './CollapsibleSection.vue'
import { getPictures, getPictureUrl } from '@/lib/api/Metadata'
import {
    PICTURE_SLOTS,
    PICTURE_SLOT_LABELS,
    PICTURE_TYPES,
    pictureTypeLabel
} from '@/lib/pictureTypes'
import type {
    PictureInfo,
    PictureSlot,
    StagedPictureSource,
    Track
} from '@/types/metadata'
import type { EditSession } from '@/composables/useEditSession'

const props = defineProps<{
    selection: Track[]
    libraryId: number | null
    session: EditSession
    // MB release IDs from the album form, used by the picker's online search.
    releaseMbid: string
    releaseGroupMbid: string
}>()

const selectionPaths = computed(() => props.selection.map((t) => t.path))

// Pictures live in the selected tracks' own directory, so picture editing
// targets that directory (resolved from the selection) rather than whatever
// folder is highlighted in the tree — that node can be a parent of the album
// folder, since the track list is populated recursively.
function dirOf(path: string): string {
    const i = path.lastIndexOf('/')
    return i === -1 ? '' : path.slice(0, i)
}
const selectionDirs = computed(() => new Set(props.selection.map((t) => dirOf(t.path))))
// Pictures are per-album (per-directory): editing one requires the selection
// to sit in a single directory. A selection spanning albums shows a note
// instead (mirrors the mixed-artist handling in the form).
const singleAlbum = computed(() => props.selection.length > 0 && selectionDirs.value.size === 1)
const pictureDir = computed<string | null>(() =>
    singleAlbum.value ? [...selectionDirs.value][0] : null
)

// Bumped after a session save wrote picture changes to cache-bust the <img>
// srcs (the endpoint sends no-cache but the URLs are otherwise unchanged).
const pictureBust = ref(0)
watch(
    () => props.session.picturesSavedAt.value,
    (v) => {
        if (v > 0) pictureBust.value = v
    }
)

// What the server currently holds: type -> slot -> detail.
const serverPictures = ref<PictureInfo[]>([])
// Guards against out-of-order responses when the selection changes quickly.
let refreshSeq = 0
async function refreshPictures() {
    const seq = ++refreshSeq
    if (props.libraryId === null || pictureDir.value === null) {
        serverPictures.value = []
        return
    }
    try {
        const pictures = await getPictures(
            props.libraryId,
            pictureDir.value,
            selectionPaths.value
        )
        if (seq === refreshSeq) serverPictures.value = pictures
    } catch {
        if (seq === refreshSeq) serverPictures.value = []
    }
}
// The selection paths matter too: embedded presence is counted over the
// selected tracks, so switching to another song in the SAME folder must
// refetch (the dir alone doesn't change).
watch(
    () => [props.libraryId, pictureDir.value, pictureBust.value, selectionPaths.value.join('\n')],
    refreshPictures,
    { immediate: true }
)

const serverSlotDetail = computed(() => {
    const map = new Map<string, Map<PictureSlot, string>>()
    for (const p of serverPictures.value) {
        const slots = new Map<PictureSlot, string>()
        for (const s of p.slots) slots.set(s.slot, s.detail ?? '')
        map.set(p.type, slots)
    }
    return map
})

// Types added by the user this session (via "Add picture…") that have nothing
// on the server yet; they render as all-empty blocks until an op is staged.
const addedTypes = ref<string[]>([])
watch(pictureDir, () => {
    addedTypes.value = []
})

// The rendered type blocks: server types ∪ staged types ∪ user-added types,
// in registry order. Only ops relevant to the current selection count (an
// embedded op staged for another track in this folder must not surface here).
const visibleTypes = computed(() => {
    const present = new Set<string>()
    for (const p of serverPictures.value) present.add(p.type)
    const entry = pictureDir.value !== null ? props.session.getPictureOps(pictureDir.value) : undefined
    if (entry) {
        for (const [type, slots] of entry.ops) {
            for (const slot of slots.keys()) {
                if (stagedOp(type, slot)) present.add(type)
            }
        }
    }
    for (const t of addedTypes.value) present.add(t)
    return PICTURE_TYPES.filter((t) => present.has(t.id)).map((t) => t.id)
})

// "Add picture…" offers the registry types not already rendered.
const addMenu = ref()
const addableTypes = computed(() =>
    PICTURE_TYPES.filter((t) => !visibleTypes.value.includes(t.id))
)
const addMenuItems = computed(() =>
    addableTypes.value.map((t) => ({
        label: t.label,
        command: () => {
            addedTypes.value.push(t.id)
        }
    }))
)
function toggleAddMenu(event: Event) {
    addMenu.value?.toggle(event)
}

// ----- Per-cell state -----

// stagedOp returns the cell's staged op when it concerns the current
// selection: folder/db ops belong to the whole album; embedded ops only to
// the tracks they were staged for.
function stagedOp(type: string, slot: PictureSlot) {
    if (pictureDir.value === null) return undefined
    const op = props.session.getPictureOp(pictureDir.value, type, slot)
    if (!op) return undefined
    if (slot === 'embedded' && !selectionPaths.value.some((p) => op.paths.includes(p))) {
        return undefined
    }
    return op
}

function serverDetail(type: string, slot: PictureSlot): string | undefined {
    return serverSlotDetail.value.get(type)?.get(slot)
}

function serverHas(type: string, slot: PictureSlot): boolean {
    return serverSlotDetail.value.get(type)?.has(slot) ?? false
}

function cellThumbUrl(type: string, slot: PictureSlot): string | null {
    const op = stagedOp(type, slot)
    if (op?.kind === 'set') return op.preview
    if (op?.kind === 'remove') return null
    if (!serverHas(type, slot) || props.libraryId === null || pictureDir.value === null) return null
    return getPictureUrl(
        props.libraryId,
        pictureDir.value,
        type,
        slot,
        pictureBust.value,
        slot === 'embedded' ? selectionPaths.value : undefined
    )
}

function cellNote(type: string, slot: PictureSlot): string {
    const op = stagedOp(type, slot)
    if (op?.kind === 'set') return 'Pending — saves on Save'
    if (op?.kind === 'remove') return 'Will be removed on Save'
    const detail = serverDetail(type, slot)
    if (detail !== undefined) return detail
    return 'No image'
}

// ----- Actions -----

const picker = ref<{ open: boolean; type: string; slot: PictureSlot }>({
    open: false,
    type: 'Front Cover',
    slot: 'embedded'
})

function openPicker(type: string, slot: PictureSlot) {
    picker.value = { open: true, type, slot }
}

function onPickerSelect(source: StagedPictureSource) {
    if (pictureDir.value === null) return
    props.session.stagePictureSet(
        pictureDir.value,
        picker.value.type,
        picker.value.slot,
        source,
        selectionPaths.value
    )
}

function stageRemove(type: string, slot: PictureSlot) {
    if (pictureDir.value === null) return
    props.session.stagePictureRemoval(pictureDir.value, type, slot, selectionPaths.value)
}

function undoCell(type: string, slot: PictureSlot) {
    if (pictureDir.value === null) return
    props.session.discardPictureOp(pictureDir.value, type, slot)
}
</script>

<template>
    <CollapsibleSection
        title="Attached pictures"
        :help="
            'Artwork attached to the album: front/back cover, disc, booklet… ' +
            'Each picture can be embedded in the song files, stored as an image ' +
            'file in the album folder, or kept in the internal store without ' +
            'touching your files.'
        "
        data-test="pictures-block"
    >
        <template #actions>
            <Button
                v-if="singleAlbum && addableTypes.length > 0"
                icon="pi pi-plus"
                label="Add picture…"
                text
                size="small"
                data-test="add-picture"
                :disabled="libraryId === null"
                @click="toggleAddMenu"
            />
            <Menu ref="addMenu" :model="addMenuItems" :popup="true" />
        </template>

        <template v-if="singleAlbum">
            <div
                v-for="type in visibleTypes"
                :key="type"
                class="picture-type"
                :data-test="`picture-type-${type}`"
            >
                <div class="picture-type-name">{{ pictureTypeLabel(type) }}</div>
                <div class="picture-slots">
                    <template v-for="(slot, i) in PICTURE_SLOTS" :key="slot">
                    <div
                        v-if="i > 0"
                        class="slot-priority"
                        aria-hidden="true"
                        v-tooltip.top="
                            'Loading priority: embedded in file, then album folder, ' +
                            'then internal store'
                        "
                    >
                        <i class="pi pi-angle-right"></i>
                    </div>
                    <div
                        class="picture-cell"
                        :class="{
                            pending: stagedOp(type, slot)?.kind === 'set',
                            removing: stagedOp(type, slot)?.kind === 'remove'
                        }"
                        :data-test="`picture-cell-${type}-${slot}`"
                    >
                        <img
                            v-if="cellThumbUrl(type, slot)"
                            :src="cellThumbUrl(type, slot) ?? undefined"
                            class="cell-thumb"
                            :alt="`${pictureTypeLabel(type)} — ${PICTURE_SLOT_LABELS[slot]}`"
                        />
                        <div v-else class="cell-placeholder"><i class="pi pi-image"></i></div>
                        <div class="cell-info">
                            <span class="cell-slot">{{ PICTURE_SLOT_LABELS[slot] }}</span>
                            <span class="cell-note">{{ cellNote(type, slot) }}</span>
                        </div>
                        <div class="cell-actions">
                            <Button
                                v-if="stagedOp(type, slot)"
                                icon="pi pi-undo"
                                text
                                size="small"
                                aria-label="Undo staged change"
                                :data-test="`picture-undo-${type}-${slot}`"
                                v-tooltip.top="'Discard this staged change'"
                                @click="undoCell(type, slot)"
                            />
                            <template v-else>
                                <Button
                                    icon="pi pi-images"
                                    text
                                    size="small"
                                    aria-label="Change picture"
                                    :data-test="`picture-change-${type}-${slot}`"
                                    :disabled="libraryId === null"
                                    v-tooltip.top="'Choose an image for this slot'"
                                    @click="openPicker(type, slot)"
                                />
                                <Button
                                    v-if="serverHas(type, slot)"
                                    icon="pi pi-trash"
                                    text
                                    size="small"
                                    severity="danger"
                                    aria-label="Remove picture"
                                    :data-test="`picture-remove-${type}-${slot}`"
                                    v-tooltip.top="'Remove this image on Save'"
                                    @click="stageRemove(type, slot)"
                                />
                            </template>
                        </div>
                    </div>
                    </template>
                </div>
            </div>

            <div v-if="visibleTypes.length === 0" class="no-pictures" data-test="no-pictures">
                No attached pictures. Use “Add picture…” to add one.
            </div>
        </template>
        <small v-else class="mixed-note" data-test="pictures-multi-album">
            Select tracks from a single album to manage its pictures.
        </small>

        <PicturePickerDialog
            v-model:visible="picker.open"
            :pictureType="picker.type"
            :pictureSlot="picker.slot"
            :releaseMbid="releaseMbid"
            :releaseGroupMbid="releaseGroupMbid"
            @select="onPickerSelect"
        />
    </CollapsibleSection>
</template>

<style scoped>
.picture-type {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
}
.picture-type-name {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--app-text-secondary);
}
.picture-slots {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr) auto minmax(0, 1fr);
    gap: 0.5rem;
    max-width: 36rem;
}
.slot-priority {
    align-self: center;
    color: var(--app-text-secondary);
    font-size: 1rem;
    line-height: 1;
    cursor: help;
}
.picture-cell {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.35rem;
    padding: 0.5rem;
    border: 1px solid var(--app-border);
    border-radius: 6px;
}
/* Staged (unsaved) cells: the shared amber accent for pending edits. */
.picture-cell.pending {
    border-color: var(--app-staged);
    background-color: var(--app-staged-soft);
}
.picture-cell.pending .cell-note {
    color: var(--app-staged);
}
.picture-cell.removing {
    border-color: var(--app-staged);
    background-color: var(--app-staged-soft);
}
.picture-cell.removing .cell-note {
    color: var(--app-staged);
    text-decoration: line-through;
}
.cell-thumb {
    width: 100%;
    aspect-ratio: 1 / 1;
    object-fit: cover;
    border-radius: 6px;
    border: 1px solid var(--app-border);
    background: var(--app-bg-subtle, #f3f4f6);
}
.picture-cell.pending .cell-thumb {
    border: 2px solid var(--app-staged);
}
.cell-placeholder {
    width: 100%;
    aspect-ratio: 1 / 1;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 6px;
    border: 1px dashed var(--app-border);
    color: var(--app-text-secondary);
    font-size: 1.5rem;
}
.cell-info {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.1rem;
    min-width: 0;
    width: 100%;
    text-align: center;
}
.cell-slot {
    font-size: 0.8rem;
    font-weight: 500;
}
.cell-note {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.cell-actions {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.15rem;
}
.no-pictures {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
}
.mixed-note {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}
</style>
