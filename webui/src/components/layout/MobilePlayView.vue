<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import Popover from 'primevue/popover'
import QueueBody from '@/components/layout/QueueBody.vue'
import QueueHeaderActions from '@/components/layout/QueueHeaderActions.vue'
import SavePlaylistDialog from '@/components/layout/SavePlaylistDialog.vue'
import { useCurrentTrackFavorite } from '@/composables/useCurrentTrackFavorite'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueActions } from '@/composables/useQueueActions'
import { useQueueEdit } from '@/composables/useQueueEdit'
import { useQueueSummary } from '@/composables/useQueueSummary'
import { subsonicClient } from '@/lib/api/subsonic'
import { prefersReducedMotion } from '@/lib/motion'

// The phone's Now Playing, rendered by HomeView on the mobile shell. This is
// a first-class ROUTE view (entered and left through normal navigation, system
// back included) — not an overlay, so none of the history/focus-trap machinery
// the old PlayerSheet needed. Two panels, one scroller: the player face and
// the queue stack vertically in a snap container, so swiping the face up
// reveals the queue, and swiping down from the queue's top — or dragging the
// queue heading, which works at any list position — returns; no header toggle.
//
// Deliberately NOT a ContentScaffold host, the one main-content view that
// isn't (docs/architecture/main-content-view-layout.md): a fixed header above
// both panels is a bar the player face has no use for — it showed a heading
// that belongs to the queue, ate the artwork's height, and put its hamburger
// exactly where the browser's URL bar sits. So the heading ("Queue" + track
// summary + shuffle/repeat inline, edit/save/clear behind its ⋮) is part of
// the QUEUE panel and arrives with it, and the face is bare: a downward swipe
// there — with the ⌄ hint button as its non-gesture twin — is what the
// scaffold's hamburger used to be, navigating to /browse. The mini player is
// hidden on this route (see MobileShell), so the face carries the full
// transport, and both panels reserve the bottom safe-area inset.
const player = usePlayer()
const router = useRouter()
const route = useRoute()
const { isStarred, toggleFavorite } = useCurrentTrackFavorite()
const { showSaveDialog, playlistName, openSaveDialog, handleSave, isSaving, clearQueue } =
    useQueueActions()
const { editMode, toggleEditMode, exitEditMode } = useQueueEdit()
const { trackCount, summary } = useQueueSummary()

const currentTrack = computed(() => player.currentTrack.value)

const coverUrl = computed(() => {
    const art = currentTrack.value?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 512)
})

// The swipe is the primary way between the panels; the hint chevron under the
// transport is a real button to the same place, so pointer and AT users are
// not locked out of a gesture-only path.
const panels = ref<HTMLElement | null>(null)
const facePanel = ref<HTMLElement | null>(null)
const queuePanel = ref<HTMLElement | null>(null)

// While one of these programmatic scrolls is in flight, the scroll handler
// must not read the passing positions as a user swipe: a smooth scroll to the
// queue starts on the face's side of the midpoint, and the handler would flip
// the panel (and the hash) straight back. The flag names the destination and
// the handler stays quiet until the scroll crosses to it; a touch on the
// container cancels it, because a finger taking over IS a user swipe again.
let programmaticTarget: 'face' | 'queue' | null = null

// The switch distance: both panels are one container tall, so the queue's
// start IS the offset — measured off the panels rather than assumed, so a
// layout change can't silently drift it (jsdom reports 0 for both, hence the
// clientHeight fallback).
const panelOffset = (panel: 'face' | 'queue'): number => {
    if (panel === 'face') return 0
    const face = facePanel.value
    const queue = queuePanel.value
    const delta = face && queue ? queue.offsetTop - face.offsetTop : 0
    return delta > 0 ? delta : (panels.value?.clientHeight ?? 0)
}

