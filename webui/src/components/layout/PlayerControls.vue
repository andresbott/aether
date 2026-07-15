<script setup lang="ts">
import { computed } from 'vue'
import Slider from 'primevue/slider'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueSidebar } from '@/composables/useQueueSidebar'

const player = usePlayer()
const { sidebarCollapsed, toggleSidebar } = useQueueSidebar()

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
        <!-- Spacer: balances the right cluster so the center column stays truly
             centered. Reserved for now-playing info (cover/title) in a later step. -->
        <div class="player-left"></div>

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
                <div class="progress-slider">
                    <Slider
                        :modelValue="progressPercent"
                        @update:modelValue="onProgressChange"
                    />
                </div>
                <span class="time-label">{{ formatTime(player.duration.value) }}</span>
            </div>
        </div>

        <div class="player-right">
            <i :class="volumeIcon" class="volume-icon"></i>
            <div class="volume-slider">
                <Slider v-model="volumePercent" />
            </div>
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
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 20px;
    padding: 0 22px;
    z-index: 100;
    border-top: 1px solid rgba(255, 255, 255, 0.1);
}

.player-left {
    flex: 1;
}

.player-center {
    flex: 0 1 560px;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 11px;
}

.playback-buttons {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}

.control-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 38px;
    height: 38px;
    background: none;
    border: none;
    color: var(--app-player-dim);
    cursor: pointer;
    border-radius: 50%;
    transition: color 0.2s, background-color 0.2s;
    position: relative;
    font-size: 1.1rem;
}

.control-btn:hover {
    color: var(--app-player-text);
    background-color: rgba(255, 255, 255, 0.07);
}

.control-btn:disabled {
    opacity: 0.3;
    cursor: not-allowed;
}

.control-btn.active {
    color: var(--app-accent);
}

.play-btn {
    width: 44px;
    height: 44px;
    border-radius: 50%;
    background-color: var(--app-accent);
    border: none;
    color: #ffffff;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 1.125rem;
    box-shadow: 0 3px 10px rgba(14, 155, 181, 0.3);
    transition: background-color 0.2s;
}

.play-btn:hover {
    background-color: var(--app-accent-hover);
}

:global(.dark-mode) .play-btn {
    box-shadow: 0 3px 12px rgba(47, 211, 239, 0.3);
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
}

.time-label {
    font-size: 0.75rem;
    color: var(--app-player-dim);
    min-width: 36px;
    text-align: center;
    font-variant-numeric: tabular-nums;
}

/* Each Slider is wrapped in its own div so :deep() can reach PrimeVue's inner
   elements reliably. The visible knob is the handle's ::before dot, so it must be
   coloured too. PrimeVue centres the handle with top:50% + margin-block-start of
   -handleHeight/2 (from its token), so a resized knob needs its margins reset. */
.progress-slider {
    flex: 1;
}

.progress-slider :deep(.p-slider) {
    height: 6px;
    background: var(--app-player-track);
    border-radius: 99px;
}

.progress-slider :deep(.p-slider-range) {
    background: var(--app-accent);
    border-radius: 99px;
}

.progress-slider :deep(.p-slider-handle) {
    width: 14px;
    height: 14px;
    margin-top: -7px;
    margin-left: -7px;
    background: var(--app-accent);
    border: none;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.35);
}

.progress-slider :deep(.p-slider-handle)::before {
    width: 14px;
    height: 14px;
    background: var(--app-accent);
}

.player-right {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    justify-content: flex-end;
}

.queue-toggle {
    margin-left: 0.5rem;
}

.volume-icon {
    font-size: 1rem;
    color: var(--app-player-dim);
}

.volume-slider {
    width: 90px;
}

.volume-slider :deep(.p-slider) {
    height: 5px;
    background: var(--app-player-track);
    border-radius: 99px;
}

.volume-slider :deep(.p-slider-range) {
    background: var(--app-player-dim);
    border-radius: 99px;
}

/* Mock volume has no visible knob; keep it draggable but invisible. */
.volume-slider :deep(.p-slider-handle) {
    opacity: 0;
}

@media (max-width: 768px) {
    .player-controls {
        gap: 0.75rem;
        padding: 0 0.75rem;
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
