<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Song } from '@/types/subsonic'

const props = defineProps<{
    song: Song
    queueIndex: number
    editing?: boolean
    selected?: boolean
    current?: boolean
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
        <span class="row-info">
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
                v-tooltip.left="'Delete from queue'"
                @click.stop="emit('delete')"
            />
        </span>
    </div>

    <button
        v-else
        type="button"
        class="queue-row"
        :data-queue-index="queueIndex"
        @click="emit('play')"
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
        <span class="row-info">
            <span class="row-title">{{ song.title }}</span>
            <span class="row-artist">{{ song.artist || 'Unknown' }}</span>
        </span>
        <span class="row-end">
            <span class="row-duration">{{ formatDuration(song.duration) }}</span>
        </span>
    </button>
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

.row-end {
    position: relative;
    width: 56px;
    flex-shrink: 0;
    text-align: right;
}

.row-end--editing {
    display: flex;
    align-items: center;
    justify-content: flex-end;
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
