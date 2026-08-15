<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import QueueBody from '@/components/layout/QueueBody.vue'
import QueueHeaderActions from '@/components/layout/QueueHeaderActions.vue'
import SavePlaylistDialog from '@/components/layout/SavePlaylistDialog.vue'
import { useCurrentTrackFavorite } from '@/composables/useCurrentTrackFavorite'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueActions } from '@/composables/useQueueActions'
import { useQueueEdit } from '@/composables/useQueueEdit'
import { useQueueSummary } from '@/composables/useQueueSummary'
import { subsonicClient } from '@/lib/api/subsonic'

// The phone's Now Playing, rendered by HomeView on the mobile shell. This is
// a first-class ROUTE view (hamburger in the scaffold header, entered and
// left through normal navigation, system back included) — not an overlay, so
// none of the history/focus-trap machinery the old PlayerSheet needed. Two
// panels, one scroller: the player face and the queue stack vertically in a
// snap container, so swiping the face up reveals the queue and swiping down
// from the queue's top returns — no header toggle. The scaffold header
// always carries the queue heading ("Queue" + track summary + the queue
// actions: shuffle/repeat inline, edit/save/clear behind the scaffold's ⋮
// overflow) so its height cannot change, but it is only
// REVEALED while the queue panel is up: invisible over the face (the artwork
// and track meta are the heading there), fading in as a swipe crosses the
// panel midpoint — not after the snap settles. The mini player is hidden on
// this route (see MobileShell), so the face carries the full transport and
// both panels reserve the bottom safe-area inset.
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
const scrollToQueue = (behavior: ScrollBehavior = 'smooth'): void => {
    programmaticTarget = 'queue'
    queuePanel.value?.scrollIntoView?.({ behavior, block: 'start' })
}
const scrollToFace = (behavior: ScrollBehavior = 'smooth'): void => {
    programmaticTarget = 'face'
    facePanel.value?.scrollIntoView?.({ behavior, block: 'start' })
}
const onPanelsTouchStart = (): void => {
    programmaticTarget = null
}

// Swipe-back assist (queue → face). The reverse of the reveal swipe cannot be
// left to native scroll chaining: the gesture starts on the queue's own
// scroller, and a chained drag hands the snap container almost none of the
// fling's momentum, so mandatory snap settles straight back on the queue. The
// list contains its overscroll instead (see the CSS) and this handler owns the
// switch: a downward pull that STARTS with the list already at its top is a
// return to the player, one per gesture.
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
        if (target === 'queue') scrollToQueue()
        else scrollToFace()
    }
)

onMounted(() => {
    // Arriving addressed to the queue (drawer entry, reload, shared link):
    // land on it, no animation from the face.
    if (route.hash === '#queue') {
        currentPanel.value = 'queue'
        scrollToQueue('auto')
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
    <!-- The queue heading is ALWAYS rendered so the header height cannot
         change with the panel switch; `queue-up` fades it in while the queue
         panel is the visible one (invisible over the face, where the artwork
         and track meta are the heading). -->
    <ContentScaffold
        class="mobile-play-view"
        :class="{ 'queue-up': currentPanel === 'queue' }"
        title="Queue"
        :summary="summary"
    >
        <!-- Shuffle and repeat are QUEUE behaviour, so they live with the
             queue heading rather than in the face's transport row; the
             management trio (edit/save/clear) collapses behind the scaffold's
             ⋮ overflow (#secondary-actions on phone tier), labeled because a
             popover stack of bare icons is not a menu. -->
        <template #actions>
            <Button
                class="queue-action-shuffle"
                icon="pi pi-arrow-right-arrow-left"
                text
                rounded
                size="small"
                :class="{ 'is-active': player.shuffle.value }"
                :aria-pressed="player.shuffle.value"
                aria-label="Shuffle"
                v-tooltip.bottom="'Shuffle'"
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
                v-tooltip.bottom="'Repeat'"
                @click="player.toggleRepeat()"
            />
        </template>
        <template #secondary-actions>
            <QueueHeaderActions
                :edit-mode="editMode"
                :disabled="trackCount === 0"
                size="small"
                labels
                @toggle-edit="toggleEditMode"
                @save="openSaveDialog"
                @clear="clearQueue"
            />
        </template>
        <div
            ref="panels"
            class="play-panels"
            @scroll.passive="onPanelsScroll"
            @touchstart.passive="onPanelsTouchStart"
        >
            <section ref="facePanel" class="play-panel play-face">
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

            <section
                ref="queuePanel"
                class="play-panel play-queue"
                @touchstart.passive="onQueueTouchStart"
                @touchmove.passive="onQueueTouchMove"
            >
                <div class="play-queue-list">
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
    </ContentScaffold>
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
       player chrome, not the page. Everything inside (scaffold header, queue
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
}

.mobile-play-view :deep(.content-scaffold-body) {
    display: flex;
    flex-direction: column;
    min-height: 0;
}

/* The queue heading ("Queue" over the track summary, plus the actions) is
   rendered in BOTH panel states — the header's height comes from real
   content that never changes, so the panel switch cannot move anything.
   This view owns the row geometry the stack needs: one non-wrapping row,
   centered cross-axis (a stacked block has no one baseline for the buttons
   to sit on), the title column dropping the scaffold's 12rem floor. */
.mobile-play-view :deep(.scaffold-header-inner) {
    flex-wrap: nowrap;
    align-items: center;
}

.mobile-play-view :deep(.scaffold-title) {
    flex-direction: column;
    align-items: flex-start;
    gap: 0;
    min-width: 0;
}

.mobile-play-view :deep(.scaffold-summary) {
    /* The scaffold's phone layout gives the summary flex-basis: 100% to drop
       it onto its own row; in this column stack that would be 100% of the
       block's height — reset it to content size. */
    flex-basis: auto;
    max-width: 100%;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

/* The reveal: invisible over the player face, fading in the moment the
   switch is requested — a swipe crossing the panel midpoint or a tap on the
   hint/drawer entry — so the fade runs DURING the snap animation instead of
   popping after it. `visibility` (delayed until the fade-out ends) keeps the
   hidden heading out of the accessibility tree and off the tap surface while
   still occupying layout. */
.mobile-play-view :deep(.scaffold-title),
.mobile-play-view :deep(.scaffold-actions) {
    opacity: 0;
    visibility: hidden;
    transition:
        opacity 0.18s ease,
        visibility 0s linear 0.18s;
}

.mobile-play-view.queue-up :deep(.scaffold-title),
.mobile-play-view.queue-up :deep(.scaffold-actions) {
    opacity: 1;
    visibility: visible;
    transition: opacity 0.18s ease;
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
    padding: 0 1.5rem calc(0.5rem + env(safe-area-inset-bottom));
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
       capped by height so the transport keeps room on short screens. */
    width: min(100%, 45vh);
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

/* The affordance for the swipe: a chevron under the transport, and a real
   button so the queue stays reachable without the gesture. */
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
