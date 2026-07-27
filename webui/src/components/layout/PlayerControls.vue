<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import Slider from 'primevue/slider'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueSidebar } from '@/composables/useQueueSidebar'
import { useToggleStar } from '@/composables/useSubsonicQueries'
import { subsonicClient } from '@/lib/api/subsonic'

const player = usePlayer()
const { sidebarCollapsed, toggleSidebar } = useQueueSidebar()
const toggleStar = useToggleStar()

const currentTrack = computed(() => player.currentTrack.value)

const nowCoverUrl = computed(() => {
    const art = currentTrack.value?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 96)
})

const isStarred = computed(() => !!currentTrack.value?.starred)

const toggleLike = (): void => {
    const track = currentTrack.value
    if (!track) return
    toggleStar.mutate({ id: track.id, starred: isStarred.value })
    // Optimistic local flip so the heart updates immediately (currentTrack isn't
    // query-backed, so it wouldn't otherwise reflect the change until reload).
    track.starred = isStarred.value ? undefined : new Date().toISOString()
}

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

// PrimeVue's Slider only binds mousemove/mouseup after a mousedown on the
// handle; pressing the bare rail is handled by a plain `click`, so holding and
// moving from there doesn't update anything until release. We take over the
// rail-press path: apply the value on mousedown and keep following the pointer
// until mouseup, which is what dragging the handle already does.
const useRailDrag = (apply: (percent: number) => void) => {
    const rail = ref<HTMLElement | null>(null)

    const onDrag = (event: MouseEvent): void => {
        const track = rail.value?.querySelector('.p-slider')
        if (!track) return
        const rect = track.getBoundingClientRect()
        if (rect.width === 0) return
        const ratio = (event.clientX - rect.left) / rect.width
        // floor, matching PrimeVue's own rail math, so the value doesn't shift
        // by a percent when its click handler recomputes it on release.
        apply(Math.min(100, Math.max(0, Math.floor(ratio * 100))))
    }

    const stop = (): void => {
        document.removeEventListener('mousemove', onDrag)
        document.removeEventListener('mouseup', stop)
    }

    const onMouseDown = (event: MouseEvent): void => {
        // Let PrimeVue own presses that start on the handle — that path already
        // tracks the pointer correctly.
        const target = event.target as HTMLElement | null
        if (target?.closest('.p-slider-handle')) return
        if (event.button !== 0) return
        onDrag(event)
        document.addEventListener('mousemove', onDrag)
        document.addEventListener('mouseup', stop)
    }

    onBeforeUnmount(stop)

    return { rail, onMouseDown }
}

const { rail: volumeRail, onMouseDown: onVolumeRailMouseDown } = useRailDrag((percent) => {
    volumePercent.value = percent
})

const { rail: progressRail, onMouseDown: onProgressRailMouseDown } = useRailDrag((percent) => {
    onProgressChange(percent)
})
</script>

<template>
    <div class="player-controls">
        <!-- Now playing: cover + title/artist + like. Also balances the right
             cluster so the center column stays truly centered. -->
        <div class="player-left">
            <template v-if="currentTrack">
                <div class="now-cover">
                    <img v-if="nowCoverUrl" :src="nowCoverUrl" alt="" />
                    <i v-else class="pi pi-music"></i>
                </div>
                <div class="now-text">
                    <div class="now-title">{{ currentTrack.title }}</div>
                    <div class="now-artist">{{ currentTrack.artist || 'Unknown' }}</div>
                </div>
                <button
                    class="now-like"
                    :class="{ liked: isStarred }"
                    type="button"
                    :aria-label="isStarred ? 'Remove from favorites' : 'Add to favorites'"
                    @click="toggleLike"
                >
                    <i :class="isStarred ? 'pi pi-heart-fill' : 'pi pi-heart'"></i>
                </button>
            </template>
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
                </button>
            </div>

            <div class="progress-row">
                <span class="time-label">{{ formatTime(player.currentTime.value) }}</span>
                <div
                    ref="progressRail"
                    class="progress-slider"
                    @mousedown="onProgressRailMouseDown"
                >
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
            <div ref="volumeRail" class="volume-slider" @mousedown="onVolumeRailMouseDown">
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
}

.player-left {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 0.75rem;
    min-width: 0;
}

.now-cover {
    width: 52px;
    height: 52px;
    flex-shrink: 0;
    border-radius: 5px;
    overflow: hidden;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    display: flex;
    align-items: center;
    justify-content: center;
}

.now-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.now-cover i {
    color: rgba(255, 255, 255, 0.85);
    font-size: 1.1rem;
}

.now-text {
    min-width: 0;
    max-width: 200px;
    display: flex;
    flex-direction: column;
}

.now-title {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--app-player-text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.now-artist {
    font-size: 0.75rem;
    color: var(--app-player-dim);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.now-like {
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: none;
    cursor: pointer;
    color: var(--app-player-dim);
    font-size: 1rem;
    padding: 0.4rem;
    border-radius: 50%;
    transition: color 0.2s, background-color 0.2s;
}

.now-like:hover {
    color: var(--app-player-text);
    background-color: rgba(255, 255, 255, 0.07);
}

.now-like.liked {
    color: var(--app-accent);
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

.progress-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    width: 100%;
}

.time-label {
    font-size: 0.75rem;
    font-family: var(--app-player-time-font);
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

/* cursor on .p-slider (not just the rail) so the enlarged ::before hit strip
   below inherits it — the whole clickable area reads as seekable on hover. */
.progress-slider :deep(.p-slider) {
    height: 6px;
    background: var(--app-player-track);
    border-radius: 99px;
    cursor: pointer;
}

/* Enlarge the click/seek target vertically without changing the thin rail's
   look. .p-slider is position:relative, so this transparent pseudo extends the
   hit area ~11px above and below; clicks on it still register on .p-slider
   (pseudo-elements share the element's hit target), and for a horizontal slider
   only the X position feeds the seek — so tapping anywhere on the taller strip
   scrubs to that point. */
.progress-slider :deep(.p-slider)::before {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    top: -11px;
    bottom: -11px;
}

.progress-slider :deep(.p-slider-range) {
    background: var(--app-accent);
    border-radius: 99px;
}

.progress-slider :deep(.p-slider-handle),
.volume-slider :deep(.p-slider-handle) {
    width: 14px;
    height: 14px;
    margin-top: -7px;
    margin-left: -7px;
    background: var(--app-accent);
    border: none;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.35);
}

.progress-slider :deep(.p-slider-handle)::before,
.volume-slider :deep(.p-slider-handle)::before {
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

/* Wide enough that each 1% volume step is ~1.5px of travel, so the rail can be
   set precisely by eye instead of jumping several percent per pixel. */
.volume-slider {
    width: 150px;
}

.volume-slider :deep(.p-slider) {
    height: 5px;
    background: var(--app-player-track);
    border-radius: 99px;
    cursor: pointer;
}

/* Same enlarged click target as the progress bar — thin rail, taller hit strip. */
.volume-slider :deep(.p-slider)::before {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    top: -11px;
    bottom: -11px;
}

.volume-slider :deep(.p-slider-range) {
    background: var(--app-accent);
    border-radius: 99px;
}

/* Below this the wider rail starts squeezing the centered playback column, so
   fall back to the compact width. */
@media (max-width: 1100px) {
    .volume-slider {
        width: 90px;
    }
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
