<script setup lang="ts">
import Button from 'primevue/button'

// The edit-mode counterpart to HeroSelectionBar: a frosted pill that sits inside
// the hero band, below the name/description form, whenever tracks are selected in
// the reorder list. Where HeroSelectionBar retargets the item's own actions
// (Play/Queue/Add to playlist), this one carries the editor's reorder + delete
// actions for the selection. Self-contained chrome (its own pill) because it is
// slotted into the edit form column, not the shared .hero-actions-slot.
defineProps<{ count: number }>()

defineEmits<{
    (e: 'move-top'): void
    (e: 'move-bottom'): void
    (e: 'delete'): void
    (e: 'clear'): void
}>()
</script>

<template>
    <div class="edit-select" role="toolbar" aria-label="Selected tracks">
        <Button
            class="es-top"
            label="Move to top"
            icon="pi pi-angle-double-up"
            severity="secondary"
            text
            @click="$emit('move-top')"
        />
        <Button
            class="es-bottom"
            label="Move to bottom"
            icon="pi pi-angle-double-down"
            severity="secondary"
            text
            @click="$emit('move-bottom')"
        />
        <Button
            class="es-delete"
            label="Delete"
            icon="pi pi-trash"
            severity="secondary"
            text
            @click="$emit('delete')"
        />
        <span class="es-sep" aria-hidden="true"></span>
        <span class="es-count"><b>{{ count }}</b> selected</span>
        <Button
            class="es-clear"
            icon="pi pi-times"
            severity="secondary"
            text
            rounded
            aria-label="Clear selection"
            v-tooltip.bottom="'Clear selection'"
            @click="$emit('clear')"
        />
    </div>
</template>

<style scoped>
/* The frosted pill, mirroring HeroSelectionBar's .hero-actions-slot chrome so the
   edit bubble reads as the same control family on the dark band. */
.edit-select {
    align-self: flex-start;
    max-width: 100%;
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.35rem;
    padding: 0.3rem 0.55rem;
    background: rgba(255, 255, 255, 0.1);
    border: 1px solid rgba(255, 255, 255, 0.18);
    border-radius: 12px;
    backdrop-filter: blur(8px);
}

/* Compact the buttons to the header controls' size, matching HeroActions/
   HeroSelectionBar in the band. min-height (not height) so a label may wrap to a
   second line when the column is squeezed instead of clipping. */
.edit-select :deep(.p-button) {
    min-height: 2.125rem;
    min-width: 0;
    padding-block: 0.15rem;
    padding-inline: 0.55rem;
    font-size: 0.875rem;
}
/* Match the icon-only clear button to the compact row height of its siblings.
   Without an explicit height it keeps PrimeVue's taller icon-only default (~40px),
   which makes this pill taller than the HeroVisibilityBar pill it swaps with and
   jumps the band on select. */
.edit-select :deep(.p-button-icon-only) {
    width: 2.125rem;
    height: 2.125rem;
    padding: 0;
}
.edit-select :deep(.p-button-icon) {
    font-size: 0.95rem;
}
/* The bubble wraps to a stack when the hero column is narrow (e.g. editing with
   the queue panel open); labels wrap with it rather than clipping. In the common
   full-width case there is room for a single row. */
.edit-select :deep(.p-button-label) {
    white-space: normal;
    text-align: left;
}

/* Ghost (secondary text) buttons stay light on the band. */
.edit-select :deep(.p-button.p-button-secondary.p-button-text) {
    color: #e2edf4;
}
.edit-select :deep(.p-button.p-button-secondary.p-button-text:hover) {
    background: rgba(255, 255, 255, 0.14);
    color: #ffffff;
}

/* Delete reads as the destructive action, matching the hero's Delete ink. */
.edit-select :deep(.es-delete.p-button.p-button-secondary.p-button-text) {
    color: #ff9aa6;
}
.edit-select :deep(.es-delete.p-button.p-button-secondary.p-button-text:hover) {
    background: rgba(255, 120, 130, 0.18);
    color: #ffffff;
}

.es-sep {
    width: 1px;
    height: 22px;
    background: rgba(255, 255, 255, 0.22);
    margin: 0 0.1rem;
}
.es-count {
    font-size: 0.9rem;
    color: #d3dbe3;
    white-space: nowrap;
}
.es-count b {
    color: #8fecff;
    font-weight: 700;
}
</style>
