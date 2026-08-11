<script setup lang="ts">
/**
 * The multi-select toggle for one track row — the mouse-only counterpart to
 * CTRL/⌘+click, sitting immediately left of the row's `TrackFavoriteButton`.
 * Clicking it toggles just this row in or out of the selection, so several
 * songs can be picked without a modifier and without a plain click replacing
 * the set. Rendered by every selectable track row (`AlbumTrackRow`,
 * `GenreTrackRow`).
 *
 * Its root keeps the `.row-select` class the host rows reveal on hover
 * (`.<row>:hover .row-select { opacity: 1 }`), mirroring how `.row-star`
 * works: hover semantics belong to the row, the icon and wording belong here.
 *
 * The click is both `preventDefault`ed and `stopPropagation`ed for the same
 * reason as the heart: an unhandled click would fall through to the row and
 * replace the selection instead of toggling this one row.
 */
defineProps<{ selected?: boolean }>()

const emit = defineEmits<{ toggle: [] }>()

const onClick = (event: Event): void => {
    event.preventDefault()
    event.stopPropagation()
    emit('toggle')
}
</script>

<template>
    <button
        class="row-select"
        :class="{ 'is-selected': selected }"
        type="button"
        :aria-pressed="!!selected"
        :aria-label="selected ? 'Deselect track' : 'Select track'"
        @click="onClick"
        @dblclick.stop.prevent
    >
        <i :class="selected ? 'pi pi-check-circle' : 'pi pi-circle'"></i>
    </button>
</template>

<style scoped>
/*
 * Hidden until the row is hovered — except when the row IS selected, where it
 * stays visible so the picked set reads at a glance even with the pointer
 * elsewhere. The reveal-on-hover half lives in each host row (it needs that
 * row's class); everything that must look the same everywhere lives here.
 */
.row-select {
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: none;
    padding: 0;
    line-height: 1;
    color: var(--app-text-secondary);
    font-size: 1rem;
    cursor: pointer;
    opacity: 0;
    transition:
        opacity 0.15s,
        color 0.15s;
}

/* A selected row already carries the accent-soft tint, so the check reads as
   state without a second accent colour competing with it. */
.row-select.is-selected {
    opacity: 1;
    color: var(--app-text-primary);
}

.row-select:hover,
.row-select:focus-visible {
    opacity: 1;
    color: var(--app-text-primary);
}
</style>
