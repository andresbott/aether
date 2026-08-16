<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useCurrentTrackFavorite } from '@/composables/useCurrentTrackFavorite'
import { usePlayer } from '@/composables/usePlayer'
import { subsonicClient } from '@/lib/api/subsonic'

// The Now Playing sheet's expanded face (NowPlayingSheet.vue): cover art,
// title/artist links, seek, and a prev/play/next transport — shuffle/repeat
// are queue behaviour and live in QueuePanel's heading. Deliberately free of
// gesture code: the sheet owns every drag, this face only names the two
// non-gesture twins of its swipes (⌄ collapse, ⌃ show queue) as emits.
const emit = defineEmits<{ (e: 'collapse'): void; (e: 'show-queue'): void }>()

const player = usePlayer()
const router = useRouter()
const { isStarred, toggleFavorite } = useCurrentTrackFavorite()

const currentTrack = computed(() => player.currentTrack.value)

const coverUrl = computed(() => {
    const art = currentTrack.value?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 512)
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

// Favorite is a gesture here, not a button: double-tap the cover to toggle,
// with the starred state echoed by the corner indicator below. Detected by
// click timing because iOS Safari doesn't fire dblclick for double-taps;
// `touch-action: manipulation` on the art removes the double-tap zoom that
// would otherwise swallow the second tap.
const DOUBLE_TAP_MS = 300
let lastArtTap = 0
const onArtTap = (): void => {
    const now = Date.now()
    if (now - lastArtTap <= DOUBLE_TAP_MS) {
        lastArtTap = 0
        toggleFavorite()
        return
    }
    lastArtTap = now
}

const goAlbum = (): void => {
    const id = currentTrack.value?.albumId
    if (id) void router.push({ name: 'album', params: { id } })
}

const goArtist = (): void => {
    const id = currentTrack.value?.artistId
    if (id) void router.push({ name: 'artist', params: { id } })
}
</script>

<template>
    <section class="play-face">
        <!-- The face's only chrome, and the non-gesture twin of the dismiss
             drag: where every other view carries the hamburger, this one
             carries a chevron that collapses the sheet back onto the page. -->
        <button
            type="button"
            class="play-nav-hint"
            aria-label="Close Now Playing"
            @click="emit('collapse')"
        >
            <i class="pi pi-angle-down" aria-hidden="true"></i>
        </button>

        <div class="play-art" @click="onArtTap">
            <img v-if="coverUrl" :src="coverUrl" alt="" class="play-cover" />
            <div v-else class="play-cover play-cover--placeholder" aria-hidden="true">
                <i class="pi pi-music"></i>
            </div>
            <!-- Passive echo of the starred state, set by double-tapping the
                 cover. Decorative only (the queue rows' hearts are the
                 accessible toggle), hence aria-hidden. -->
            <i
                v-if="isStarred"
                class="pi pi-heart-fill play-favorite-indicator"
                aria-hidden="true"
            ></i>
        </div>

        <div class="play-meta">
            <button
                type="button"
                class="play-title"
                :disabled="!currentTrack?.albumId"
                @click="goAlbum"
            >
                {{ currentTrack?.title }}
            </button>
            <button
                type="button"
                class="play-artist"
                :disabled="!currentTrack?.artistId"
                @click="goArtist"
            >
                {{ currentTrack?.artist }}
            </button>
        </div>

        <div class="play-seek">
            <input
                type="range"
                class="play-range"
                aria-label="Seek"
                min="0"
                :max="player.duration.value || 0"
                step="1"
                :value="player.currentTime.value"
                @input="onSeekInput"
            />
            <div class="play-times">
                <span class="play-time">{{ formatTime(player.currentTime.value) }}</span>
                <span class="play-time">{{ formatTime(player.duration.value) }}</span>
            </div>
        </div>

        <!-- Prev / play / next only: shuffle and repeat live with the queue
             heading (QueuePanel). -->
        <div class="play-transport">
            <button
                type="button"
                class="play-btn"
                aria-label="Previous track"
                :disabled="!player.hasPrevious.value"
                @click="player.playPrevious()"
            >
                <i class="pi pi-step-backward"></i>
            </button>
            <button
                type="button"
                class="play-btn play-btn--play"
                :aria-label="player.isPlaying.value ? 'Pause' : 'Play'"
                @click="player.togglePlayPause()"
            >
                <i :class="player.isPlaying.value ? 'pi pi-pause' : 'pi pi-play'"></i>
            </button>
            <button
                type="button"
                class="play-btn"
                aria-label="Next track"
                :disabled="!player.hasNext.value"
                @click="player.playNext()"
            >
                <i class="pi pi-step-forward"></i>
            </button>
        </div>

        <button
            type="button"
            class="play-swipe-hint"
            aria-label="Show queue"
            @click="emit('show-queue')"
        >
            <i class="pi pi-angle-up" aria-hidden="true"></i>
        </button>
    </section>
</template>

<style scoped>
.play-face {
    position: relative;
    height: 100%;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1.25rem;
    /* Reserves BOTH insets: expanded, the face is the whole screen — the
       status bar (or the notch, in landscape) would otherwise sit on the nav
       chevron, and there is no mini strip below it to hold the home inset. */
    padding: calc(0.25rem + env(safe-area-inset-top)) 1.5rem
        calc(0.5rem + env(safe-area-inset-bottom));
}

.play-art {
    /* This margin pairs with .play-meta's margin-bottom: equal spare height
       above the art and below the title/artist centers the two as one group
       on a tall screen, while the seek bar and buttons anchor to the bottom
       where thumbs are — instead of the whole stack floating mid-screen. On a
       short screen the auto margins shrink to zero and the face packs as
       before. */
    margin-top: auto;
    position: relative;
    /* Full padded width — the face's 1.5rem side padding is the only border —
       capped by height so the transport keeps room on short screens. dvh, not
       vh: the cap has to track the height the panel actually gets. */
    width: min(100%, 45dvh);
    aspect-ratio: 1;
    /* Kill the browser's double-tap zoom so the favorite gesture gets both
       taps, and the 300ms click delay with it. */
    touch-action: manipulation;
    -webkit-user-select: none;
    user-select: none;
}

.play-cover {
    width: 100%;
    height: 100%;
    border-radius: var(--app-radius);
    object-fit: cover;
}

.play-cover--placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: var(--app-hover);
    color: var(--app-text-secondary);
    font-size: 3rem;
}

