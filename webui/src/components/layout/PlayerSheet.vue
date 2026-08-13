<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import QueueBody from '@/components/layout/QueueBody.vue'
import { usePlayer } from '@/composables/usePlayer'
import { usePlayerSheet } from '@/composables/usePlayerSheet'
import { useCurrentTrackFavorite } from '@/composables/useCurrentTrackFavorite'
import { subsonicClient } from '@/lib/api/subsonic'

const player = usePlayer()
const { isOpen, close } = usePlayerSheet()
const { isStarred, toggleFavorite } = useCurrentTrackFavorite()
const router = useRouter()
const route = useRoute()

const currentTrack = computed(() => player.currentTrack.value)

const coverUrl = computed(() => {
    const art = currentTrack.value?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 512)
})

// Two faces, one overlay: the player face and the queue list. Reset to the
// player face on every open so the sheet always comes up showing the track.
const showQueue = ref(false)
watch(isOpen, (open) => {
    if (open) showQueue.value = false
})

const formatTime = (seconds: number): string => {
    if (!seconds || !isFinite(seconds)) return '0:00'
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const onSeekInput = (event: Event): void => {
    player.seek(Number((event.target as HTMLInputElement).value))
}

const goAlbum = (): void => {
    const id = currentTrack.value?.albumId
    if (!id) return
    close(() => void router.push({ name: 'album', params: { id } }))
}

const goArtist = (): void => {
    const id = currentTrack.value?.artistId
    if (!id) return
    close(() => void router.push({ name: 'artist', params: { id } }))
}

// Fallback dismissal (spec §6): any navigation closes the sheet, so a failed
// popstate consumption can never leave it stranded over a different view.
watch(
    () => route.fullPath,
    () => {
        close()
    }
)

// Esc closes. Bound to our own listener while open — the desktop shortcut
// registry never runs in the mobile shell, so nothing else claims the key.
const onKeydown = (event: KeyboardEvent): void => {
    if (event.key === 'Escape') {
        close()
    }
}
watch(
    isOpen,
    (open) => {
        if (open) window.addEventListener('keydown', onKeydown)
        else window.removeEventListener('keydown', onKeydown)
    },
    { immediate: true }
)
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

// Swipe-down on the header dismisses. Header only: a downward drag on the
// seek rail or the queue list is scrolling, not dismissal.
let touchStartY: number | null = null
const onHeaderTouchStart = (event: TouchEvent): void => {
    touchStartY = event.touches[0]?.clientY ?? null
}
const onHeaderTouchEnd = (event: TouchEvent): void => {
    if (touchStartY === null) return
    const endY = event.changedTouches[0]?.clientY ?? touchStartY
    if (endY - touchStartY > 80) {
        close()
    }
    touchStartY = null
}
</script>

<template>
    <Teleport to="body">
        <Transition name="sheet">
            <div v-if="isOpen" class="player-sheet" role="dialog" aria-label="Now playing">
                <div
                    class="sheet-header"
                    @touchstart.passive="onHeaderTouchStart"
                    @touchend.passive="onHeaderTouchEnd"
                >
                    <button
                        type="button"
                        class="sheet-btn"
                        aria-label="Close player"
                        @click="() => close()"
                    >
                        <i class="pi pi-chevron-down"></i>
                    </button>
                    <span class="sheet-header-title">{{ showQueue ? 'Queue' : 'Now Playing' }}</span>
                    <button
                        type="button"
                        class="sheet-btn"
                        :class="{ active: showQueue }"
                        :aria-label="showQueue ? 'Show now playing' : 'Show queue'"
                        :aria-pressed="showQueue"
                        @click="showQueue = !showQueue"
                    >
                        <i class="pi pi-list"></i>
                    </button>
                </div>

                <div v-if="showQueue" class="sheet-queue">
                    <QueueBody variant="sidebar" :edit-mode="false" />
                </div>

                <div v-else class="sheet-face">
                    <div class="sheet-art">
                        <img v-if="coverUrl" :src="coverUrl" alt="" class="sheet-cover" />
                        <div v-else class="sheet-cover sheet-cover--placeholder" aria-hidden="true">
                            <i class="pi pi-music"></i>
                        </div>
                    </div>

                    <div class="sheet-meta">
                        <button
                            type="button"
                            class="sheet-title"
                            :disabled="!currentTrack?.albumId"
                            @click="goAlbum"
                        >
                            {{ currentTrack?.title }}
                        </button>
                        <button
                            type="button"
                            class="sheet-artist"
                            :disabled="!currentTrack?.artistId"
                            @click="goArtist"
                        >
                            {{ currentTrack?.artist }}
                        </button>
                    </div>

                    <div class="sheet-seek">
                        <span class="sheet-time">{{ formatTime(player.currentTime.value) }}</span>
                        <input
                            type="range"
                            class="sheet-range"
                            aria-label="Seek"
                            min="0"
                            :max="player.duration.value || 0"
                            step="1"
                            :value="player.currentTime.value"
                            @input="onSeekInput"
                        />
                        <span class="sheet-time">{{ formatTime(player.duration.value) }}</span>
                    </div>

                    <div class="sheet-transport">
                        <button
                            type="button"
                            class="sheet-btn"
                            :class="{ active: player.shuffle.value }"
                            aria-label="Shuffle"
                            @click="player.toggleShuffle()"
                        >
                            <i class="pi pi-arrow-right-arrow-left"></i>
                        </button>
                        <button
                            type="button"
                            class="sheet-btn"
                            aria-label="Previous track"
                            :disabled="!player.hasPrevious.value"
                            @click="player.playPrevious()"
                        >
                            <i class="pi pi-step-backward"></i>
                        </button>
                        <button
                            type="button"
                            class="sheet-btn sheet-btn--play"
                            :aria-label="player.isPlaying.value ? 'Pause' : 'Play'"
                            @click="player.togglePlayPause()"
                        >
                            <i :class="player.isPlaying.value ? 'pi pi-pause' : 'pi pi-play'"></i>
                        </button>
                        <button
                            type="button"
                            class="sheet-btn"
                            aria-label="Next track"
                            :disabled="!player.hasNext.value"
                            @click="player.playNext()"
                        >
                            <i class="pi pi-step-forward"></i>
                        </button>
                        <button
                            type="button"
                            class="sheet-btn"
                            :class="{ active: player.repeat.value !== 'none' }"
                            aria-label="Repeat"
                            @click="player.toggleRepeat()"
                        >
                            <i class="pi pi-refresh"></i>
                        </button>
                    </div>

                    <button
                        type="button"
                        class="sheet-btn sheet-favorite"
                        :aria-label="isStarred ? 'Remove from favorites' : 'Add to favorites'"
                        @click="toggleFavorite()"
                    >
                        <i :class="isStarred ? 'pi pi-heart-fill' : 'pi pi-heart'"></i>
                    </button>
                </div>
            </div>
        </Transition>
    </Teleport>
</template>

<style scoped>
.player-sheet {
    position: fixed;
    inset: 0;
    z-index: 1200;
    display: flex;
    flex-direction: column;
    background-color: var(--app-player-bg);
    color: var(--app-player-text);
    padding-top: env(safe-area-inset-top);
    padding-bottom: env(safe-area-inset-bottom);
}

.sheet-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.5rem 0.75rem;
    flex-shrink: 0;
}

