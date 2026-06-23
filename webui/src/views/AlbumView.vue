<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import { useAlbum, useToggleStar } from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { useAlbumDrag } from '@/composables/useAlbumDrag'
import { subsonicClient } from '@/lib/api/subsonic'

const props = defineProps<{ id: string }>()
const router = useRouter()
const player = usePlayer()
const toggleStar = useToggleStar()
const albumDrag = useAlbumDrag()

const onAlbumDragStart = (event: DragEvent): void => {
    if (album.value) albumDrag.start(event, album.value, coverUrl.value)
}

const handleStar = () => {
    if (!album.value) return
    toggleStar.mutate({ id: album.value.id, starred: !!album.value.starred })
}

const { data: album, isLoading, error } = useAlbum(props.id)

const coverUrl = computed(() => {
    if (!album.value?.coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(album.value.coverArt, 250)
})

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const totalDuration = computed(() => {
    if (!album.value?.duration) return ''
    const mins = Math.floor(album.value.duration / 60)
    return `${mins} min`
})

const playAlbum = () => {
    if (album.value?.song) {
        player.playAlbum(album.value.song)
    }
}

const addToQueue = () => {
    if (album.value?.song) {
        player.addMultipleToQueue(album.value.song)
    }
}

const playFromTrack = (index: number) => {
    if (album.value?.song) {
        player.playAlbum(album.value.song, index)
    }
}
</script>

<template>
    <div class="album-view">
        <Button icon="pi pi-arrow-left" text rounded @click="router.back()" />

        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="error" class="error">
            <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
            <p>{{ error.message }}</p>
        </div>

        <div v-else-if="album" class="album-content">
            <div class="album-header">
                <div class="album-cover">
                    <img v-if="coverUrl" :src="coverUrl" :alt="album.name" />
                    <div v-else class="cover-placeholder">
                        <i class="pi pi-music" style="font-size: 3rem"></i>
                    </div>
                </div>
                <div class="album-info">
                    <h1>{{ album.name }}</h1>
                    <router-link
                        v-if="album.artistId"
                        :to="{ name: 'artist', params: { id: album.artistId } }"
                        class="artist-link"
                    >
                        {{ album.artist }}
                    </router-link>
                    <p v-else class="artist-name">{{ album.artist }}</p>
                    <p class="album-meta">
                        <span v-if="album.year">{{ album.year }}</span>
                        <span v-if="album.songCount">{{ album.songCount }} songs</span>
                        <span v-if="totalDuration">{{ totalDuration }}</span>
                    </p>
                    <div class="album-actions">
                        <Button label="Play" icon="pi pi-play" @click="playAlbum" />
                        <Button
                            label="Add to Queue"
                            icon="pi pi-plus"
                            severity="secondary"
                            text
                            @click="addToQueue"
                        />
                        <Button
                            :icon="album?.starred ? 'pi pi-star-fill' : 'pi pi-star'"
                            text
                            rounded
                            @click="handleStar"
                        />
                        <span
                            class="album-drag-handle"
                            draggable="true"
                            v-tooltip.bottom="'Drag album to queue'"
                            @dragstart="onAlbumDragStart"
                            @dragend="albumDrag.end"
                        >
                            <i class="pi pi-bars"></i>
                        </span>
                    </div>
                </div>
            </div>

            <DataTable
                v-if="album.song && album.song.length > 0"
                :value="album.song"
                stripedRows
                @row-click="(e) => playFromTrack(e.index)"
                class="track-table"
                :rowClass="() => 'clickable-row'"
            >
                <Column field="track" header="#" style="width: 60px" />
                <Column field="title" header="Title" />
                <Column field="artist" header="Artist" />
                <Column header="Duration" style="width: 80px">
                    <template #body="{ data }">
                        {{ formatDuration(data.duration) }}
                    </template>
                </Column>
            </DataTable>
        </div>
    </div>
</template>

<style scoped>
.album-view {
    max-width: 1200px;
    margin: 0 auto;
}

.loading,
.error {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 3rem;
    gap: 1rem;
    color: var(--app-text-secondary);
}

.error {
    color: #ef4444;
}

.album-header {
    display: flex;
    gap: 2rem;
    margin: 1.5rem 0 2rem;
}

.album-cover {
    width: 250px;
    height: 250px;
    flex-shrink: 0;
    border-radius: 8px;
    overflow: hidden;
}

.album-cover img {
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

.album-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.album-info h1 {
    font-size: 2.5rem;
    font-weight: 700;
    margin: 0;
    line-height: 1.2;
}

.artist-link {
    font-size: 1.25rem;
    color: var(--app-accent);
}

.artist-name {
    font-size: 1.25rem;
    color: var(--app-text-secondary);
    margin: 0;
}

.album-meta {
    display: flex;
    gap: 0.75rem;
    color: var(--app-text-secondary);
    font-size: 0.95rem;
    margin: 0;
}

.album-meta span:not(:last-child)::after {
    content: '\00b7';
    margin-left: 0.75rem;
}

.album-actions {
    display: flex;
    gap: 1rem;
    margin-top: auto;
}

.track-table :deep(.clickable-row) {
    cursor: pointer;
}

.track-table :deep(.clickable-row:hover) {
    background-color: #f9fafb !important;
}

.album-drag-handle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 2.25rem;
    height: 2.25rem;
    color: var(--app-text-secondary);
    cursor: grab;
}

.album-drag-handle:active {
    cursor: grabbing;
}
</style>
