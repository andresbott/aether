<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import Slider from 'primevue/slider'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueSidebar } from '@/composables/useQueueSidebar'
import { useCurrentTrackFavorite } from '@/composables/useCurrentTrackFavorite'
import { subsonicClient } from '@/lib/api/subsonic'

const player = usePlayer()
const { sidebarCollapsed, toggleSidebar } = useQueueSidebar()
// Shared with the `L` shortcut so the heart and the key flip the same state.
const { isStarred, toggleFavorite } = useCurrentTrackFavorite()

const currentTrack = computed(() => player.currentTrack.value)

const nowCoverUrl = computed(() => {
    const art = currentTrack.value?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 96)
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

// Three loudness steps that have to be told apart at a glance. PrimeIcons has no
// slashed-speaker glyph — `pi-volume-off` is a bare cone, which reads as "quiet"
// next to `pi-volume-down`, not as "muted". The `muted` class carries the strike
// the stylesheet draws over the cone, giving silence a distinct icon.
const volumeIcon = computed(() => {
    if (player.isMuted.value) return 'pi pi-volume-off muted'
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
//
// The same wrapper reports whether its rail is "active" — hovered or being
// dragged. An inactive bar is fully neutral: no knob, and a grey fill instead of
// the accent, so a resting player bar carries no colour. Hover alone can't drive
// this in CSS — dragging past the bar's edge fires mouseleave while the grab is
// still on, and the rail must not go neutral mid-drag.
const useRailDrag = (apply: (percent: number) => void) => {
    const rail = ref<HTMLElement | null>(null)
    const hovered = ref(false)
    const dragging = ref(false)
    const active = computed(() => hovered.value || dragging.value)

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
        dragging.value = false
        document.removeEventListener('mousemove', onDrag)
        document.removeEventListener('mouseup', stop)
    }

    const onMouseDown = (event: MouseEvent): void => {
        if (event.button !== 0) return
        dragging.value = true
        document.addEventListener('mouseup', stop)
        // Let PrimeVue own presses that start on the handle — that path already
        // tracks the pointer correctly. We still follow the drag for the active
        // state.
        const target = event.target as HTMLElement | null
        if (target?.closest('.p-slider-handle')) return
        onDrag(event)
        document.addEventListener('mousemove', onDrag)
    }

    onBeforeUnmount(stop)

    return {
        rail,
        active,
        onMouseDown,
        onMouseEnter: () => (hovered.value = true),
        onMouseLeave: () => (hovered.value = false)
    }
}

const {
    rail: volumeRail,
    active: volumeRailActive,
    onMouseDown: onVolumeRailMouseDown,
    onMouseEnter: onVolumeRailEnter,
    onMouseLeave: onVolumeRailLeave
} = useRailDrag((percent) => {
    volumePercent.value = percent
})

const {
    rail: progressRail,
    active: progressRailActive,
    onMouseDown: onProgressRailMouseDown,
    onMouseEnter: onProgressRailEnter,
    onMouseLeave: onProgressRailLeave
} = useRailDrag((percent) => {
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
                <!-- data-shortcut anchors the help overlay's key badge to this
                     control; the overlay measures it live, so nothing here has
                     to know about the badge. -->
                <button
                    class="now-like"
                    data-shortcut="favorite"
                    :class="{ liked: isStarred }"
                    type="button"
                    :aria-label="isStarred ? 'Remove from favorites' : 'Add to favorites'"
                    @click="toggleFavorite"
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
                    data-shortcut="previous"
                    :disabled="!player.hasPrevious.value"
                    @click="player.playPrevious"
                >
                    <i class="pi pi-step-backward"></i>
                </button>
                <button class="play-btn" data-shortcut="play-pause" @click="player.togglePlayPause">
                    <i :class="player.isPlaying.value ? 'pi pi-pause' : 'pi pi-play'"></i>
                </button>
                <button
                    class="control-btn"
                    data-shortcut="next"
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
                <!-- The anchor is the rail itself, not the row: the row also holds
                     the two time labels, so a badge centred on it would miss the
                     bar the arrows actually scrub. -->
                <div
                    ref="progressRail"
                    class="progress-slider"
                    data-shortcut="progress"
                    :class="{ 'rail-active': progressRailActive }"
                    @mousedown="onProgressRailMouseDown"
                    @mouseenter="onProgressRailEnter"
                    @mouseleave="onProgressRailLeave"
                >
                    <Slider :modelValue="progressPercent" @update:modelValue="onProgressChange" />
                </div>
                <span class="time-label">{{ formatTime(player.duration.value) }}</span>
            </div>
        </div>

        <div class="player-right">
            <button
                class="control-btn volume-toggle"
                data-shortcut="mute"
                type="button"
                :aria-label="player.isMuted.value ? 'Unmute' : 'Mute'"
                @click="player.toggleMute"
            >
                <i :class="volumeIcon"></i>
            </button>
            <div
                ref="volumeRail"
                class="volume-slider"
                data-shortcut="volume"
                :class="{ 'rail-active': volumeRailActive }"
                @mousedown="onVolumeRailMouseDown"
                @mouseenter="onVolumeRailEnter"
                @mouseleave="onVolumeRailLeave"
            >
                <Slider v-model="volumePercent" />
            </div>
            <button
                class="control-btn queue-toggle"
                data-shortcut="queue"
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
    transition:
        color 0.2s,
        background-color 0.2s;
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
    transition:
        color 0.2s,
        background-color 0.2s;
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

/* An idle bar carries no colour and no knob: the fill is the neutral player
   token and the handle is faded out. Both take the accent only while the rail is
   hovered or dragged — the wrapper carries .rail-active then. The fill stays
   brighter than the groove so the position is readable without hovering. */
.progress-slider :deep(.p-slider-range),
.volume-slider :deep(.p-slider-range) {
    background: var(--app-player-range);
    border-radius: 99px;
    transition: background-color 0.15s;
}

.progress-slider.rail-active :deep(.p-slider-range),
.volume-slider.rail-active :deep(.p-slider-range) {
    background: var(--app-accent);
}

/* The knob is faded with opacity alone — deliberately not `visibility` or
   `display`, which would drop the handle out of the tab order. It is the
   slider's focusable element (tabindex=0, role=slider), so hiding it that way
   would make volume and seek unreachable by keyboard. Focus reveals it (and its
   rail's accent) for the same reason. */
.progress-slider :deep(.p-slider-handle),
.volume-slider :deep(.p-slider-handle) {
    opacity: 0;
    transition: opacity 0.15s;
}

.progress-slider.rail-active :deep(.p-slider-handle),
.volume-slider.rail-active :deep(.p-slider-handle),
.progress-slider :deep(.p-slider-handle:focus),
.volume-slider :deep(.p-slider-handle:focus) {
    opacity: 1;
}

.progress-slider :deep(.p-slider:focus-within .p-slider-range),
.volume-slider :deep(.p-slider:focus-within .p-slider-range) {
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

/* Narrower than the playback controls: the speaker sits right next to its rail,
   so a 38px circle would push the two apart. */
.volume-toggle {
    width: 30px;
    height: 30px;
    font-size: 1rem;
}

/* The muted state's own icon. PrimeIcons has no slashed speaker — `pi-volume-off`
   is a bare cone that reads as "quiet" beside `pi-volume-down` — so the strike is
   drawn here, over the cone, in the icon's own colour.
   Every dimension is in `em`, including the knockout ring: the glyph is 1rem in
   the bar but resizes, and a px ring that looks tight at 40px swallows the cone at
   16px. The ring is the bar's own background, so the line stays legible where it
   crosses the cone's strokes without hiding them. */
.volume-toggle i.muted {
    position: relative;
}

.volume-toggle i.muted::after {
    content: '';
    position: absolute;
    left: 50%;
    top: 50%;
    width: 0.8em;
    height: 0.105em;
    border-radius: 0.105em;
    background: currentColor;
    box-shadow: 0 0 0 0.11em var(--app-player-bg);
    transform: translate(-50%, -50%) rotate(-45deg);
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

    .volume-toggle {
        display: none;
    }
}
</style>
