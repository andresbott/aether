<script setup lang="ts">
import { computed } from 'vue'
import type { Song } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'

const props = defineProps<{
    song?: Song
    index: number
    selected?: boolean
    playing?: boolean
}>()

const emit = defineEmits<{
    select: [payload: { additive: boolean; range: boolean }]
    play: []
    dragstart: [event: DragEvent]
    dragend: []
}>()

const coverUrl = computed(() => {
    const art = props.song?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 80)
})

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const onClick = (event: MouseEvent): void => {
    emit('select', { additive: event.ctrlKey || event.metaKey, range: event.shiftKey })
}

// Keep the row click (selection) from also following the album link.
const onAlbumClick = (event: MouseEvent): void => {
    event.stopPropagation()
}
</script>

<template>
    <div v-if="!song" class="genre-track-row placeholder" aria-hidden="true">
        <div class="col-cover"></div>
        <div class="col-title"></div>
    </div>
    <div
        v-else
        class="genre-track-row"
        :class="{ selected, playing, striped: index % 2 === 1 }"
        role="option"
        :aria-selected="selected"
        :data-track-index="index"
        draggable="true"
        @click="onClick"
        @dblclick="emit('play')"
        @dragstart="emit('dragstart', $event)"
        @dragend="emit('dragend')"
    >
        <div class="col-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="song.album" draggable="false" />
            <div v-else class="cover-placeholder"><i class="pi pi-music"></i></div>
        </div>
        <span class="col-title row-title">
            <i v-if="playing" class="pi pi-volume-up playing-icon"></i>
            {{ song.title }}
        </span>
        <span class="col-artist">{{ song.artist || 'Unknown' }}</span>
        <span class="col-album">
            <router-link
                v-if="song.albumId"
                :to="{ name: 'album', params: { id: song.albumId } }"
                class="album-link"
                @click="onAlbumClick"
            >
                {{ song.album }}
            </router-link>
            <template v-else>{{ song.album }}</template>
        </span>
        <span class="col-duration row-duration">{{ formatDuration(song.duration) }}</span>
    </div>
</template>

<style scoped>
.genre-track-row {
    display: grid;
    grid-template-columns: var(--genre-track-cols);
    align-items: center;
    column-gap: 0.75rem;
    width: 100%;
    min-height: 56px;
    padding: 0 0.5rem;
    box-sizing: border-box;
    cursor: pointer;
    transition: background-color 0.15s;
    /* SHIFT+click selects a range of rows; suppress the browser's text
       highlighting the shift drag would otherwise produce. */
    user-select: none;
}

/* Zebra striping — kept subtler than the hover tint so hover still reads on
   striped rows. Order matters: hover overrides the stripe, and the selection
   tint overrides both (equal specificity → later rule wins). */
.genre-track-row.striped {
    background-color: rgba(0, 0, 0, 0.025);
}

.genre-track-row:hover:not(.placeholder) {
    background-color: var(--app-hover);
}

.genre-track-row.selected,
.genre-track-row.selected.striped {
    background-color: var(--app-accent-soft);
}

.genre-track-row.selected:hover {
    background-color: var(--app-accent-soft-hover);
}

.genre-track-row.playing,
.genre-track-row.playing.striped {
    background-color: var(--app-accent-soft);
}

.genre-track-row.playing:hover {
    background-color: var(--app-accent-soft-hover);
}

.genre-track-row.playing .playing-icon,
.genre-track-row.playing .col-title {
    color: var(--app-accent);
}

.col-cover {
    width: 40px;
    height: 40px;
    border-radius: 4px;
    overflow: hidden;
}

.col-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.cover-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: rgba(255, 255, 255, 0.8);
}

.col-title,
.col-artist,
.col-album {
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

.col-artist,
.col-album {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
}

.album-link {
    color: inherit;
    text-decoration: none;
}

.album-link:hover {
    color: var(--app-accent);
    text-decoration: underline;
}

.col-duration {
    text-align: right;
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    font-variant-numeric: tabular-nums;
}
</style>
