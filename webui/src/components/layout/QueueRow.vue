<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Song } from '@/types/subsonic'

const props = defineProps<{ song: Song; position: number }>()
const emit = defineEmits<{ play: []; remove: [] }>()

const hovered = ref(false)

const coverUrl = computed<string | null>(() => {
    if (!props.song.coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(props.song.coverArt, 48)
})

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}
</script>

<template>
    <button
        type="button"
        class="queue-row"
        @click="emit('play')"
        @mouseenter="hovered = true"
        @mouseleave="hovered = false"
    >
        <span class="row-index">
            <i v-if="hovered" class="pi pi-play play-hover-icon"></i>
            <span v-else class="track-number">{{ position }}</span>
        </span>
        <span class="row-cover">
            <img v-if="coverUrl" :src="coverUrl" alt="" />
            <i v-else class="pi pi-music"></i>
        </span>
        <span class="row-info">
            <span class="row-title">{{ song.title }}</span>
            <span class="row-artist">{{ song.artist || 'Unknown' }}</span>
        </span>
        <span class="row-end">
            <span class="row-duration">{{ formatDuration(song.duration) }}</span>
            <Button
                icon="pi pi-trash"
                text
                rounded
                size="small"
                severity="secondary"
                class="remove-button"
                v-tooltip.left="'Remove from queue'"
                @click.stop="emit('remove')"
            />
        </span>
    </button>
</template>

<style scoped>
.queue-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    width: 100%;
    padding: 0.4rem 0.5rem;
    border: none;
    background: none;
    cursor: pointer;
    text-align: left;
    transition: background-color 0.15s;
}

.queue-row:hover {
    background-color: var(--app-background);
}

.row-index {
    width: 1.75rem;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
}

.track-number {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    font-weight: 500;
}

.play-hover-icon {
    color: var(--app-text-primary);
    font-size: 1rem;
}

.row-cover {
    width: 40px;
    height: 40px;
    flex-shrink: 0;
    border-radius: 4px;
    overflow: hidden;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    display: flex;
    align-items: center;
    justify-content: center;
}

.row-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.row-cover i {
    font-size: 0.9rem;
    color: rgba(255, 255, 255, 0.85);
}

.row-info {
    min-width: 0;
    flex: 1;
    display: flex;
    flex-direction: column;
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
    font-size: 0.75rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.row-end {
    position: relative;
    width: 56px;
    flex-shrink: 0;
    text-align: right;
}

.row-duration {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}

.queue-row:hover .row-duration {
    visibility: hidden;
}

.remove-button {
    position: absolute;
    top: 50%;
    right: 0;
    transform: translateY(-50%);
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.15s;
}

.queue-row:hover .remove-button {
    opacity: 1;
    pointer-events: auto;
}
</style>
