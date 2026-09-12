<script setup lang="ts">
import Button from 'primevue/button'
import ButtonGroup from 'primevue/buttongroup'

// The queue's three header actions (edit / save / clear), rendered in one of two
// shapes from a single set of buttons:
//
//  • Toolbar (the full Now Playing header and the queue side panel): a single
//    segmented ButtonGroup — bordered, shared dividers, no gaps — so the trio
//    reads as one control. Matches the app's existing "button group" language
//    (the List/Grid + tabs group, .as-button-group in _main.scss).
//  • ⋮ overflow menu (QueuePanel on phones, `labels`): a plain column of labeled
//    text rows. A horizontal segmented group would not read as a menu, and
//    tooltips don't exist on touch, so the actions are spelled out instead.
//
// Buttons keep the default size/severity in the group; the side panel passes
// size="small" to fit its tighter header.
defineProps<{ editMode: boolean; disabled: boolean; size?: 'small'; labels?: boolean }>()
defineEmits<{ (e: 'toggle-edit'): void; (e: 'save'): void; (e: 'clear'): void }>()
</script>

<template>
    <component
        :is="labels ? 'div' : ButtonGroup"
        :class="labels ? 'queue-actions-menu' : 'queue-actions-group'"
    >
        <Button
            class="queue-action-edit"
            icon="pi pi-pencil"
            :text="labels"
            :rounded="labels"
            :outlined="!labels"
            :severity="labels ? undefined : 'secondary'"
            :size="size"
            :class="{ 'is-active': editMode, 'is-labeled': labels }"
            :label="labels ? (editMode ? 'Done editing' : 'Edit queue') : undefined"
            :aria-label="labels ? undefined : editMode ? 'Done editing' : 'Edit queue'"
            :aria-pressed="editMode"
            :disabled="disabled"
            v-tooltip.bottom="labels ? undefined : editMode ? 'Done editing' : 'Edit queue'"
            @click="$emit('toggle-edit')"
        />
        <Button
            class="queue-action-save"
            icon="pi pi-save"
            :text="labels"
            :rounded="labels"
            :outlined="!labels"
            :severity="labels ? undefined : 'secondary'"
            :size="size"
            :class="{ 'is-labeled': labels }"
            :label="labels ? 'Save as playlist' : undefined"
            :aria-label="labels ? undefined : 'Save as playlist'"
            :disabled="disabled"
            v-tooltip.bottom="labels ? undefined : 'Save as playlist'"
            @click="$emit('save')"
        />
        <Button
            class="queue-action-clear"
            icon="pi pi-eraser"
            :text="labels"
            :rounded="labels"
            :outlined="!labels"
            :severity="labels ? undefined : 'secondary'"
            :size="size"
            :class="{ 'is-labeled': labels }"
            :label="labels ? 'Clear queue' : undefined"
            :aria-label="labels ? undefined : 'Clear queue'"
            :disabled="disabled"
            v-tooltip.bottom="labels ? undefined : 'Clear queue'"
            @click="$emit('clear')"
        />
    </component>
</template>

<style scoped>
/* --- Menu variant (⋮ overflow) --- */

/* Labeled entries stack in a column, same as the scaffold's overflow panel that
   hosts them (QueuePanel), so icons and text line up down the list. */
.queue-actions-menu {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
}

/* Menu rows, not toolbar buttons: labeled entries left-align and fill the width
   so icons and text line up down the stack. */
.is-labeled {
    justify-content: flex-start;
    width: 100%;
}

/* The pencil is a toggle (unlike the one-shot header actions elsewhere), so
   edit mode gets a soft accent fill to read as "pressed". */
.queue-action-edit.is-active {
    background: var(--app-accent-soft);
}

/* --- Toolbar variant (segmented button group) --- */

/* PrimeVue's ButtonGroup ships no styling in this preset (it only sets the
   class), so the segmented look is built here to match the app's other header
   groups (.as-button-group: List/Grid, tabs) — same 2.125rem cells, grey
   inactive icons, accent only on the active one. ContentScaffold's lone-icon
   accent treatment deliberately skips grouped buttons, so the group owns its
   full look. inline-flex is also what joins the trio: it drops the inline-block
   whitespace that otherwise gaps the buttons apart. */
.queue-actions-group {
    display: inline-flex;
    align-items: stretch;
}

/* A neutral segmented control on the header surface: grey inactive cells,
   bordered like the SelectButton groups beside it. */
.queue-actions-group :deep(.p-button) {
    height: 2.125rem;
    padding: 0 0.9rem;
    background: var(--app-surface-2);
    border: 1px solid var(--app-border);
    border-radius: 6px;
    color: var(--app-text-secondary);
}

.queue-actions-group :deep(.p-button:not(:disabled):hover) {
    background: color-mix(in srgb, var(--app-accent) 7%, var(--app-surface-2));
    border-color: var(--app-border);
    color: var(--app-text-primary);
}

/* Match the segmented tabs' icon size (0.95rem), and follow the cell colour. */
.queue-actions-group :deep(.p-button-icon) {
    font-size: 0.95rem;
    color: inherit;
}

/* Square the touching corners and overlap the shared 1px border (margin-left
   -1px) so adjacent cells read as one divider, not a gap or a double line. The
   outer ends keep the button's own radius. */
.queue-actions-group :deep(.p-button:not(:first-child)) {
    border-top-left-radius: 0;
    border-bottom-left-radius: 0;
    margin-left: -1px;
}

.queue-actions-group :deep(.p-button:not(:last-child)) {
    border-top-right-radius: 0;
    border-bottom-right-radius: 0;
}

/* Lift the hovered / active cell so its full border sits above the neighbour it
   overlaps, instead of being clipped by the -1px margin. */
.queue-actions-group :deep(.p-button:not(:disabled):hover),
.queue-actions-group :deep(.queue-action-edit.is-active) {
    position: relative;
    z-index: 1;
}

/* The edit toggle is the one cell that can be "on": it carries the same soft
   accent tint + accent text the segmented tabs use for their active option. */
.queue-actions-group :deep(.queue-action-edit.is-active) {
    background: color-mix(in srgb, var(--app-accent) 12%, var(--app-surface-2));
    border-color: var(--app-border);
    color: var(--app-accent);
}

.queue-actions-group :deep(.queue-action-edit.is-active:not(:disabled):hover) {
    background: color-mix(in srgb, var(--app-accent) 20%, var(--app-surface-2));
}
</style>
