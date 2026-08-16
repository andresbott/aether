<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { usePlayer } from '@/composables/usePlayer'
import { subsonicClient } from '@/lib/api/subsonic'
import { prefersReducedMotion } from '@/lib/motion'

const player = usePlayer()
const router = useRouter()

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

// The tap target is the Now Playing route: the play view is a first-class
// screen (MobilePlayView on `/`), not an overlay, so opening it is plain
// navigation. The bar is hidden on that route (see MobileShell), so this can
// never push a duplicate entry for the view already on screen.
const openNowPlaying = (): void => {
    void router.push({ name: 'home' })
}

// The bar doubles as the play screen's handle: LIFTING it opens Now Playing, the
// counterpart of the downward drag that leaves it (MobilePlayView). Same feel —
// the bar follows the finger, and only the release decides — because a gesture
// that does nothing until it fires reads as a jump.
//
// Past the commit distance the extra travel is rubber-banded: the pull is
// already going to open, and 1:1 to the top of the screen would fling a 3.5rem
// bar into the middle of the page.
const DRAG_COMMIT_PX = 64
const DRAG_RESIST = 0.4
// Mirrors this component's transform transition; read only by the safety timer.
const LEAVE_MS = 200
// Clears the strip the bar itself occupied, so the commit reads as "it left"
// rather than "it twitched".
const LEAVE_CLEARANCE_PX = 24

const bar = ref<HTMLElement | null>(null)
const dragY = ref(0)
const dragging = ref(false)
const leaving = ref(false)

let startY = 0
let armed = false
// A drag and a tap are the same touch until the finger moves: once it has, the
// click the browser may still deliver on release is not a tap any more, and
// acting on it would navigate on a cancelled pull or skip a track the user was
// only dragging from.
let tapSwallowed = false
let leaveTimer: ReturnType<typeof setTimeout> | null = null

const finishLeave = (): void => {
    if (!leaving.value) return
    leaving.value = false
    if (leaveTimer) {
        clearTimeout(leaveTimer)
        leaveTimer = null
    }
    openNowPlaying()
}

const liftAway = (): void => {
    if (leaving.value) return
    leaving.value = true
    dragging.value = false
    if (prefersReducedMotion()) {
        dragY.value = 0
        finishLeave()
        return
    }
    dragY.value = (bar.value?.offsetHeight ?? 0) + LEAVE_CLEARANCE_PX
    // A transition that never runs never ends (a dropped frame, a browser that
    // skips it), so the navigation cannot hang on the event alone.
    leaveTimer = setTimeout(finishLeave, LEAVE_MS + 80)
}

const onTransitionEnd = (event: TransitionEvent): void => {
    if (event.propertyName === 'transform') finishLeave()
}

const onTouchStart = (event: TouchEvent): void => {
    if (leaving.value) return
    armed = true
    tapSwallowed = false
    startY = event.touches[0]?.clientY ?? 0
}

const onTouchMove = (event: TouchEvent): void => {
    if (!armed) return
    // Up is positive: this bar only goes one way.
    const lift = startY - (event.touches[0]?.clientY ?? 0)
    if (lift <= 0 && !dragging.value) return
    dragging.value = true
    tapSwallowed = true
    dragY.value =
        lift <= DRAG_COMMIT_PX
            ? Math.max(0, lift)
            : DRAG_COMMIT_PX + (lift - DRAG_COMMIT_PX) * DRAG_RESIST
}

const onTouchEnd = (): void => {
    if (!armed) return
    armed = false
    if (!dragging.value) return
    dragging.value = false
    if (dragY.value >= DRAG_COMMIT_PX) liftAway()
    else dragY.value = 0
}

/** True once per gesture that moved: the release's click is not a tap. */
const swallowedTap = (): boolean => {
    if (!tapSwallowed) return false
    tapSwallowed = false
    return true
}

// A tap opens immediately — unlike the drag it has no motion to finish, and
// making it wait out a slide-out would only add latency to the bar's oldest,
// most-used interaction.
const onOpenClick = (): void => {
    if (swallowedTap()) return
    openNowPlaying()
}

const onPlayPauseClick = (): void => {
    if (swallowedTap()) return
    player.togglePlayPause()
}

const onNextClick = (): void => {
    if (swallowedTap()) return
    player.playNext()
}

onUnmounted(() => {
    if (leaveTimer) clearTimeout(leaveTimer)
})
</script>

<template>
    <!-- `--mini-lift` feeds the ::after strip that keeps the vacated space
         painted while the bar rides up, so no page background flashes below it. -->
    <div
        ref="bar"
        class="mini-player"
        :class="{ 'is-dragging': dragging, 'is-leaving': leaving }"
        :style="{ transform: `translateY(${-dragY}px)`, '--mini-lift': `${dragY}px` }"
        @touchstart.passive="onTouchStart"
        @touchmove.passive="onTouchMove"
        @touchend.passive="onTouchEnd"
        @touchcancel.passive="onTouchEnd"
        @transitionend="onTransitionEnd"
    >
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
            @click="onOpenClick"
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
            @click="onPlayPauseClick"
        >
            <i :class="player.isPlaying.value ? 'pi pi-pause' : 'pi pi-play'"></i>
        </button>
        <button type="button" class="mini-btn" aria-label="Next track" @click="onNextClick">
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
    /* The lift-to-open motion (see the script): a release animates, a finger
       owning it does not. Same curve as MobilePlayView's drag out, so the two
       halves of the same gesture pair match. */
    transition: transform 0.2s cubic-bezier(0.32, 0.72, 0, 1);
}

.mini-player.is-dragging {
    transition: none;
}

.mini-player.is-dragging,
.mini-player.is-leaving {
    will-change: transform;
}

/* The bar is the bottom-most surface in the shell column, so lifting it would
   expose the page background in the strip it left — including the safe-area
   inset it reserves. This keeps that strip painted for the length of the drag. */
.mini-player::after {
    content: '';
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    height: var(--mini-lift, 0px);
    background-color: var(--app-player-bg);
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
