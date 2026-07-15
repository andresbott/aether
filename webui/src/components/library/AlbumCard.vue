<script setup lang="ts">
import { computed } from 'vue'
import type { Album } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { useAlbumDrag } from '@/composables/useAlbumDrag'
import { usePlayer } from '@/composables/usePlayer'

const props = defineProps<{
    album?: Album
}>()

const albumDrag = useAlbumDrag()
const player = usePlayer()

// Play from the card without navigating: album summaries carry no tracks, so
// fetch the full album first, then queue it.
const onPlay = async (event: Event): Promise<void> => {
    event.preventDefault()
    event.stopPropagation()
    if (!props.album) return
    const full = await subsonicClient.getAlbum(props.album.id)
    if (full?.song?.length) player.playAlbum(full.song)
}

const coverUrl = computed(() => {
    const art = props.album?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 200)
})

const onCardDragStart = (event: DragEvent): void => {
    if (props.album) albumDrag.start(event, props.album, coverUrl.value)
}
</script>

<template>
    <div v-if="!album" class="album-card placeholder" aria-hidden="true">
        <div class="card-cover"></div>
        <div class="card-info">
            <div class="card-title"></div>
            <div class="card-subtitle"></div>
        </div>
    </div>
    <router-link
        v-else
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
            <button class="card-play" type="button" aria-label="Play album" @click="onPlay">
                <i class="pi pi-play"></i>
            </button>
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
    position: relative;
    width: 100%;
    aspect-ratio: 1;
    border-radius: 8px;
    overflow: hidden;
}

/* Hover play affordance, bottom-right of the cover. */
.card-play {
    position: absolute;
    right: 8px;
    bottom: 8px;
    width: 40px;
    height: 40px;
    border: none;
    border-radius: 50%;
    background: var(--app-accent);
    color: #fff;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.95rem;
    padding-left: 2px;
    cursor: pointer;
    opacity: 0;
    transform: translateY(6px);
    transition: opacity 0.15s, transform 0.15s, background-color 0.15s;
    box-shadow: 0 3px 10px rgba(0, 0, 0, 0.25);
}

.album-card:hover .card-play {
    opacity: 1;
    transform: translateY(0);
}

.card-play:hover {
    background: var(--app-accent-hover);
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

.album-card.placeholder {
    cursor: default;
}

.album-card.placeholder .card-cover {
    background: var(--app-hover, rgba(127, 127, 127, 0.08));
}

.album-card.placeholder .card-title,
.album-card.placeholder .card-subtitle {
    height: 0.9em;
    margin: 0.15rem 0;
    border-radius: 3px;
    background: var(--app-hover, rgba(127, 127, 127, 0.08));
}
</style>
