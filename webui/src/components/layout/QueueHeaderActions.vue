<script setup lang="ts">
import Button from 'primevue/button'

// The queue's three header actions (edit / save / clear), rendered in one of two
// shapes from a single set of buttons:
//
//  • Toolbar (the full Now Playing header and the queue side panel): three
//    separate square buttons, styled like every other header control — the
//    segmented look is reserved for mutually exclusive choices (List/Grid, tabs),
//    which these are not. Edit is a toggle, so it looks like the standalone
//    favorites / public filters (grey, accent tint when on); save and clear are
//    one-shot actions, so they carry the accent like the views' lone "+" button
//    (ContentScaffold's lone-icon rule).
//  • ⋮ overflow menu (QueuePanel on phones, `labels`): a plain column of labeled
//    text rows. Tooltips don't exist on touch, so the actions are spelled out.
defineProps<{ editMode: boolean; disabled: boolean; size?: 'small'; labels?: boolean }>()
defineEmits<{ (e: 'toggle-edit'): void; (e: 'save'): void; (e: 'clear'): void }>()
</script>

<template>
    <div :class="labels ? 'queue-actions-menu' : 'queue-actions-toolbar'">
        <Button
            class="queue-action-edit"
            icon="ms-edit"
            :text="labels"
            :rounded="labels"
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
            icon="ms-save"
            :text="labels"
            :rounded="labels"
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
            icon="ms-close"
            :text="labels"
            :rounded="labels"
            :size="size"
            :class="{ 'is-labeled': labels }"
            :label="labels ? 'Clear queue' : undefined"
            :aria-label="labels ? undefined : 'Clear queue'"
            :disabled="disabled"
            v-tooltip.bottom="labels ? undefined : 'Clear queue'"
            @click="$emit('clear')"
        />
    </div>
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

/* The pencil is a toggle (unlike the one-shot actions), so edit mode gets a
   soft accent fill to read as "pressed". */
.queue-action-edit.is-labeled.is-active {
    background: var(--app-accent-soft);
}

/* --- Toolbar variant (separate header buttons) --- */

/* Same spacing as ContentScaffold's .scaffold-actions row. */
.queue-actions-toolbar {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}

/* The shared header-control cell: the 2.125rem bordered square on the header
   surface that the .as-button-group toggles and ContentScaffold's lone "+" use.
   Duplicated here (not inherited) because the buttons sit inside this wrapper,
   not directly in .scaffold-actions, and the side panel has no scaffold. */
.queue-actions-toolbar .p-button {
    width: 2.125rem;
    height: 2.125rem;
    padding: 0;
    background: var(--app-surface-2);
    border: 1px solid var(--app-border);
    border-radius: 6px;
    color: var(--app-accent);
}

.queue-actions-toolbar .p-button:not(:disabled):hover {
    background: color-mix(in srgb, var(--app-accent) 12%, var(--app-surface-2));
    border-color: var(--app-border);
    color: var(--app-accent);
}

/* Same icon size as the header toggles (0.95rem), following the button colour. */
.queue-actions-toolbar :deep(.p-button-icon) {
    font-size: 0.95rem;
    color: inherit;
}

/* Edit is a toggle: grey while off, like the favorites / public filters,
   with their 7% hover tint… */
.queue-actions-toolbar .queue-action-edit {
    color: var(--app-text-secondary);
}

.queue-actions-toolbar .queue-action-edit:not(:disabled):hover {
    background: color-mix(in srgb, var(--app-accent) 7%, var(--app-surface-2));
    color: var(--app-text-primary);
}

/* …and their checked look while on: accent tint + accent icon. */
.queue-actions-toolbar .queue-action-edit.is-active {
    background: color-mix(in srgb, var(--app-accent) 12%, var(--app-surface-2));
    color: var(--app-accent);
}

.queue-actions-toolbar .queue-action-edit.is-active:not(:disabled):hover {
    background: color-mix(in srgb, var(--app-accent) 20%, var(--app-surface-2));
    color: var(--app-accent);
}
</style>
