<script setup lang="ts">
import { computed } from 'vue'
import type { InternetRadioStation } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { useSongsDrag } from '@/composables/useSongsDrag'
import { stationToSong } from '@/utils/radioSong'

const props = defineProps<{ station?: InternetRadioStation }>()

const songsDrag = useSongsDrag()

const coverUrl = computed(() => {
    const art = props.station?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 80)
})

const onRowDragStart = (event: DragEvent): void => {
    if (props.station) songsDrag.start(event, [stationToSong(props.station)], event.currentTarget as HTMLElement)
}
</script>

<template>
    <div v-if="!station" class="radio-row placeholder">
        <div class="col-avatar"></div>
        <div class="col-name"></div>
    </div>
    <RouterLink
        v-else
        class="radio-row"
        :to="{ name: 'radio-station-detail', params: { id: station.id } }"
        draggable="true"
        @dragstart="onRowDragStart"
        @dragend="songsDrag.end"
    >
        <div class="col-avatar">
            <img v-if="coverUrl" :src="coverUrl" :alt="station.name" draggable="false" />
            <div v-else class="avatar-placeholder"><i class="pi pi-wifi"></i></div>
        </div>
        <div class="col-name">{{ station.name }}</div>
        <div class="col-homepage">{{ station.homepageUrl }}</div>
    </RouterLink>
</template>

<style scoped>
.radio-row {
    display: grid;
    grid-template-columns: 48px 1fr minmax(0, 1fr);
    align-items: center;
    gap: 1rem;
    height: 56px;
    padding: 0 0.5rem;
    text-decoration: none;
    color: inherit;
    border-bottom: 1px solid var(--p-content-border-color);
    cursor: grab;
    transition: background-color 0.15s;
}

.radio-row:hover:not(.placeholder) {
    background-color: var(--app-hover);
}

.col-avatar {
    width: 40px;
    height: 40px;
    border-radius: 50%;
    overflow: hidden;
}

.col-avatar img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.avatar-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: rgba(255, 255, 255, 0.8);
}

.col-name {
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.col-homepage {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
</style>
