<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Button from 'primevue/button'
import Menu from 'primevue/menu'
import PicturePickerDialog from '@/components/library/PicturePickerDialog.vue'
import CollapsibleSection from './CollapsibleSection.vue'
import { getPictures, getPictureUrl } from '@/lib/api/Metadata'
import { selectionAlbumKey, selectionDirs } from '@/lib/albumIdentity'
import {
    PICTURE_SLOTS,
    PICTURE_SLOT_LABELS,
    PICTURE_TYPES,
    pictureTypeLabel
} from '@/lib/pictureTypes'
import type {
    PictureCopySource,
    PictureInfo,
    PictureSlot,
    PictureSlotInfo,
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
    // Album name from the form, prefilling the picker's manual release search.
    albumName: string
}>()

const selectionPaths = computed(() => props.selection.map((t) => t.path))

// Pictures belong to an ALBUM, identified by its tags (see selectionAlbumKey) —
// not to a directory. A multi-disc release is usually laid out as CD 1/, CD 2/
// subfolders yet is one album, and its folder art is written into each of those
// folders. Editing requires the selection to be one album; a selection spanning
// albums shows a note instead (mirrors the mixed-artist handling in the form).
// albumId keys the album's staged picture ops in the session.
const albumId = computed<string | null>(() => selectionAlbumKey(props.selection))
const singleAlbum = computed(() => albumId.value !== null)
// Picture requests are anchored on the album's primary (first) directory; the
// selection paths tell the server every folder the album spans.
const pictureDir = computed<string | null>(() =>
    singleAlbum.value ? (selectionDirs(props.selection)[0] ?? null) : null
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
        const pictures = await getPictures(props.libraryId, pictureDir.value, selectionPaths.value)
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
    const map = new Map<string, Map<PictureSlot, PictureSlotInfo>>()
    for (const p of serverPictures.value) {
        const slots = new Map<PictureSlot, PictureSlotInfo>()
        for (const s of p.slots) slots.set(s.slot, s)
        map.set(p.type, slots)
    }
    return map
})

// Types added by the user this session (via "Add picture…") that have nothing
// on the server yet; they render as all-empty blocks until an op is staged.
const addedTypes = ref<string[]>([])
watch(albumId, () => {
    addedTypes.value = []
})

