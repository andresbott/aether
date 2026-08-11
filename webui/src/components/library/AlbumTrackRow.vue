<script setup lang="ts">
import { computed } from 'vue'
import TrackFavoriteButton from '@/components/library/TrackFavoriteButton.vue'
import TrackSelectButton from '@/components/library/TrackSelectButton.vue'
import type { Song } from '@/types/subsonic'

const props = defineProps<{
    song: Song
    index: number
    selected?: boolean
    playing?: boolean
}>()

const emit = defineEmits<{
    select: [payload: { additive: boolean; range: boolean }]
    // A double-click appends the track to the queue; the host decides what that
    // means for playback (see docs/architecture/unified-play-experience.md).
    enqueue: []
    dragstart: [event: DragEvent]
    dragend: []
}>()

// The track's position in its disc, falling back to its position in the flat
// list so a row always shows a number.
const trackNumber = computed(() => props.song.track ?? props.index + 1)

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const onClick = (event: MouseEvent): void => {
    emit('select', { additive: event.ctrlKey || event.metaKey, range: event.shiftKey })
}
</script>

<template>
    <div
        class="album-track-row"
        :class="{ selected, playing, striped: index % 2 === 1 }"
        role="option"
        :aria-selected="selected"
        :data-track-index="index"
        draggable="true"
        @click="onClick"
        @dblclick="emit('enqueue')"
        @dragstart="emit('dragstart', $event)"
        @dragend="emit('dragend')"
    >
        <span class="col-index track-number">
            <i v-if="playing" class="pi pi-volume-up playing-icon"></i>
            <template v-else>{{ trackNumber }}</template>
        </span>
        <!-- .row-title is read by the multi-song drag image builder. -->
        <span class="col-title row-title">{{ song.title }}</span>
        <span class="col-artist">{{ song.artist || 'Unknown' }}</span>
        <!-- Same additive toggle a CTRL/⌘+click performs, for pointer users. -->
        <span class="col-select">
            <TrackSelectButton
                :selected="selected"
                @toggle="emit('select', { additive: true, range: false })"
            />
        </span>
        <span class="col-star"><TrackFavoriteButton :song="song" /></span>
        <span class="col-duration row-duration">{{ formatDuration(song.duration) }}</span>
    </div>
</template>

<style scoped>
.album-track-row {
    display: grid;
    grid-template-columns: var(--album-track-cols);
    align-items: center;
    column-gap: 0.75rem;
    width: 100%;
    /* Match the Library list view's row height (AlbumListView VirtualScroller
       itemSize) so album tracks and library rows line up at the same height. */
    min-height: 56px;
    padding: 0 0.5rem;
    box-sizing: border-box;
    cursor: pointer;
    transition: background-color 0.15s;
    /* SHIFT+click selects a range of rows; suppress the browser's text
       highlighting the shift drag would otherwise produce. */
    user-select: none;
}

/* Zebra striping for the table look — kept subtler than the hover tint so hover
   still reads on striped rows. Order matters: hover overrides the stripe, and
   the selection tint overrides both (equal specificity → later rule wins). */
.album-track-row.striped {
    background-color: rgba(0, 0, 0, 0.025);
}

.album-track-row:hover {
    background-color: var(--app-hover);
}

.album-track-row.selected,
.album-track-row.selected.striped {
    background-color: var(--app-accent-soft);
}

.album-track-row.selected:hover {
    background-color: var(--app-accent-soft-hover);
}

/* Currently-playing track: accent-soft row with cyan index + title, and a
   volume glyph in place of the number. */
.album-track-row.playing,
.album-track-row.playing.striped {
    background-color: var(--app-accent-soft);
}

.album-track-row.playing:hover {
    background-color: var(--app-accent-soft-hover);
}

.album-track-row.playing .track-number,
.album-track-row.playing .playing-icon,
.album-track-row.playing .col-title {
    color: var(--app-accent);
}

.col-index {
    text-align: right;
}

/* The select and favorite toggles are revealed by hovering the row; a selected
   row keeps its check visible (`.is-selected`) and a track that IS a favorite
   keeps its heart visible (`.is-starred`), both styled in their buttons. */
.col-select,
.col-star {
    display: flex;
    align-items: center;
    justify-content: center;
}

.album-track-row:hover .col-select :deep(.row-select),
.album-track-row:hover .col-star :deep(.row-star) {
    opacity: 1;
}

.track-number {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    font-weight: 500;
    font-variant-numeric: tabular-nums;
}

.col-title,
.col-artist {
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.col-title {
    font-size: 0.9rem;
    font-weight: 500;
    color: var(--app-text-primary);
}

.col-artist {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
}

.col-duration {
    text-align: right;
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    font-variant-numeric: tabular-nums;
}
</style>