// Scrolls ONLY this container — never scrollIntoView on a panel, which reveals
// the element in every scrollable ANCESTOR and in the visual viewport too. On
// mobile Chrome the URL bar shrinks the visual viewport while the layout
// viewport stays large, so there is always URL-bar-height of room to offset it:
// revealing a panel slid the whole app up under the URL bar and left the layout
// viewport's tail as dead space above the system nav — and it stuck there,
// since nothing at document level can scroll it back. Same lesson as
// QueueBody's row scrolling; device emulation shows neither symptom, because it
// has no separate visual viewport.
const scrollToPanel = (panel: 'face' | 'queue', behavior: ScrollBehavior = 'smooth'): void => {
    programmaticTarget = panel
    panels.value?.scrollTo?.({ top: panelOffset(panel), behavior })
}
const onPanelsTouchStart = (): void => {
    programmaticTarget = null
}

// The way OUT of Now Playing, and the face's replacement for the scaffold
// hamburger: dragging the player face down leaves for /browse, the phone's nav
// surface. The gesture is free there — the face is the snap container's first
// panel, so a downward drag has nothing to scroll to and the container contains
// its overscroll.
//
// It has to FEEL like its mirror image, the swipe up to the queue: that one is
// a native scroll, so the panel tracks the finger and a release either settles
// or springs back. A threshold that fired mid-gesture and navigated jumped
// instead, with nothing moving until it did. So the view follows the finger 1:1
// (`dragY` translates the whole screen, transition suppressed while a finger
// owns it), and only a RELEASE decides: past the commit distance it slides the
// rest of the way out and then navigates, short of it it springs back to 0.
//
// The commit distance scales with the screen (a fifth of it) with a floor for
// short ones, and the seek bar is exempt from arming at all, or dragging the
// slider a little off-axis would start pulling the view away mid-seek.
const DRAG_COMMIT_RATIO = 0.2
const DRAG_COMMIT_MIN_PX = 64
// Mirrors the transform transition in this component's CSS; only the safety
// timer reads it, so a small drift costs nothing — it just fires a little early
// or late for a transitionend that already handles the normal case.
const LEAVE_MS = 240

const viewRoot = ref<HTMLElement | null>(null)
const dragY = ref(0)
// Separate from `dragY > 0`: a finger resting at 0 after pulling back up still
// owns the motion, and the transition must stay off until it lifts.
const dragging = ref(false)
// Committed and animating out; the navigation waits for the slide to finish.
const leaving = ref(false)

let faceStartY = 0
let faceSwipeArmed = false
let leaveTimer: ReturnType<typeof setTimeout> | null = null

const viewHeight = (): number => viewRoot.value?.offsetHeight ?? 0

// Idempotent: whichever of transitionend / the safety timer arrives first wins.
// The timer exists because a transition that never runs never ends — reduced
// motion turns it off, and the view would sit half-off-screen forever.
const finishLeave = (): void => {
    if (!leaving.value) return
    leaving.value = false
    if (leaveTimer) {
        clearTimeout(leaveTimer)
        leaveTimer = null
    }
    // push(), like the hamburger it replaces: system-back returns to the player.
    void router.push({ name: 'browse' })
}

const leaveForBrowse = (): void => {
    if (leaving.value) return
    leaving.value = true
    dragging.value = false
    // Asked for no motion: go straight there rather than sit through a
    // slide-out — or worse, wait on the safety timer for a transition that will
    // never run.
    if (prefersReducedMotion()) {
        dragY.value = 0
        finishLeave()
        return
    }
    // Slide the rest of the way off before the route swap, so the motion the
    // finger started carries through to the end.
    dragY.value = viewHeight()
    leaveTimer = setTimeout(finishLeave, LEAVE_MS + 80)
}

const onLeaveTransitionEnd = (event: TransitionEvent): void => {
    if (event.propertyName === 'transform') finishLeave()
}

const onFaceTouchStart = (event: TouchEvent): void => {
    if (leaving.value) return
    const onSeekBar = event.target instanceof Element && !!event.target.closest('.play-seek')
    // Only with the face fully settled: part-way to the queue, a downward drag
    // is the user scrolling back to the face, not asking to leave.
    faceSwipeArmed =
        !onSeekBar && currentPanel.value === 'face' && (panels.value?.scrollTop ?? 0) <= 0
    faceStartY = event.touches[0]?.clientY ?? 0
}

