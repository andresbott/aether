<script setup lang="ts">
import { computed } from 'vue'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import type { Song } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'

const props = defineProps<{
    song: Song
    showBackButton?: boolean
}>()

const emit = defineEmits<{
    back: []
    play: []
}>()

const coverArtUrl = computed(() => {
    if (!props.song.coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(props.song.coverArt, 400)
})

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const formatFileSize = (bytes?: number): string => {
    if (!bytes) return ''
    const mb = bytes / (1024 * 1024)
    return `${mb.toFixed(1)} MB`
}
</script>

<template>
    <div class="song-detail">
        <Button
            v-if="showBackButton"
            icon="pi pi-arrow-left"
            text
            rounded
            class="back-btn"
            @click="emit('back')"
        />

        <div class="detail-layout">
            <div class="cover-art">
                <img v-if="coverArtUrl" :src="coverArtUrl" :alt="song.title" />
                <div v-else class="cover-placeholder">
                    <i class="pi pi-music" style="font-size: 4rem"></i>
                </div>
            </div>

            <div class="song-info">
                <h1>{{ song.title }}</h1>
                <p class="artist">{{ song.artist }}</p>
                <router-link
                    v-if="song.albumId"
                    :to="{ name: 'album', params: { id: song.albumId } }"
                    class="album-link"
                >
                    {{ song.album }}
                </router-link>

                <div class="meta">
                    <span v-if="song.year">{{ song.year }}</span>
                    <Tag v-if="song.genre" :value="song.genre" severity="secondary" />
                    <span v-if="song.duration">{{ formatDuration(song.duration) }}</span>
                    <span v-if="song.bitRate">{{ song.bitRate }} kbps</span>
                    <span v-if="song.size">{{ formatFileSize(song.size) }}</span>
                    <span v-if="song.contentType">{{ song.contentType }}</span>
                </div>

                <div class="actions">
                    <Button
                        label="Play"
                        icon="pi pi-play"
                        @click="emit('play')"
                    />
                </div>
            </div>
        </div>
    </div>
</template>

<style scoped>
.song-detail {
    max-width: 1000px;
    margin: 0 auto;
}

.back-btn {
    margin-bottom: 1rem;
}

.detail-layout {
    display: flex;
    gap: 2.5rem;
}

.cover-art {
    width: 400px;
    height: 400px;
    flex-shrink: 0;
    border-radius: 8px;
    overflow: hidden;
}

.cover-art img {
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

.song-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.song-info h1 {
    font-size: 2.5rem;
    font-weight: 700;
    margin: 0;
    line-height: 1.2;
}

.artist {
    font-size: 1.25rem;
    color: var(--app-text-secondary);
    margin: 0;
}

.album-link {
    font-size: 1rem;
    color: var(--app-accent);
}

.meta {
    display: flex;
    flex-wrap: wrap;
    gap: 0.75rem;
    align-items: center;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
    margin-top: 0.5rem;
}

.meta span:not(:last-child)::after {
    content: '\00b7';
    margin-left: 0.75rem;
}

.actions {
    margin-top: auto;
    display: flex;
    gap: 0.75rem;
}

@media (max-width: 768px) {
    .detail-layout {
        flex-direction: column;
        align-items: center;
    }

    .cover-art {
        width: 280px;
        height: 280px;
    }

    .song-info {
        align-items: center;
        text-align: center;
    }

    .song-info h1 {
        font-size: 1.75rem;
    }
}
</style>
