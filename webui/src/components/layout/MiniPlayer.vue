<script setup lang="ts">
import { computed } from 'vue'
import { usePlayer } from '@/composables/usePlayer'
import { subsonicClient } from '@/lib/api/subsonic'

// The Now Playing sheet's collapsed strip (NowPlayingSheet.vue). Dumb on
// purpose: the sheet owns the lift gesture, the strip cross-fade and the
// click-after-drag swallowing — this bar only renders the track and emits
// `open` for a tap. It never navigates itself, so it works no matter which
// route sits under the sheet.
const emit = defineEmits<{ (e: 'open'): void }>()

const player = usePlayer()

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
</script>

<template>
    <div class="mini-player">
        <div class="mini-progress" aria-hidden="true">
            <div class="mini-progress-fill" :style="{ width: progressPercent + '%' }"></div>
        </div>

        <!-- The whole-bar tap target: a real button UNDER the transport (which
             is positioned above it) rather than a role="button" wrapper around
             it — nested buttons break keyboard transport, since Enter/Space on
             Pause bubbles into the wrapper and opens the sheet instead. -->
        <button
            type="button"
            class="mini-open"
            aria-label="Open Now Playing"
            @click="emit('open')"
        ></button>

        <img v-if="coverUrl" :src="coverUrl" alt="" class="mini-cover" />
        <div v-else class="mini-cover mini-cover--placeholder" aria-hidden="true">
            <i class="pi pi-music"></i>
        </div>

        <div class="mini-meta">
            <span class="mini-title">{{ currentTrack?.title }}</span>
            <span class="mini-artist">{{ currentTrack?.artist }}</span>
        </div>

        <button
            type="button"
            class="mini-btn"
            :aria-label="player.isPlaying.value ? 'Pause' : 'Play'"
            @click="player.togglePlayPause()"
        >
            <i :class="player.isPlaying.value ? 'pi pi-pause' : 'pi pi-play'"></i>
        </button>
        <button
            type="button"
            class="mini-btn"
            aria-label="Next track"
            @click="player.playNext()"
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
    /* Bottom-most mobile chrome, so it reserves the home-indicator inset
       (the tab bar used to, before the nav moved into the drawer). */
    height: calc(var(--app-mini-player-height) + env(safe-area-inset-bottom));
    flex-shrink: 0;
    padding: 0 0.75rem env(safe-area-inset-bottom);
    box-sizing: border-box;
    background-color: var(--app-player-bg);
    color: var(--app-player-text);
}

/* Positioned after the static cover/meta in paint order, so the whole bar is
   one tap target; the transport buttons are position: relative and later in
   the DOM, so they stack above it and keep their own clicks. */
.mini-open {
    position: absolute;
    inset: 0;
    border: none;
    background: none;
    padding: 0;
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
    position: relative;
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