.sheet-header-title {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--app-player-dim);
}

.sheet-face {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1.25rem;
    padding: 0 1.5rem 1.5rem;
}

.sheet-queue {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
}

.sheet-art {
    width: min(75vw, 45vh);
    aspect-ratio: 1;
}

.sheet-cover {
    width: 100%;
    height: 100%;
    border-radius: var(--app-radius);
    object-fit: cover;
}

.sheet-cover--placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: var(--app-player-track);
    color: var(--app-player-dim);
    font-size: 3rem;
}

.sheet-meta {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.25rem;
    max-width: 100%;
}

.sheet-title,
.sheet-artist {
    border: none;
    background: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
    max-width: 80vw;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.sheet-title {
    font-size: 1.25rem;
    font-weight: 700;
}

.sheet-artist {
    font-size: 0.95rem;
    color: var(--app-player-dim);
}

.sheet-title:disabled,
.sheet-artist:disabled {
    cursor: default;
}

.sheet-seek {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    width: 100%;
}

.sheet-range {
    flex: 1;
    accent-color: var(--app-accent);
}

.sheet-time {
    font-size: 0.75rem;
    font-family: var(--app-player-time-font);
    color: var(--app-player-dim);
    min-width: 2.5rem;
    text-align: center;
}

.sheet-transport {
    display: flex;
    align-items: center;
    gap: 1.1rem;
}

.sheet-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2.75rem;
    height: 2.75rem;
    border: none;
    background: none;
    color: var(--app-player-text);
    cursor: pointer;
    font-size: 1.15rem;
}

.sheet-btn:disabled {
    color: var(--app-player-dim);
    cursor: default;
}

.sheet-btn.active {
    color: var(--app-accent);
}

.sheet-btn--play {
    width: 3.5rem;
    height: 3.5rem;
    border-radius: 50%;
    background-color: var(--app-accent);
    color: var(--app-player-bg);
    font-size: 1.3rem;
}

.sheet-favorite {
    position: absolute;
    right: 1rem;
    bottom: calc(1rem + env(safe-area-inset-bottom));
}

.sheet-enter-active,
.sheet-leave-active {
    transition: transform 0.25s ease, opacity 0.25s ease;
}

.sheet-enter-from,
.sheet-leave-to {
    transform: translateY(100%);
    opacity: 0.6;
}
</style>
