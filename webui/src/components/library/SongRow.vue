<script setup lang="ts">
import { computed } from 'vue'
import TrackFavoriteButton from '@/components/library/TrackFavoriteButton.vue'
import TrackSelectButton from '@/components/library/TrackSelectButton.vue'
import type { Song } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { useViewport } from '@/composables/useViewport'

const props = defineProps<{
    song?: Song
    index: number
    selected?: boolean
    playing?: boolean
}>()

const { isTouch } = useViewport()

const emit = defineEmits<{
    select: [payload: { additive: boolean; range: boolean }]
    // A double-click appends the track to the queue; the host decides what that
    // means for playback (see docs/architecture/unified-play-experience.md).
    enqueue: []
    // Touch counterparts (isTouch only): a tap plays this song now, the ⋮
    // opens the host's TrackActionSheet. Pointer users keep select/dblclick.
    play: []
    menu: []
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
    if (isTouch.value) {
        emit('play')
        return
    }
    emit('select', { additive: event.ctrlKey || event.metaKey, range: event.shiftKey })
}

// Keep the row click (selection) from also following the artist/album links.
const onLinkClick = (event: MouseEvent): void => {
    event.stopPropagation()
}
</script>

<template>
    <div v-if="!song" class="song-row placeholder" aria-hidden="true">
        <div class="col-cover"></div>
        <div class="col-title"></div>
    </div>
    <div
        v-else
        class="song-row"
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
        <div class="col-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="song.album" draggable="false" />
            <div v-else class="cover-placeholder"><i class="pi pi-music"></i></div>
        </div>
        <div class="col-title-artist">
            <span class="row-title">
                <i v-if="playing" class="pi pi-volume-up playing-icon"></i>
                {{ song.title }}
            </span>
            <span class="row-artist">
                <router-link
                    v-if="song.artistId"
                    :to="{ name: 'artist', params: { id: song.artistId } }"
                    class="artist-link"
                    @click="onLinkClick"
                >
                    {{ song.artist || 'Unknown' }}
                </router-link>
                <template v-else>{{ song.artist || 'Unknown' }}</template>
            </span>
        </div>
        <span class="col-album">
            <router-link
                v-if="song.albumId"
                :to="{ name: 'album', params: { id: song.albumId } }"
                class="album-link"
                @click="onLinkClick"
            >
                {{ song.album }}
            </router-link>
            <template v-else>{{ song.album }}</template>
        </span>
        <span class="col-duration row-duration">{{ formatDuration(song.duration) }}</span>
        <span class="col-star"><TrackFavoriteButton :song="song" /></span>
    </div>
</template>

<style scoped>
.song-row {
    display: grid;
    grid-template-columns: 48px 1fr 1fr 5rem 2rem;
    align-items: center;
    gap: 1rem;
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
.song-row.striped {
    background-color: rgba(0, 0, 0, 0.025);
}

.song-row:hover:not(.placeholder) {
    background-color: var(--app-hover);
}

.song-row.selected,
.song-row.selected.striped {
    background-color: var(--app-accent-soft);
}

.song-row.selected:hover {
    background-color: var(--app-accent-soft-hover);
}

.song-row.playing,
.song-row.playing.striped {
    background-color: var(--app-accent-soft);
}

.song-row.playing:hover {
    background-color: var(--app-accent-soft-hover);
}

.song-row.playing .playing-icon,
.song-row.playing .row-title {
    color: var(--app-accent);
}

.col-cover {
    width: 48px;
    height: 48px;
    border-radius: 4px;
    overflow: hidden;
    flex-shrink: 0;
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

/* Title + Artist stacked in one cell. */
.col-title-artist {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    min-width: 0;
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
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.col-album {
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: 0.85rem;
    color: var(--app-text-secondary);
}

.artist-link,
.album-link {
    color: inherit;
    text-decoration: none;
}

.artist-link:hover,
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

/* The favorite toggle is revealed by hovering the row; a track that IS a
   favorite keeps its heart visible (`.is-starred`), styled in the button. */
.col-star {
    display: flex;
    align-items: center;
    justify-content: center;
}

.song-row:hover .col-star :deep(.row-star) {
    opacity: 1;
}

/* Touch has no hover: the shared rows expose the heart permanently there.
   :deep because the opacity lives on the button component's own class. */
@media (pointer: coarse) {
    .col-star :deep(.row-star) {
        opacity: 1;
    }
}
</style>
