<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import TrackFavoriteButton from '@/components/library/TrackFavoriteButton.vue'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Song } from '@/types/subsonic'

const props = defineProps<{
    song: Song
    queueIndex: number
    editing?: boolean
    selected?: boolean
    current?: boolean
    deleteLabel?: string
    // Lay the artist out as its own column beside the title instead of
    // stacked under it (used by the full Now Playing view; the sidebar
    // keeps the compact stacked layout).
    artistColumn?: boolean
}>()

const emit = defineEmits<{
    play: []
    select: [payload: { additive: boolean; range: boolean }]
    delete: []
}>()

const hovered = ref(false)

const position = computed(() => props.queueIndex + 1)

const coverUrl = computed<string | null>(() => {
    if (!props.song.coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(props.song.coverArt, 48)
})

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const onRowClick = (event: MouseEvent): void => {
    emit('select', { additive: event.ctrlKey || event.metaKey, range: event.shiftKey })
}

// The view-mode row is a `role="button"` div rather than a real <button>, because
// it now contains the favorite toggle and nested buttons are invalid HTML (and
// swallow clicks unpredictably). Enter/Space are therefore wired by hand — a
// real button got them for free.
const onRowKeydown = (event: KeyboardEvent): void => {
    if (event.key !== 'Enter' && event.key !== ' ') return
    event.preventDefault()
    emit('play')
}
</script>

<template>
    <div
        v-if="editing"
        class="queue-row queue-row--editing"
        :class="{ selected, 'queue-row--current': current }"
        role="option"
        :aria-selected="selected"
        :data-queue-index="queueIndex"
        @click="onRowClick"
    >
        <!-- The whole index cell is the checkbox hit target — bigger and more
             reliable than the bare native box. Clicking it toggles this row in/out
             of the selection additively, so several songs can be picked without a
             modifier and without the row body's plain click replacing the set.
             stop keeps the row's own click handler from also firing; the box is a
             pointer-events-free indicator driven by `selected`, and the row's
             aria-selected conveys state to assistive tech. The now-playing row is
             an ordinary selectable row here — no playback control in edit mode. -->
        <span
            class="row-index row-index--checkbox"
            @click.stop="emit('select', { additive: true, range: false })"
        >
            <input
                class="row-checkbox"
                type="checkbox"
                :checked="selected"
                tabindex="-1"
                aria-hidden="true"
            />
        </span>
        <span class="row-cover">
            <img v-if="coverUrl" :src="coverUrl" alt="" />
            <i v-else class="pi pi-music"></i>
        </span>
        <span class="row-info" :class="{ 'row-info--columns': artistColumn }">
            <span class="row-title">{{ song.title }}</span>
            <span class="row-artist">{{ song.artist || 'Unknown' }}</span>
        </span>
        <span class="row-end row-end--editing">
            <span class="drag-handle" @click.stop v-tooltip.left="'Drag to reorder'">
                <i class="pi pi-bars"></i>
            </span>
            <Button
                icon="pi pi-trash"
                text
                rounded
                size="small"
                severity="secondary"
                class="delete-button"
                v-tooltip.left="deleteLabel ?? 'Remove from queue'"
                @click.stop="emit('delete')"
            />
        </span>
    </div>

    <!-- A div with role="button", not a <button>: the favorite toggle is a button
         itself, and nesting one inside another is invalid HTML. onRowKeydown
         restores the Enter/Space activation a real button provided. Edit mode
         above keeps its own row and gets no heart — that mode is for reordering
         and removal. -->
    <div
        v-else
        role="button"
        tabindex="0"
        class="queue-row"
        :class="{ 'queue-row--columns': artistColumn }"
        :data-queue-index="queueIndex"
        @click="emit('play')"
        @keydown="onRowKeydown"
        @mouseenter="hovered = true"
        @mouseleave="hovered = false"
    >
        <span class="row-index">
            <i v-if="hovered" class="pi pi-play play-hover-icon"></i>
            <span v-else class="track-number">{{ position }}</span>
        </span>
        <span class="row-cover">
            <img v-if="coverUrl" :src="coverUrl" alt="" />
            <i v-else class="pi pi-music"></i>
        </span>
        <span class="row-info" :class="{ 'row-info--columns': artistColumn }">
            <span class="row-title">{{ song.title }}</span>
            <span class="row-artist">{{ song.artist || 'Unknown' }}</span>
        </span>
        <!-- Two separate cells, not one flex group: the heart and the duration are
             each their own fixed-width column, so they line up down the list
             instead of drifting with each duration's text width. -->
        <span class="row-star-cell"><TrackFavoriteButton :song="song" /></span>
        <span class="row-end">
            <span class="row-duration">{{ formatDuration(song.duration) }}</span>
        </span>
    </div>
</template>

<style scoped>
.queue-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    width: 100%;
    padding: 0.4rem 0.5rem;
    border: none;
    background: none;
    cursor: pointer;
    text-align: left;
    transition: background-color 0.15s;
    box-sizing: border-box;
}

