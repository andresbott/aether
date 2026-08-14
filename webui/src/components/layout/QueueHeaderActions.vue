<script setup lang="ts">
import Button from 'primevue/button'

// The queue's three header actions, rendered as a fragment so the buttons sit
// directly in whichever header hosts them: the ContentScaffold #actions slot
// (full Now Playing view) or the sidebar's compact header row. Buttons follow
// the shared header-action convention (plain text+rounded, default
// size/severity — see EditActionBar and the ContentScaffold views); the
// sidebar passes size="small" to fit its tighter header. Inside the
// scaffold's ⋮ overflow panel (MobilePlayView) `labels` spells the actions
// out — a stack of bare icons is not a readable menu, and tooltips don't
// exist on touch.
defineProps<{ editMode: boolean; disabled: boolean; size?: 'small'; labels?: boolean }>()
defineEmits<{ (e: 'toggle-edit'): void; (e: 'save'): void; (e: 'clear'): void }>()
</script>

<template>
    <Button
        class="queue-action-edit"
        icon="pi pi-pencil"
        text
        rounded
        :size="size"
        :class="{ 'is-active': editMode, 'is-labeled': labels }"
        :label="labels ? (editMode ? 'Done editing' : 'Edit queue') : undefined"
        :aria-pressed="editMode"
        :disabled="disabled"
        v-tooltip.bottom="labels ? undefined : editMode ? 'Done editing' : 'Edit queue'"
        @click="$emit('toggle-edit')"
    />
    <Button
        class="queue-action-save"
        icon="pi pi-save"
        text
        rounded
        :size="size"
        :class="{ 'is-labeled': labels }"
        :label="labels ? 'Save as playlist' : undefined"
        :disabled="disabled"
        v-tooltip.bottom="labels ? undefined : 'Save as playlist'"
        @click="$emit('save')"
    />
    <Button
        class="queue-action-clear"
        icon="pi pi-eraser"
        text
        rounded
        :size="size"
        :class="{ 'is-labeled': labels }"
        :label="labels ? 'Clear queue' : undefined"
        :disabled="disabled"
        v-tooltip.bottom="labels ? undefined : 'Clear queue'"
        @click="$emit('clear')"
    />
</template>

<style scoped>
/* The pencil is a toggle (unlike the one-shot header actions elsewhere), so
   edit mode gets a soft accent fill to read as "pressed". */
.queue-action-edit.is-active {
    background: var(--app-accent-soft);
}

/* Menu rows, not toolbar buttons: labeled entries left-align in the overflow
   panel's column so icons and text line up down the stack. */
.is-labeled {
    justify-content: flex-start;
    width: 100%;
}
</style>
