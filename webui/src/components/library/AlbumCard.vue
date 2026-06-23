<script setup lang="ts">
import { computed } from 'vue'
import type { Album } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { useAlbumDrag } from '@/composables/useAlbumDrag'

const props = defineProps<{
    album: Album
}>()

const albumDrag = useAlbumDrag()

const coverUrl = computed(() => {
    if (!props.album.coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(props.album.coverArt, 200)
})

const onCardDragStart = (event: DragEvent): void => {
    albumDrag.start(event, props.album, coverUrl.value)
}
</script>

<template>
    <router-link
        :to="{ name: 'album', params: { id: album.id } }"
        class="album-card"
        draggable="true"
        @dragstart="onCardDragStart"
        @dragend="albumDrag.end"
    >
        <div class="card-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="album.name" draggable="false" />
            <div v-else class="cover-placeholder">
                <i class="pi pi-music" style="font-size: 2rem"></i>
            </div>
        </div>
        <div class="card-info">
            <div class="card-title">{{ album.name }}</div>
            <div class="card-subtitle">{{ album.artist }}</div>
        </div>
    </router-link>
</template>

<style scoped>
.album-card {
    display: flex;
    flex-direction: column;
    text-decoration: none;
    color: inherit;
    border-radius: 8px;
    overflow: hidden;
    transition: transform 0.2s, box-shadow 0.2s;
    cursor: pointer;
}

.album-card:hover {
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

.card-year {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}
</style>