.queue-row:hover {
    background-color: var(--app-hover);
}

/* The view-mode row is a focusable div (see template), so it needs the focus
   ring a real <button> would have drawn. */
.queue-row:focus-visible {
    outline: 2px solid var(--app-accent);
    outline-offset: -2px;
}

.queue-row--editing {
    cursor: default;
    /* SHIFT+click selects a range of rows; suppress the browser's text
       highlighting that the shift drag would otherwise produce. */
    user-select: none;
}

.queue-row--editing.selected {
    background-color: var(--app-accent-soft);
}

/* The now-playing row is selectable like any other in edit mode; a thin accent
   bar on its left edge is the only hint of which track is playing. */
.queue-row--editing.queue-row--current {
    box-shadow: inset 3px 0 0 var(--app-accent);
}

.row-index {
    width: 1.75rem;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
}

/* The cell, not the box, receives the click (see template). */
.row-index--checkbox {
    cursor: pointer;
}

.row-checkbox {
    width: 1rem;
    height: 1rem;
    margin: 0;
    accent-color: var(--app-accent);
    /* Indicator only — let the click reach the cell handler. */
    pointer-events: none;
}

.track-number {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    font-weight: 500;
}

.play-hover-icon {
    color: var(--app-text-primary);
    font-size: 1rem;
}

.row-cover {
    width: 40px;
    height: 40px;
    flex-shrink: 0;
    border-radius: 4px;
    overflow: hidden;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    display: flex;
    align-items: center;
    justify-content: center;
}

.row-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.row-cover i {
    font-size: 0.9rem;
    color: rgba(255, 255, 255, 0.85);
}

.row-info {
    min-width: 0;
    flex: 1;
    display: flex;
    flex-direction: column;
}

/* Column layout (full Now Playing view): the artist sits in its own column
   beside the title instead of stacked under it. */
.row-info--columns {
    flex-direction: row;
    align-items: center;
    gap: 0.75rem;
}

.row-info--columns .row-title {
    flex: 2.4;
    min-width: 0;
}

.row-info--columns .row-artist {
    flex: 1.4;
    min-width: 0;
    font-size: 0.85rem;
}

.row-title {
    font-size: 0.9rem;
    font-weight: 500;
    color: var(--app-text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.row-artist {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

/* The favorite toggle's own column, sized like the album/genre views' star column
   so every track list in the app puts its heart at the same width. Separated from
   the duration by the row gap, which is what makes it read as a column.
   `queue-row--columns` widens it in the full Now Playing view, where the row is a
   real multi-column table (artist beside the title) and the tighter sidebar
   spacing reads as two glyphs crowded together rather than as columns. */
.row-star-cell {
    width: 2rem;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
}

.row-end {
    position: relative;
    width: 40px;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: flex-end;
}

/* Full Now Playing view: wider gutters around the star column so it reads as its
   own column in a row that already has three text columns. */
.queue-row--columns .row-star-cell {
    width: 3rem;
    margin-left: 0.75rem;
}

.queue-row--columns .row-end {
    width: 56px;
}

.queue-row:hover :deep(.row-star),
.queue-row:focus-visible :deep(.row-star) {
    opacity: 1;
}

/* Touch has no hover: the shared rows expose the heart permanently there. Same
   rule AlbumTrackRow and GenreTrackRow carry — without it the queue's hearts were
   invisible on touch, since neither hover nor focus-visible ever fires.
   :deep because the opacity lives on the button component's own class. */
@media (pointer: coarse) {
    .row-star-cell :deep(.row-star) {
        opacity: 1;
    }
}

/* Edit mode carries the drag handle and delete instead of heart + duration, and
   sizes to them rather than to the fixed view-mode width. */
.row-end--editing {
    gap: 0.25rem;
    width: auto;
}

.row-duration {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}

.drag-handle {
    display: flex;
    align-items: center;
    color: var(--app-text-secondary);
    cursor: grab;
    padding: 0.25rem;
}

.drag-handle:active {
    cursor: grabbing;
}
</style>