const onFaceTouchMove = (event: TouchEvent): void => {
    if (!faceSwipeArmed) return
    const deltaY = (event.touches[0]?.clientY ?? 0) - faceStartY
    // Downward only, and never past 0 on the way back: an upward drag is the
    // queue reveal, which is the scroller's business, not ours.
    if (deltaY <= 0 && !dragging.value) return
    dragging.value = true
    dragY.value = Math.max(0, deltaY)
}

const onFaceTouchEnd = (): void => {
    if (!dragging.value) {
        faceSwipeArmed = false
        return
    }
    dragging.value = false
    faceSwipeArmed = false
    const commitAt = Math.max(DRAG_COMMIT_MIN_PX, viewHeight() * DRAG_COMMIT_RATIO)
    if (dragY.value >= commitAt) leaveForBrowse()
    else dragY.value = 0
}

onUnmounted(() => {
    if (leaveTimer) clearTimeout(leaveTimer)
})

// Swipe-back assist (queue → face). The reverse of the reveal swipe cannot be
// left to native scroll chaining: the gesture starts on the queue's own
// scroller, and a chained drag hands the snap container almost none of the
// fling's momentum, so mandatory snap settles straight back on the queue. The
// list contains its overscroll instead (see the CSS) and this handler owns the
// switch: a downward pull that STARTS with the list already at its top is a
// return to the player, one per gesture. The listeners sit on the LIST rather
// than the whole panel, so a drag on the heading above it (see below) is that
// heading's own gesture and not two handlers racing to the same destination.
const SWIPE_BACK_PX = 48
let touchStartY = 0
let swipeBackArmed = false

const queueScrolledDown = (target: EventTarget | null): boolean => {
    let el = target instanceof HTMLElement ? target : null
    while (el && el !== queuePanel.value) {
        if (el.scrollTop > 0) return true
        el = el.parentElement
    }
    return false
}

const onQueueTouchStart = (event: TouchEvent): void => {
    touchStartY = event.touches[0]?.clientY ?? 0
    swipeBackArmed = !queueScrolledDown(event.target)
}

const onQueueTouchMove = (event: TouchEvent): void => {
    if (!swipeBackArmed) return
    const deltaY = (event.touches[0]?.clientY ?? 0) - touchStartY
    if (deltaY > SWIPE_BACK_PX) {
        swipeBackArmed = false
        showPanel('face')
    }
}

// The queue heading is the queue's escape hatch, and the reason it needs to be
// one: the list gesture above only arms with the list ALREADY at its top, so
// reading down a long queue means scrolling all the way back up before the
// pull works at all. Dragging the heading switches at any list position.
//
// Unlike the list, the heading is not a scroller — a drag there cannot mean
// "move this content", so it takes EITHER direction past the same threshold
// rather than making the user guess which way the panel stack runs. Only from
// the queue: the heading rides in the queue panel, so while the face is up it
// is scrolled off-screen and there is nowhere to go back to anyway.
let headerStartY = 0
let headerSwipeArmed = false

const onHeaderTouchStart = (event: TouchEvent): void => {
    headerSwipeArmed = currentPanel.value === 'queue'
    headerStartY = event.touches[0]?.clientY ?? 0
}

const onHeaderTouchMove = (event: TouchEvent): void => {
    if (!headerSwipeArmed) return
    const deltaY = (event.touches[0]?.clientY ?? 0) - headerStartY
    if (Math.abs(deltaY) > SWIPE_BACK_PX) {
        headerSwipeArmed = false
        showPanel('face')
    }
}

