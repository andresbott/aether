<script setup lang="ts">
import { computed } from 'vue'
import { usePlayer } from '@/composables/usePlayer'
import { usePlayerSheet } from '@/composables/usePlayerSheet'
import { subsonicClient } from '@/lib/api/subsonic'

const player = usePlayer()
const { open: openPlayerSheet } = usePlayerSheet()

const currentTrack = computed(() => player.currentTrack.value)

const coverUrl = computed(() => {
    const art = currentTrack.value?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 96)
})

const progressPercent = computed(() => {
    if (!player.duration.value) return 0
    return (player.currentTime.value / player.duration.value) * 100
})

// Phase 2: the tap target is the PlayerSheet overlay (was the home route in
// phase 1 — this function body is the swap point the phase-1 comment promised).
const openNowPlaying = (): void => {
    openPlayerSheet()
}
</script>

<template>
    <div
        class="mini-player"
        role="button"
        tabindex="0"
        aria-label="Open player"
        @click="openNowPlaying"
        @keydown.enter="openNowPlaying"
        @keydown.space.prevent="openNowPlaying"
    >
        <div class="mini-progress" aria-hidden="true">
            <div class="mini-progress-fill" :style="{ width: progressPercent + '%' }"></div>
        </div>

        <img v-if="coverUrl" :src="coverUrl" alt="" class="mini-cover" />
        <div v-else class="mini-cover mini-cover--placeholder" aria-hidden="true">
            <i class="pi pi-music"></i>
        </div>

        <div class="mini-meta">
            <span class="mini-title">{{ currentTrack?.title }}</span>
            <span class="mini-artist">{{ currentTrack?.artist }}</span>
        </div>

        <!-- .stop: the transport must not also open Now Playing. -->
        <button
            type="button"
            class="mini-btn"
            :aria-label="player.isPlaying.value ? 'Pause' : 'Play'"
            @click.stop="player.togglePlayPause()"
        >
            <i :class="player.isPlaying.value ? 'pi pi-pause' : 'pi pi-play'"></i>
        </button>
        <button
            type="button"
            class="mini-btn"
            aria-label="Next track"
            @click.stop="player.playNext()"
        >
            <i class="pi pi-step-forward"></i>
        </button>
    </div>
</template>

<style scoped>
.mini-player {
    position: relative;
    display: flex;
    align-items: center;
    gap: 0.65rem;
    height: var(--app-mini-player-height);
    flex-shrink: 0;
    padding: 0 0.75rem;
    background-color: var(--app-player-bg);
    color: var(--app-player-text);
    cursor: pointer;
}

.mini-progress {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    height: 2px;
    background-color: var(--app-player-track);
}

.mini-progress-fill {
    height: 100%;
    background-color: var(--app-accent);
}

.mini-cover {
    width: 2.4rem;
    height: 2.4rem;
    border-radius: 4px;
    object-fit: cover;
    flex-shrink: 0;
}

.mini-cover--placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: var(--app-player-track);
    color: var(--app-player-dim);
}

.mini-meta {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
}

.mini-title,
.mini-artist {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.mini-title {
    font-size: 0.85rem;
    font-weight: 600;
}

.mini-artist {
    font-size: 0.75rem;
    color: var(--app-player-dim);
}

.mini-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2.5rem;
    height: 2.5rem;
    border: none;
    background: none;
    color: var(--app-player-text);
    cursor: pointer;
    font-size: 1.1rem;
    flex-shrink: 0;
}
</style>
