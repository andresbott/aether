<script setup lang="ts">
import { computed } from 'vue'
import Slider from 'primevue/slider'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueSidebar } from '@/composables/useQueueSidebar'
import { subsonicClient } from '@/lib/api/subsonic'

const player = usePlayer()
const { sidebarCollapsed, toggleSidebar } = useQueueSidebar()

const coverArtUrl = computed(() => {
    if (!player.currentTrack.value?.coverArt) return null
    if (!subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(player.currentTrack.value.coverArt, 56)
})

const formatTime = (seconds: number): string => {
    if (!seconds || !isFinite(seconds)) return '0:00'
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const progressPercent = computed(() => {
    if (!player.duration.value) return 0
    return (player.currentTime.value / player.duration.value) * 100
})

const onProgressChange = (value: number | number[]) => {
    const numValue = Array.isArray(value) ? value[0] : value
    if (player.duration.value) {
        player.seek((numValue / 100) * player.duration.value)
    }
}

const volumeIcon = computed(() => {
    if (player.volume.value === 0) return 'pi pi-volume-off'
    if (player.volume.value < 0.5) return 'pi pi-volume-down'
    return 'pi pi-volume-up'
})

const volumePercent = computed({
    get: () => player.volume.value * 100,
    set: (val: number) => player.setVolume(val / 100)
})
</script>

<template>
    <div class="player-controls">
        <div class="player-left">
            <div v-if="player.currentTrack.value" class="track-info">
                <div class="track-cover">
                    <img v-if="coverArtUrl" :src="coverArtUrl" alt="Cover" />
                    <div v-else class="cover-placeholder">
                        <i class="pi pi-music"></i>
                    </div>
                </div>
                <div class="track-details">
                    <div class="track-title">{{ player.currentTrack.value.title }}</div>
                    <div class="track-artist">{{ player.currentTrack.value.artist }}</div>
                </div>
            </div>
        </div>

        <div class="player-center">
            <div class="playback-buttons">
                <button
                    class="control-btn"
                    :class="{ active: player.shuffle.value }"
                    @click="player.toggleShuffle"
                >
                    <i class="pi pi-sort-alt"></i>
                </button>
                <button
                    class="control-btn"
                    :disabled="!player.hasPrevious.value"
                    @click="player.playPrevious"
                >
                    <i class="pi pi-step-backward"></i>
                </button>
                <button class="play-btn" @click="player.togglePlayPause">
                    <i :class="player.isPlaying.value ? 'pi pi-pause' : 'pi pi-play'"></i>
                </button>
                <button
                    class="control-btn"
                    :disabled="!player.hasNext.value"
                    @click="player.playNext"
                >
                    <i class="pi pi-step-forward"></i>
                </button>
                <button
                    class="control-btn"
                    :class="{ active: player.repeat.value !== 'none' }"
                    @click="player.toggleRepeat"
                >
                    <i class="pi pi-sync"></i>
                    <span v-if="player.repeat.value === 'one'" class="repeat-badge">1</span>
                </button>
            </div>
            <div class="progress-row">
                <span class="time-label">{{ formatTime(player.currentTime.value) }}</span>
                <Slider
                    :modelValue="progressPercent"
                    @update:modelValue="onProgressChange"
                    class="progress-slider"
                />
                <span class="time-label">{{ formatTime(player.duration.value) }}</span>
            </div>
        </div>

        <div class="player-right">
            <i :class="volumeIcon" class="volume-icon"></i>
            <Slider v-model="volumePercent" class="volume-slider" />
            <button
                class="control-btn queue-toggle"
                :class="{ active: !sidebarCollapsed }"
                @click="toggleSidebar"
            >
                <i class="pi pi-list"></i>
            </button>
        </div>
    </div>
</template>

<style scoped>
.player-controls {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: var(--app-player-height);
    background-color: var(--app-player-bg);
    color: var(--app-player-text);
    display: grid;
    grid-template-columns: 1fr 2fr 1fr;
    align-items: center;
    padding: 0 1.5rem;
    z-index: 100;
    border-top: 1px solid rgba(255, 255, 255, 0.1);
}

.player-left {
    display: flex;
    align-items: center;
}

.track-info {
    display: flex;
    align-items: center;
    gap: 0.75rem;
}

.track-cover {
    width: 56px;
    height: 56px;
    border-radius: 4px;
    overflow: hidden;
    flex-shrink: 0;
}

.track-cover img {
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
    font-size: 1.2rem;
}

.track-details {
    min-width: 0;
}

.track-title {
    font-size: 0.9rem;
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.track-artist {
    font-size: 0.8rem;
    color: rgba(226, 232, 240, 0.7);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.player-center {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
}

.playback-buttons {
    display: flex;
    align-items: center;
    gap: 0.75rem;
}

.control-btn {
    background: none;
    border: none;
    color: rgba(226, 232, 240, 0.7);
    cursor: pointer;
    padding: 0.5rem;
    border-radius: 50%;
    transition: color 0.2s;
    position: relative;
    font-size: 1rem;
}

.control-btn:hover {
    color: #ffffff;
}

.control-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
}

.control-btn.active {
    color: var(--app-accent);
}

.play-btn {
    width: 48px;
    height: 48px;
    border-radius: 50%;
    background-color: var(--app-accent);
    border: none;
    color: #ffffff;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.2rem;
    transition: background-color 0.2s;
}

.play-btn:hover {
    background-color: var(--app-accent-hover);
}

.repeat-badge {
    position: absolute;
    top: -2px;
    right: -2px;
    font-size: 0.6rem;
    font-weight: 700;
    background-color: var(--app-accent);
    color: white;
    border-radius: 50%;
    width: 14px;
    height: 14px;
    display: flex;
    align-items: center;
    justify-content: center;
}

.progress-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    width: 100%;
    max-width: 600px;
}

.time-label {
    font-size: 0.75rem;
    color: rgba(226, 232, 240, 0.6);
    min-width: 36px;
    text-align: center;
}

.progress-slider {
    flex: 1;
}

.player-right {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.75rem;
}

.volume-icon {
    font-size: 1rem;
    color: rgba(226, 232, 240, 0.7);
}

.volume-slider {
    width: 100px;
}

.queue-toggle {
    margin-left: 0.5rem;
}

@media (max-width: 768px) {
    .player-controls {
        grid-template-columns: 1fr auto;
        height: 80px;
        padding: 0 1rem;
    }

    .player-center {
        display: none;
    }

    .player-right {
        gap: 0.5rem;
    }

    .volume-slider {
        display: none;
    }

    .volume-icon {
        display: none;
    }
}
</style>