// `/#queue` is the queue panel's address: the nav drawer's Queue entry
// navigates there, and a reload reopens on the panel the user left. The hash
// follows a manual swipe too, flipping as the swipe CROSSES THE MIDPOINT —
// not once the snap settles — so the header fade runs during the gesture
// instead of popping in after it. Programmatic scrolls are exempt (see
// programmaticTarget above). The drawer's Now Playing / Queue highlight
// therefore never lies about the visible panel.
const currentPanel = ref<'face' | 'queue'>('face')
const onPanelsScroll = (): void => {
    const el = panels.value
    if (!el) return
    const panel = el.scrollTop > el.clientHeight / 2 ? 'queue' : 'face'
    if (programmaticTarget) {
        if (panel === programmaticTarget) programmaticTarget = null
        return
    }
    if (panel === currentPanel.value) return
    currentPanel.value = panel
    void router.replace({ hash: panel === 'queue' ? '#queue' : '' })
}

// Edit mode is queue-panel UI: leaving the queue for the player face — by
// swipe, hint button or hash — ends the editing session, so returning to the
// queue never lands on a stale selection. Watching currentPanel catches every
// path, since both the scroll handler and the hash watcher set it.
watch(currentPanel, (panel) => {
    if (panel === 'face') exitEditMode()
})

// The non-gesture paths (hint chevron, swipe-back assist) go THROUGH the
// hash rather than scrolling directly: the hash watcher below is then the
// one place that flips the panel state and starts the scroll, so the header
// reveal always begins the moment the switch is requested.
const showPanel = (panel: 'face' | 'queue'): void => {
    void router.replace({ hash: panel === 'queue' ? '#queue' : '' })
}

watch(
    () => route.hash,
    (hash) => {
        const target = hash === '#queue' ? 'queue' : 'face'
        if (target === currentPanel.value) return
        // Set before the scroll so the settle timer doesn't rewrite the hash
        // mid-flight with the panel the smooth scroll is still leaving.
        currentPanel.value = target
        scrollToPanel(target)
    }
)

