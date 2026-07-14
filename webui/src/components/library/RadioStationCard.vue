<script setup lang="ts">
import { computed } from 'vue'
import type { InternetRadioStation } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { useSongsDrag } from '@/composables/useSongsDrag'
import { stationToSong } from '@/utils/radioSong'

const props = defineProps<{
    station?: InternetRadioStation
}>()

const songsDrag = useSongsDrag()

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
            <div class="card-title"></div>
            <div class="card-subtitle"></div>
        </div>
    </div>
    <div
        v-else
        class="radio-card"
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
            <div class="card-title">{{ station.name }}</div>
            <div class="card-subtitle">
                <template v-if="station.homepageUrl">{{ station.homepageUrl }}</template>
                <template v-else>&nbsp;</template>
            </div>
        </div>
    </div>
</template>

<style scoped>
.radio-card {
    display: flex;
    flex-direction: column;
    text-decoration: none;
    color: inherit;
    border-radius: 8px;
    overflow: hidden;
    transition: transform 0.2s, box-shadow 0.2s;
    cursor: grab;
}

.radio-card:hover {
    transform: translateY(-2px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.card-cover {
    width: 100%;
    aspect-ratio: 1;
    border-radius: 8px;
    overflow: hidden;
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
    padding: 0.5rem 0.25rem;
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
