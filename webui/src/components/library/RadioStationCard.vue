<script setup lang="ts">
import { computed } from 'vue'
import type { InternetRadioStation } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { useSongsDrag } from '@/composables/useSongsDrag'
import { usePlayer } from '@/composables/usePlayer'
import { stationToSong } from '@/utils/radioSong'

const props = defineProps<{
    station?: InternetRadioStation
}>()

const songsDrag = useSongsDrag()
const player = usePlayer()

const onPlay = (event: Event): void => {
    event.preventDefault()
    event.stopPropagation()
    if (props.station) player.playNow(stationToSong(props.station))
}

const coverUrl = computed(() => {
    const art = props.station?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 200)
})

const onCardDragStart = (event: DragEvent): void => {
    if (props.station) songsDrag.start(event, [stationToSong(props.station)], event.currentTarget as HTMLElement)
}
</script>

<template>
    <div v-if="!station" class="radio-card placeholder" aria-hidden="true">
        <div class="card-cover"></div>
        <div class="card-info">
            <div class="card-text">
                <div class="card-title"></div>
                <div class="card-subtitle"></div>
            </div>
        </div>
    </div>
    <RouterLink
        v-else
        class="radio-card"
        :to="{ name: 'radio-station-detail', params: { id: station.id } }"
        draggable="true"
        @dragstart="onCardDragStart"
        @dragend="songsDrag.end"
    >
        <div class="card-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="station.name" draggable="false" />
            <div v-else class="cover-placeholder">
                <i class="pi pi-wifi" style="font-size: 2rem"></i>
            </div>
        </div>
        <div class="card-info">
            <div class="card-text">
                <div class="card-title">{{ station.name }}</div>
                <div class="card-subtitle">
                    <template v-if="station.homepageUrl">{{ station.homepageUrl }}</template>
                    <template v-else>&nbsp;</template>
                </div>
            </div>
            <button class="card-play" type="button" aria-label="Play station" @click="onPlay">
                <i class="pi pi-play"></i>
            </button>
        </div>
    </RouterLink>
</template>

<style scoped>
.radio-card {
    position: relative;
    display: flex;
    flex-direction: column;
    text-decoration: none;
    color: inherit;
    /* Transparent border reserved so the hover border never shifts layout. */
    border: 1px solid transparent;
    border-radius: 10px;
    padding: 0.5rem;
    transition: border-color 0.2s, background 0.2s, box-shadow 0.2s;
    cursor: pointer;
}

/* Border + accent tint wrap the whole card (cover + text) on hover. */
.radio-card:hover {
    border-color: var(--app-accent);
    background: var(--app-accent-soft);
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.12);
}

.card-cover {
    width: 100%;
    aspect-ratio: 1;
    border-radius: 8px;
    overflow: hidden;
    background: var(--app-bg-subtle);
}

/* Simple play icon spanning the height of both text lines, dimmed until hover —
   a card whose actions only appear on hover doesn't advertise that it has any. */
.card-play {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: none;
    padding: 0 0.15rem;
    line-height: 1;
    color: var(--app-text-secondary);
    font-size: 2rem;
    cursor: pointer;
    opacity: 0.4;
    transition: opacity 0.15s, color 0.15s;
}

.radio-card:hover .card-play {
    opacity: 1;
}

.card-play:hover {
    color: var(--app-accent);
}

.card-cover img {
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

.card-info {
    display: flex;
    align-items: stretch;
    gap: 0.5rem;
    padding: 0.5rem 0.15rem 0.1rem;
}

.card-text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
}

.card-title {
    font-size: 0.9rem;
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.card-subtitle {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.radio-card.placeholder {
    cursor: default;
}

.radio-card.placeholder .card-cover {
    background: var(--app-hover, rgba(127, 127, 127, 0.08));
}

.radio-card.placeholder .card-title,
.radio-card.placeholder .card-subtitle {
    height: 0.9em;
    margin: 0.15rem 0;
    border-radius: 3px;
    background: var(--app-hover, rgba(127, 127, 127, 0.08));
}
</style>
