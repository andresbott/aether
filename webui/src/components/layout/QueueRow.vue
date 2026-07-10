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
    playing?: boolean
}>()

const emit = defineEmits<{
    play: []
    select: [payload: { additive: boolean; range: boolean }]
    togglePlay: []
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
    // The current track isn't part of the selectable set (it carries a play
    // toggle, not a checkbox), so clicking its body does nothing.
    if (props.current) return
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
        <span class="row-index">
            <!-- The current track shows a play/pause toggle in place of the
                 selection checkbox; it isn't selectable. -->
            <button
                v-if="current"
                type="button"
                class="current-play-toggle"
                :aria-label="playing ? 'Pause' : 'Play'"
                @click.stop="emit('togglePlay')"
            >
                <i :class="playing ? 'pi pi-pause' : 'pi pi-play'"></i>
            </button>
            <!-- Selection indicator only: pointer-events are disabled so a click
                 falls through to the row handler (preserving ctrl/shift), and it
                 carries no handler of its own. The row's aria-selected conveys
                 state, so the box is hidden from assistive tech. -->
            <input
                v-else
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

.row-index {
    width: 1.75rem;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
}

.row-checkbox {
    width: 1rem;
    height: 1rem;
    margin: 0;
    accent-color: var(--app-accent);
    /* Indicator only — let the click reach the row's select handler. */
    pointer-events: none;
}

.current-play-toggle {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    border: none;
    background: none;
    cursor: pointer;
    color: var(--app-accent);
    font-size: 0.9rem;
    transition: transform 0.15s;
}

.current-play-toggle:hover {
    transform: scale(1.15);
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
