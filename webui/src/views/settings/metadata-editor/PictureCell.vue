<script setup lang="ts">
import Button from 'primevue/button'

// PictureCell is the shared image card of the metadata editor: a square image on
// the left with its edit controls ON the image (a hover/focus flip to
// Change/Remove, or Undo for a staged change), and an info column on the right
// (title, optional subtitle, a note and an image-metadata line). Used by both
// the attached-pictures grid and the artist-image section.
//
// It is presentational: it emits change/remove/undo and lets the parent decide
// what each does. The parent also supplies the data-test ids, since the two call
// sites name their cells differently.
withDefaults(
    defineProps<{
        // Thumbnail URL, or null for an empty cell (renders the add placeholder).
        image: string | null
        alt?: string
        title: string
        subtitle?: string
        note?: string
        meta?: string | null
        // An edit is staged for this cell: the controls collapse to a single Undo.
        staged?: boolean
        // Card styling for a staged add/replace (amber) vs a staged removal
        // (amber + struck-through note).
        pending?: boolean
        removing?: boolean
        // Show Remove in the idle (unstaged) state.
        canRemove?: boolean
        // Disable Change/Add (e.g. no library selected yet).
        disabled?: boolean
        addLabel?: string
        changeTestId?: string
        removeTestId?: string
        undoTestId?: string
        metaTestId?: string
    }>(),
    {
        alt: 'Image',
        subtitle: '',
        note: '',
        meta: null,
        staged: false,
        pending: false,
        removing: false,
        canRemove: false,
        disabled: false,
        addLabel: 'Add image'
    }
)

defineEmits<{
    (e: 'change'): void
    (e: 'remove'): void
    (e: 'undo'): void
}>()
</script>

<template>
    <div class="picture-cell" :class="{ pending, removing }">
        <div class="cell-art">
            <!-- Occupied: the image, flipping on hover/focus to its controls. -->
            <div v-if="image" class="cell-flip">
                <div class="cell-face cell-front">
                    <img :src="image" class="cell-thumb" :alt="alt" />
                </div>
                <div class="cell-face cell-back">
                    <Button
                        v-if="staged"
                        icon="pi pi-undo"
                        label="Undo"
                        text
                        size="small"
                        aria-label="Undo staged change"
                        :data-test="undoTestId"
                        @click="$emit('undo')"
                    />
                    <template v-else>
                        <Button
                            icon="pi pi-images"
                            label="Change"
                            text
                            size="small"
                            aria-label="Change image"
                            :disabled="disabled"
                            :data-test="changeTestId"
                            @click="$emit('change')"
                        />
                        <Button
                            v-if="canRemove"
                            icon="pi pi-trash"
                            label="Remove"
                            text
                            size="small"
                            severity="danger"
                            aria-label="Remove image"
                            :data-test="removeTestId"
                            @click="$emit('remove')"
                        />
                    </template>
                </div>
            </div>
            <!-- Empty: the add button IS the placeholder (or Undo for a staged
                 removal that emptied the cell). -->
            <template v-else>
                <Button
                    v-if="staged"
                    class="cell-placeholder-btn"
                    icon="pi pi-undo"
                    label="Undo"
                    text
                    size="small"
                    aria-label="Undo staged change"
                    :data-test="undoTestId"
                    @click="$emit('undo')"
                />
                <Button
                    v-else
                    class="cell-placeholder-btn"
                    icon="pi pi-plus"
                    :label="addLabel"
                    text
                    size="small"
                    :aria-label="addLabel"
                    :disabled="disabled"
                    :data-test="changeTestId"
                    @click="$emit('change')"
                />
            </template>
        </div>

        <div class="cell-info">
            <span class="cell-title">{{ title }}</span>
            <span v-if="subtitle" class="cell-sub">{{ subtitle }}</span>
            <span v-if="note" class="cell-note">{{ note }}</span>
            <span v-if="meta" class="cell-meta" :data-test="metaTestId">{{ meta }}</span>
        </div>
    </div>
</template>

<style scoped>
.picture-cell {
    display: flex;
    gap: 0.6rem;
    align-items: flex-start;
    padding: 0.5rem;
    border: 1px solid var(--app-border);
    border-radius: 6px;
    text-align: left;
}
/* Staged (unsaved) cells: the shared amber accent for pending edits. */
.picture-cell.pending,
.picture-cell.removing {
    border-color: var(--app-staged);
    background-color: var(--app-staged-soft);
}
.picture-cell.pending .cell-note,
.picture-cell.removing .cell-note {
    color: var(--app-staged);
}
.picture-cell.removing .cell-note {
    text-decoration: line-through;
}

/* --- The image tile, with a 3D flip to its controls on hover/focus. ---
   `backface-visibility` (not `display: none`) hides the control face, so its
   buttons stay tabbable — tabbing to one triggers :focus-within and flips the
   tile into view. */
.cell-art {
    flex: 0 0 auto;
    width: 9rem;
    height: 9rem;
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
    padding: 0.25rem;
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

/* --- Info column: room for the full filename, note and metadata line. --- */
.cell-info {
    flex: 1 1 auto;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
}
.cell-title {
    font-size: 0.85rem;
    font-weight: 600;
    overflow-wrap: anywhere;
}
.cell-sub {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
    overflow-wrap: anywhere;
}
.cell-note {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
    overflow-wrap: anywhere;
}
.cell-meta {
    font-size: 0.72rem;
    color: var(--app-text-secondary);
    font-variant-numeric: tabular-nums;
    overflow-wrap: anywhere;
}
</style>