onMounted(() => {
    // Arriving addressed to the queue (drawer entry, reload, shared link):
    // land on it, no animation from the face.
    if (route.hash === '#queue') {
        currentPanel.value = 'queue'
        scrollToPanel('queue', 'auto')
    }
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

// The queue-management trio (edit/save/clear) behind the heading's ⋮ — three
// more bare glyphs next to shuffle/repeat would not read as a toolbar on a
// phone. Labeled inside the popover, since tooltips don't exist on touch. Same
// arrangement ContentScaffold gives its #secondary-actions on phone tier; this
// view owns it directly now that it has no scaffold.
const overflowRef = ref<InstanceType<typeof Popover> | null>(null)
const toggleOverflow = (event: Event) => overflowRef.value?.toggle(event)

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
    <!-- The transform is bound even at rest, so the spring-back animates FROM a
         real value to translateY(0px) rather than to a removed transform. Nothing
         inside is position: fixed (every overlay here teleports to body), so the
         containing block it establishes costs nothing. -->
    <div
        ref="viewRoot"
        class="mobile-play-view"
        :class="{ 'is-dragging': dragging, 'is-leaving': leaving }"
        :style="{ transform: `translateY(${dragY}px)` }"
        @transitionend="onLeaveTransitionEnd"
    >
        <div
            ref="panels"
            class="play-panels"
            @scroll.passive="onPanelsScroll"
            @touchstart.passive="onPanelsTouchStart"
        >
            <section
                ref="facePanel"
                class="play-panel play-face"
                @touchstart.passive="onFaceTouchStart"
                @touchmove.passive="onFaceTouchMove"
                @touchend.passive="onFaceTouchEnd"
                @touchcancel.passive="onFaceTouchEnd"
            >
                <!-- The face's only chrome, and the non-gesture twin of its
                     drag: where every other view carries the hamburger, this
                     one carries a chevron to the same place — through the same
                     slide-out, so both paths look alike. -->
                <button
                    type="button"
                    class="play-nav-hint"
                    aria-label="Open navigation"
                    @click="leaveForBrowse"
                >
                    <i class="pi pi-angle-down" aria-hidden="true"></i>
                </button>

                <div class="play-art" @click="onArtTap">
                    <img v-if="coverUrl" :src="coverUrl" alt="" class="play-cover" />
                    <div v-else class="play-cover play-cover--placeholder" aria-hidden="true">
                        <i class="pi pi-music"></i>
                    </div>
                    <!-- Passive echo of the starred state, set by double-tapping
                     the cover. Decorative only (the queue face's row hearts
                     are the accessible toggle), hence aria-hidden. -->
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

                <!-- Prev / play / next only: shuffle and repeat moved up to
                     the queue header (see the #actions slot). -->
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
                    @click="showPanel('queue')"
                >
                    <i class="pi pi-angle-up" aria-hidden="true"></i>
                </button>
            </section>

            <section ref="queuePanel" class="play-panel play-queue">
                <!-- The heading belongs to the queue, so it lives in the queue's
                     panel and arrives with it — no fixed bar over the player
                     face, and nothing to fade. Dragging it either way is the
                     escape hatch back to the face (see onHeaderTouchStart). -->
                <header
                    class="queue-heading"
                    @touchstart.passive="onHeaderTouchStart"
                    @touchmove.passive="onHeaderTouchMove"
                >
                    <div class="queue-heading-text">
                        <h2>Queue</h2>
                        <span class="queue-heading-summary">{{ summary }}</span>
                    </div>
                    <!-- Shuffle and repeat are QUEUE behaviour, so they sit with
                         the queue heading rather than in the face's transport
                         row; edit/save/clear collapse behind the ⋮. -->
                    <div class="queue-heading-actions">
                        <Button
                            class="queue-action-shuffle"
                            icon="pi pi-arrow-right-arrow-left"
                            text
                            rounded
                            size="small"
                            :class="{ 'is-active': player.shuffle.value }"
                            :aria-pressed="player.shuffle.value"
                            aria-label="Shuffle"
                            @click="player.toggleShuffle()"
                        />
                        <Button
                            class="queue-action-repeat"
                            icon="pi pi-refresh"
                            text
                            rounded
                            size="small"
                            :class="{ 'is-active': player.repeat.value !== 'none' }"
                            :aria-pressed="player.repeat.value !== 'none'"
                            aria-label="Repeat"
                            @click="player.toggleRepeat()"
                        />
                        <Button
                            class="queue-overflow-btn"
                            icon="pi pi-ellipsis-v"
                            text
                            rounded
                            size="small"
                            aria-label="More actions"
                            @click="toggleOverflow"
                        />
                        <Popover ref="overflowRef">
                            <div class="queue-overflow-panel">
                                <QueueHeaderActions
                                    :edit-mode="editMode"
                                    :disabled="trackCount === 0"
                                    size="small"
                                    labels
                                    @toggle-edit="toggleEditMode"
                                    @save="openSaveDialog"
                                    @clear="clearQueue"
                                />
                            </div>
                        </Popover>
                    </div>
                </header>
                <div
                    class="play-queue-list"
                    @touchstart.passive="onQueueTouchStart"
                    @touchmove.passive="onQueueTouchMove"
                >
                    <QueueBody variant="sidebar" :edit-mode="editMode" />
                </div>
            </section>
        </div>

        <SavePlaylistDialog
            v-model:visible="showSaveDialog"
            v-model:name="playlistName"
            :saving="isSaving"
            @save="handleSave"
        />
    </div>
</template>

<style scoped>
.mobile-play-view {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    width: 100%;
    /* Now Playing keeps the player-bar palette (the dark blue surface the old
       full-screen sheet used) in BOTH themes — the transport belongs to the
       player chrome, not the page. Everything inside (queue heading, queue
       rows, transport) colours itself with the app tokens, so remap those for
       the subtree rather than forking the children; custom properties inherit
       through the DOM, so this reaches their scoped rules.
       --app-accent is deliberately NOT remapped: it is the "this is playing"
       signal and already clears 5.2:1 on the player background in both themes. */
    background-color: var(--app-player-bg);
    color: var(--app-player-text);
    --app-text-primary: var(--app-player-text);
    --app-text-secondary: var(--app-player-dim);
    --app-hover: color-mix(in srgb, var(--app-player-text) 12%, transparent);
    --app-border: color-mix(in srgb, var(--app-player-text) 20%, transparent);
    /* The light-theme soft accent is mixed for a white surface and all but
       vanishes here; strengthen it so the now-playing strip stays readable. */
    --app-accent-soft: color-mix(in srgb, var(--app-accent) 20%, transparent);
    /* The drag-to-leave motion (see the script): a release animates the
       transform, a finger owning it does not — .is-dragging turns this off so
       the view sits exactly where the finger put it, frame for frame. The curve
       is scroll-snap's shape: quick to leave, settling at the end. */
    transition: transform 0.24s cubic-bezier(0.32, 0.72, 0, 1);
}

.mobile-play-view.is-dragging {
    transition: none;
}

.mobile-play-view.is-dragging,
.mobile-play-view.is-leaving {
    will-change: transform;
}

/* The swipe surface: both panels stack in this scroller, and mandatory snap
   means a released swipe always settles on a whole panel — the view never
   rests half-player, half-queue. */
.play-panels {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    scroll-snap-type: y mandatory;
    /* The panel switch must never chain into the page: on Android the
       viewport is next in line and an overshot swipe-back would trigger
       pull-to-refresh mid-gesture. */
    overscroll-behavior-y: contain;
}

.play-panel {
    height: 100%;
    scroll-snap-align: start;
    /* Two panels only, but a long fling must still stop at the queue's top
       rather than sail past the snap point. */
    scroll-snap-stop: always;
}

.play-queue {
    display: flex;
    flex-direction: column;
    min-height: 0;
}

/* The queue's own heading, inside its panel: it scrolls in with the queue, so
   nothing hovers over the player face and there is no height to keep constant
   across the switch. Reserves the TOP inset — it is the topmost surface on this
   panel, and in a standalone launch the status bar overlaps it. */
.queue-heading {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-shrink: 0;
    box-sizing: border-box;
    padding: calc(0.5rem + env(safe-area-inset-top)) var(--app-content-gutter) 0.5rem;
    border-bottom: 1px solid var(--app-border);
}

.queue-heading-text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
}

/* h2, not h1: the page heading on this route is the track on the player face,
   and the queue is a section of it. Sized like the scaffold's phone title. */
.queue-heading h2 {
    margin: 0;
    font-size: 1.2rem;
    font-weight: 700;
}

.queue-heading-summary {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.queue-heading-actions {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    flex-shrink: 0;
}

/* Menu rows in a column, same as the scaffold's overflow panel. The Popover
   teleports to body but keeps this component's scope attribute, so the scoped
   rule still reaches it. */
.queue-overflow-panel {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
}

.play-queue-list {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
    /* No mini player on this route, so the list is the bottom-most surface
       and reserves the home-indicator inset itself. */
    padding-bottom: env(safe-area-inset-bottom);
}

/* The list (and the QueueBody scroller inside it) CONTAINS its overscroll:
   native chaining into the mandatory-snap container is not a usable swipe
   back — the fling's momentum stays with the list, so the snap just settles
   back on the queue — and an uncontained hard fling to the list's top would
   yank the player face in by accident. The deliberate queue → face switch is
   the panel's touch handler instead (see onQueueTouchStart/Move). */
.play-queue-list,
.play-queue-list :deep(.queue-body) {
    overscroll-behavior-y: contain;
}

.play-face {
    position: relative;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1.25rem;
    /* Reserves BOTH insets: with no header above it and no mini player below,
       the face is the whole screen on this route — the status bar (or the
       notch, in landscape) would otherwise sit on the nav chevron. */
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
    /* Three buttons only (shuffle/repeat live in the queue header), so the
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

/* Same "pressed" convention as the queue pencil (QueueHeaderActions): a
   toggle that is on carries the soft accent fill. */
.queue-action-shuffle.is-active,
.queue-action-repeat.is-active {
    background: var(--app-accent-soft);
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

/* The two swipe affordances, one per direction: ⌄ at the top for the way out
   to /browse, ⌃ under the transport for the queue. Real buttons, so neither
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