.play-meta {
    /* The other half of .play-art's margin-top — see the comment there. */
    margin-bottom: auto;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.25rem;
    max-width: 100%;
}

.play-title,
.play-artist {
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

.play-title {
    font-size: 1.25rem;
    font-weight: 700;
}

.play-artist {
    font-size: 0.95rem;
    color: var(--app-text-secondary);
}

.play-title:disabled,
.play-artist:disabled {
    cursor: default;
}

.play-seek {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    width: 100%;
}

.play-range {
    width: 100%;
    accent-color: var(--app-accent);
}

.play-times {
    display: flex;
    justify-content: space-between;
}

.play-time {
    font-size: 0.75rem;
    font-family: var(--app-player-time-font);
    color: var(--app-text-secondary);
}

.play-transport {
    display: flex;
    align-items: center;
    /* Three buttons only (shuffle/repeat live in the queue heading), so the
       row spreads: thumb-sized gaps instead of a tight cluster. */
    gap: 2.5rem;
}

.play-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2.75rem;
    height: 2.75rem;
    border: none;
    background: none;
    color: var(--app-text-primary);
    cursor: pointer;
    font-size: 1.15rem;
}

.play-btn:disabled {
    color: var(--app-text-secondary);
    cursor: default;
}

.play-btn--play {
    width: 3.5rem;
    height: 3.5rem;
    border-radius: 50%;
    background-color: var(--app-accent);
    /* Against the accent disc the icon matches the view's surface, not the
       page background (near-white in light theme). */
    color: var(--app-player-bg);
    font-size: 1.3rem;
}

/* The two swipe affordances, one per direction: ⌄ at the top collapses the
   sheet, ⌃ under the transport reveals the queue. Real buttons, so neither
   destination is gesture-only. */
.play-nav-hint,
.play-swipe-hint {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2.75rem;
    height: 2rem;
    border: none;
    background: none;
    color: var(--app-text-secondary);
    cursor: pointer;
    font-size: 1.15rem;
}

.play-nav-hint {
    /* Sits at the face's top edge because .play-art's margin-top: auto absorbs
       the spare height below it; a short screen must squash the artwork, not
       this. */
    flex-shrink: 0;
}

/* Favorites read by the FILL alone, not by colour (see TrackFavoriteButton /
   unified-play-experience.md). Sits on the cover's bottom-right corner, so
   it needs a shadow, not a theme colour: the backdrop is arbitrary artwork. */
.play-favorite-indicator {
    position: absolute;
    right: 0.6rem;
    bottom: 0.6rem;
    font-size: 1rem;
    color: #fff;
    filter: drop-shadow(0 1px 2px rgb(0 0 0 / 0.6));
}
</style>
