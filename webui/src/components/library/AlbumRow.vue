<script setup lang="ts">
import { computed } from 'vue'
import type { Album } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { useAlbumDrag } from '@/composables/useAlbumDrag'
import { formatDuration } from '@/utils/formatDuration'

const props = defineProps<{ album?: Album }>()
const albumDrag = useAlbumDrag()

const coverUrl = computed(() => {
    const art = props.album?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 80)
})

function onDragStart(event: DragEvent): void {
    if (props.album) albumDrag.start(event, props.album, coverUrl.value)
}
</script>

<template>
    <div v-if="!album" class="album-row placeholder">
        <div class="col-cover"></div>
        <div class="col-title"></div>
    </div>
    <router-link
        v-else
        :to="{ name: 'album', params: { id: album.id } }"
        class="album-row"
        draggable="true"
        @dragstart="onDragStart"
        @dragend="albumDrag.end"
    >
        <div class="col-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="album.name" draggable="false" />
            <div v-else class="cover-placeholder"><i class="pi pi-music"></i></div>
        </div>
        <div class="col-title">{{ album.name }}</div>
        <div class="col-artist">{{ album.artist }}</div>
        <div class="col-songs">{{ album.songCount ?? '' }}</div>
        <div class="col-duration">{{ formatDuration(album.duration) }}</div>
    </router-link>
</template>

<style scoped>
.album-row {
    display: grid;
    grid-template-columns: 48px 2fr 1.5fr 4rem 5rem;
    align-items: center;
    gap: 1rem;
    height: 56px;
    padding: 0 0.5rem;
    text-decoration: none;
    color: inherit;
    border-bottom: 1px solid var(--p-content-border-color);
    cursor: pointer;
    transition: background-color 0.15s;
}

.album-row:hover:not(.placeholder) {
    background-color: var(--app-hover);
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

.col-title {
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.col-artist {
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.col-songs,
.col-duration {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
    text-align: right;
}
</style>