// The rendered type blocks: server types ∪ staged types ∪ user-added types,
// in registry order. Only ops relevant to the current selection count (an
// embedded op staged for another track in this folder must not surface here).
const visibleTypes = computed(() => {
    const present = new Set<string>()
    for (const p of serverPictures.value) present.add(p.type)
    const entry = albumId.value !== null ? props.session.getPictureOps(albumId.value) : undefined
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
const addableTypes = computed(() => PICTURE_TYPES.filter((t) => !visibleTypes.value.includes(t.id)))
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
    if (albumId.value === null) return undefined
    const op = props.session.getPictureOp(albumId.value, type, slot)
    if (!op) return undefined
    if (slot === 'embedded' && !selectionPaths.value.some((p) => op.paths.includes(p))) {
        return undefined
    }
    return op
}

function serverDetail(type: string, slot: PictureSlot): string | undefined {
    const info = serverSlotDetail.value.get(type)?.get(slot)
    return info === undefined ? undefined : (info.detail ?? '')
}

// serverMixed marks a folder cell whose art is not the same in every directory
// the album spans (a multi-disc release): the grid shows the first folder's
// image, and saving overwrites all of them.
function serverMixed(type: string, slot: PictureSlot): boolean {
    return serverSlotDetail.value.get(type)?.get(slot)?.mixed === true
}

function serverHas(type: string, slot: PictureSlot): boolean {
    return serverSlotDetail.value.get(type)?.has(slot) ?? false
}

// PICTURE_CELL_SIZE is the pixel size grid thumbnails are requested at. The
// cells render at roughly 160 CSS pixels; 320 keeps them sharp on 2x displays
// while staying a fraction of a full cover scan.
const PICTURE_CELL_SIZE = 320

function cellThumbUrl(type: string, slot: PictureSlot): string | null {
    const op = stagedOp(type, slot)
    if (op?.kind === 'set') return op.preview
    if (op?.kind === 'remove') return null
    return serverPictureUrl(type, slot, PICTURE_CELL_SIZE)
}

// serverPictureUrl builds the endpoint URL for a server-held cell. size omitted
// means the original bytes — required when the image is copied into another
// slot rather than displayed.
function serverPictureUrl(type: string, slot: PictureSlot, size?: number): string | null {
    if (!serverHas(type, slot) || props.libraryId === null || pictureDir.value === null) return null
    return getPictureUrl(
        props.libraryId,
        pictureDir.value,
        type,
        slot,
        pictureBust.value,
        // Embedded: narrow the probe to the selected tracks. Folder: name the
        // directories the album spans, since the art may sit in a later disc
        // folder than the primary one this URL is anchored on.
        slot === 'db' ? undefined : selectionPaths.value,
        size
    )
}

function cellNote(type: string, slot: PictureSlot): string {
    const op = stagedOp(type, slot)
    if (op?.kind === 'set') return 'Pending — saves on Save'
    if (op?.kind === 'remove') return 'Will be removed on Save'
    const detail = serverDetail(type, slot)
    if (detail === undefined) return 'No image'
    if (serverMixed(type, slot)) {
        return detail === '' ? 'differs across folders' : `${detail} — differs across folders`
    }
    return detail
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

// Every other cell of this album that currently holds an image — the picker
// offers them as copy sources (e.g. copy the embedded front cover into the
// internal store). A staged image hands over its own file/URL; a server-held
// one is downloaded by the picker from its image endpoint. The cell being
// edited is excluded: copying it onto itself is a no-op.
const copySources = computed<PictureCopySource[]>(() => {
    const target = picker.value
    const out: PictureCopySource[] = []
    const cells = new Set<string>()
    for (const p of serverPictures.value) for (const s of p.slots) cells.add(`${p.type}\n${s.slot}`)
    const entry = albumId.value !== null ? props.session.getPictureOps(albumId.value) : undefined
    if (entry) {
        for (const [type, slots] of entry.ops) {
            for (const slot of slots.keys()) cells.add(`${type}\n${slot}`)
        }
    }
    for (const { id: type } of PICTURE_TYPES) {
        for (const slot of PICTURE_SLOTS) {
            if (!cells.has(`${type}\n${slot}`)) continue
            if (type === target.type && slot === target.slot) continue
            // null covers both an empty cell and one staged for removal.
            const thumb = cellThumbUrl(type, slot)
            if (thumb === null) continue
            const op = stagedOp(type, slot)
            out.push({
                key: `${type}-${slot}`,
                label: `${pictureTypeLabel(type)} — ${PICTURE_SLOT_LABELS[slot]}`,
                detail:
                    op?.kind === 'set'
                        ? 'pending change in this session'
                        : (serverDetail(type, slot) ?? ''),
                thumbUrl: thumb,
                file: op?.kind === 'set' ? op.file : null,
                imageUrl: op?.kind === 'set' ? op.imageUrl : null,
                // Only server-held images need a download; a staged op already
                // carries its bytes (file) or its remote URL. The download URL
                // deliberately carries no size: copying a picture into another
                // slot must store the original, not the grid thumbnail.
                fetchUrl: op?.kind === 'set' ? null : serverPictureUrl(type, slot)
            })
        }
    }
    return out
})

function onPickerSelect(source: StagedPictureSource) {
    if (albumId.value === null) return
    props.session.stagePictureSet(
        albumId.value,
        picker.value.type,
        picker.value.slot,
        source,
        selectionPaths.value
    )
}

function stageRemove(type: string, slot: PictureSlot) {
    if (albumId.value === null) return
    props.session.stagePictureRemoval(albumId.value, type, slot, selectionPaths.value)
}

function undoCell(type: string, slot: PictureSlot) {
    if (albumId.value === null) return
    props.session.discardPictureOp(albumId.value, type, slot)
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
                            <!-- Occupied cell: the image itself, flipping on hover to
                             the change/remove controls (mirrors the hero cover). -->
                            <div v-if="cellThumbUrl(type, slot)" class="cell-art">
                                <div class="cell-flip">
                                    <div class="cell-face cell-front">
                                        <img
                                            :src="cellThumbUrl(type, slot) ?? undefined"
                                            class="cell-thumb"
                                            :alt="`${pictureTypeLabel(type)} — ${PICTURE_SLOT_LABELS[slot]}`"
                                        />
                                    </div>
                                    <div class="cell-face cell-back">
                                        <Button
                                            v-if="stagedOp(type, slot)"
                                            icon="pi pi-undo"
                                            label="Undo"
                                            text
                                            size="small"
                                            aria-label="Undo staged change"
                                            :data-test="`picture-undo-${type}-${slot}`"
                                            @click="undoCell(type, slot)"
                                        />
                                        <template v-else>
                                            <Button
                                                icon="pi pi-images"
                                                label="Change"
                                                text
                                                size="small"
                                                aria-label="Change picture"
                                                :data-test="`picture-change-${type}-${slot}`"
                                                :disabled="libraryId === null"
                                                @click="openPicker(type, slot)"
                                            />
                                            <Button
                                                v-if="serverHas(type, slot)"
                                                icon="pi pi-trash"
                                                label="Remove"
                                                text
                                                size="small"
                                                severity="danger"
                                                aria-label="Remove picture"
                                                :data-test="`picture-remove-${type}-${slot}`"
                                                @click="stageRemove(type, slot)"
                                            />
                                        </template>
                                    </div>
                                </div>
                            </div>
                            <!-- Empty cell: no image to flip, so the add button is
                             the placeholder itself. -->
                            <div v-else class="cell-art">
                                <Button
                                    v-if="stagedOp(type, slot)"
                                    class="cell-placeholder-btn"
                                    icon="pi pi-undo"
                                    label="Undo"
                                    text
                                    size="small"
                                    aria-label="Undo staged change"
                                    :data-test="`picture-undo-${type}-${slot}`"
                                    @click="undoCell(type, slot)"
                                />
                                <Button
                                    v-else
                                    class="cell-placeholder-btn"
                                    icon="pi pi-plus"
                                    label="Add image"
                                    text
                                    size="small"
                                    aria-label="Change picture"
                                    :data-test="`picture-change-${type}-${slot}`"
                                    :disabled="libraryId === null"
                                    @click="openPicker(type, slot)"
                                />
                            </div>
                            <div class="cell-info">
                                <span class="cell-slot">{{ PICTURE_SLOT_LABELS[slot] }}</span>
                                <span class="cell-note">{{ cellNote(type, slot) }}</span>
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
            :albumName="albumName"
            :sources="copySources"
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
/* --- The image tile, with a 3D flip to its controls on hover/focus. ---
   Same idea as the hero cover (HeroHeader), but hover-driven and always in a 3D
   context: `backface-visibility` (not `display: none`) hides the control face,
   so its buttons stay tabbable — tabbing to one triggers :focus-within and
   flips the tile into view. Unlike the hero cover there is no prop-driven
   initial state, so an always-on 3D layer cannot animate a flip on mount. */
.cell-art {
    width: 100%;
    aspect-ratio: 1 / 1;
    position: relative;
    perspective: 800px;
}
.cell-flip {
    width: 100%;
    height: 100%;
    position: relative;
    transform-style: preserve-3d;
    transition: transform 0.4s;
}
.cell-art:hover .cell-flip,
.cell-art:focus-within .cell-flip {
    transform: rotateY(180deg);
}
.cell-face {
    position: absolute;
    inset: 0;
    border-radius: 6px;
    overflow: hidden;
    backface-visibility: hidden;
}
.cell-back {
    transform: rotateY(180deg);
    background: var(--app-surface-2);
    border: 1px solid var(--app-border);
    display: flex;
    flex-direction: column;
    align-items: stretch;
    justify-content: center;
    gap: 0.15rem;
    padding: 0.35rem;
}
.cell-thumb {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: 6px;
    border: 1px solid var(--app-border);
    background: var(--app-bg-subtle, #f3f4f6);
    display: block;
}
.picture-cell.pending .cell-thumb {
    border: 2px solid var(--app-staged);
}
/* An empty cell has no image to flip: the add button IS the placeholder. */
.cell-placeholder-btn {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    border-radius: 6px;
    border: 1px dashed var(--app-border);
    color: var(--app-text-secondary);
}
.cell-placeholder-btn:hover {
    border-color: var(--app-accent);
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
.no-pictures {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
}
.mixed-note {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}
</style>
