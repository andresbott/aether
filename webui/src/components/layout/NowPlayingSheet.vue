<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import MiniPlayer from '@/components/layout/MiniPlayer.vue'
import PlayerFace from '@/components/layout/PlayerFace.vue'
import QueuePanel from '@/components/layout/QueuePanel.vue'
import {
    commitDetent,
    detentForHash,
    useNowPlayingSheet,
    type SheetDetent,
    DETENTS
} from '@/composables/useNowPlayingSheet'
import {
    dragPosition,
    settleDetent,
    SLOP_PX,
    VelocityTracker
} from '@/lib/sheetGesture'

// The phone's Now Playing, as one always-mounted bottom sheet over the route
// content (rendered by MobileShell; spec:
// docs/superpowers/specs/2026-08-16-mobile-sheet-navigation-design.md).
// Three detents — collapsed (mini strip) / playing (face) / queue — addressed
// by the hash of WHATEVER route is underneath: '' / #playing / #queue. The
// hash is the single source of truth: buttons and gestures commit it
// (commitDetent), this component's watcher animates toward it, and system
// back therefore walks queue → playing → page with no popstate machinery.
// The content view never unmounts, so a dismiss drag reveals the live page.
const router = useRouter()
const route = useRoute()
const { detent, position, dragging, snapTo } = useNowPlayingSheet()

// Mount the sheet onto the detent indicated by the route hash immediately,
// so the first render shows the correct position without animation.
snapTo(detentForHash(route.hash))

const root = ref<HTMLElement | null>(null)
const strip = ref<HTMLElement | null>(null)

// --- transforms -----------------------------------------------------------
// position 0..1 raises the sheet from its strip to full height; 1..2 shifts
// the two-panel track from the face to the queue. The strip height stays in
// CSS (calc against the same tokens MiniPlayer sizes itself with), so no
// resize observer is needed for rendering — JS only measures for gestures.
const expandF = computed(() => Math.min(Math.max(position.value, 0), 1))
const queueF = computed(() => Math.min(Math.max(position.value - 1, 0), 1))

const sheetStyle = computed(() => ({
    transform: `translateY(calc((100% - (var(--app-mini-player-height) + env(safe-area-inset-bottom))) * ${1 - expandF.value}))`
}))
const trackStyle = computed(() => ({
    transform: `translateY(${-queueF.value * 50}%)`
}))
// The strip cross-fades out over the first half of the rise, while the face
// underneath is already lit — the bar melts into the player it becomes.
const stripStyle = computed(() => ({
    opacity: String(Math.max(0, 1 - expandF.value * 2))
}))
const stripHidden = computed(() => expandF.value >= 0.5)

// --- gestures ---------------------------------------------------------------
// One drag controller for four surfaces; each surface arms a direction and a
// range, and every claimed drag maps the finger 1:1 through sheetGesture's
// travel model. A claim needs SLOP_PX of dominant-vertical movement, so taps,
// the transport buttons and the seek slider never notice a wobble.
type DragSurface = 'strip' | 'face' | 'queue' | 'heading'
const RANGES: Record<DragSurface, [number, number]> = {
    strip: [0, 1],
    face: [0, 2],
    queue: [1, 2],
    heading: [1, 2]
}

interface ActiveDrag {
    surface: DragSurface
    startY: number
    startX: number
    startPos: number
    min: number
    max: number
    claimed: boolean
    denied: boolean
    viewportH: number
    stripH: number
    tracker: VelocityTracker
}

let drag: ActiveDrag | null = null
// A drag and a tap are the same touch until the finger moves: once it has,
// the click the browser still delivers on release is not a tap any more.
let swallowClick = false

const queuePanelEl = ref<HTMLElement | null>(null)

// True when anything between the touch target and the queue panel has been
// scrolled: that pull belongs to the list, not to the sheet.
const queueScrolledDown = (target: EventTarget | null): boolean => {
    let el = target instanceof HTMLElement ? target : null
    while (el && el !== queuePanelEl.value) {
        if (el.scrollTop > 0) return true
        el = el.parentElement
    }
    return false
}

const beginDrag = (surface: DragSurface, event: TouchEvent): void => {
    const touch = event.touches[0]
    if (!touch) return
    const denied =
        (surface === 'face' &&
            event.target instanceof Element &&
            !!event.target.closest('.play-seek')) ||
        (surface === 'queue' && queueScrolledDown(event.target))
    const [min, max] = RANGES[surface]
    const tracker = new VelocityTracker()
    tracker.push(touch.clientY, event.timeStamp)
    drag = {
        surface,
        startY: touch.clientY,
        startX: touch.clientX,
        startPos: position.value,
        min,
        max,
        claimed: false,
        denied,
        viewportH: 1,
        stripH: 0,
        tracker
    }
}

const onStripTouchStart = (event: TouchEvent): void => beginDrag('strip', event)
const onFaceTouchStart = (event: TouchEvent): void => beginDrag('face', event)
// The heading is the queue's escape hatch AT ANY list position (the list pull
// only arms at its top); same range, different arming.
const onQueueTouchStart = (event: TouchEvent): void => {
    const onHeading =
        event.target instanceof Element && !!event.target.closest('.queue-heading')
    beginDrag(onHeading ? 'heading' : 'queue', event)
}

const moveDrag = (event: TouchEvent): void => {
    const active = drag
    if (!active || active.denied) return
    const touch = event.touches[0]
    if (!touch) return
    const dy = touch.clientY - active.startY
    if (!active.claimed) {
        const dx = touch.clientX - active.startX
        if (Math.abs(dy) <= SLOP_PX || Math.abs(dy) <= Math.abs(dx)) return
        // Direction gates: the strip only lifts, the queue surfaces only pull
        // down (up is the list's business); the face goes both ways.
        const gate =
            active.surface === 'strip' ? dy < 0 : active.surface === 'face' ? true : dy > 0
        if (!gate) {
            active.denied = true
            return
        }
        active.claimed = true
        // Measured at claim time, not per frame: the drag needs one stable
        // travel mapping, and mid-gesture URL-bar movement would warp it.
        active.viewportH = root.value?.offsetHeight || 1
        active.stripH = strip.value?.offsetHeight ?? 0
        dragging.value = true
        swallowClick = true
    }
    // Once claimed, the queue surfaces own the touch outright: without
    // preventDefault the list (at its top) still elastic-scrolls against the
    // drag on iOS. Their touchmove is bound WITHOUT .passive for exactly this.
    if (event.cancelable && (active.surface === 'queue' || active.surface === 'heading')) {
        event.preventDefault()
    }
    active.tracker.push(touch.clientY, event.timeStamp)
    position.value = dragPosition(
        active.startPos,
        dy,
        active.viewportH,
        active.stripH,
        active.min,
        active.max
    )
}

const endDrag = (): void => {
    const active = drag
    drag = null
    if (!active?.claimed) return
    dragging.value = false
    settleTo(settleDetent(position.value, active.tracker.velocity(), active.min, active.max))
}

// Settle: the position is already at the target, so set the detent FIRST and
// let commitDetent navigate — the hash watcher then sees hash and detent
// agree and does nothing.
const settleTo = (index: number): void => {
    const to = DETENTS[index]
    const from = detent.value
    position.value = index
    detent.value = to
    commitDetent(router, from, to)
}

const onRootClickCapture = (event: MouseEvent): void => {
    if (!swallowClick) return
    swallowClick = false
    event.preventDefault()
    event.stopPropagation()
}

// --- route sync ------------------------------------------------------------
// Buttons (mini tap, face ⌄/⌃) go THROUGH the hash: commitDetent navigates,
// the watcher below is the one place that moves the sheet. Back button and
// deep links arrive through the same watcher for free.
const requestDetent = (to: SheetDetent): void => {
    commitDetent(router, detent.value, to)
}

watch(
    () => route.hash,
    (hash) => {
        const target = detentForHash(hash)
        if (target !== detent.value) snapTo(target)
    }
)

// Transitions are suppressed until after the first paint, so arriving
// addressed to a detent (reload, shared link) lands there with no animation.
const ready = ref(false)
onMounted(() => {
    // Let the browser paint the initial position, then enable animations for
    // future gestures and navigation.
    requestAnimationFrame(() => {
        ready.value = true
    })
})

// Unmounting (queue emptied, shell flipped to desktop) with a live sheet hash
// would leave a stale #playing on a chrome that has no sheet: strip it.
onUnmounted(() => {
    if (detentForHash(route.hash) !== 'collapsed') void router.replace({ hash: '' })
    snapTo('collapsed')
})
</script>

<template>
    <div
        ref="root"
        class="now-playing-sheet"
        :class="{ 'is-dragging': dragging || !ready }"
        :style="sheetStyle"
        @click.capture="onRootClickCapture"
    >
        <!-- Collapsed, everything above the strip is off-screen; inert keeps
             it out of the tab order and the AT tree without unmounting it. -->
        <div class="sheet-body" :inert="detent === 'collapsed' || undefined">
            <div class="sheet-track" :style="trackStyle">
                <section
                    class="sheet-panel sheet-panel-face"
                    @touchstart.passive="onFaceTouchStart"
                    @touchmove.passive="moveDrag"
                    @touchend.passive="endDrag"
                    @touchcancel.passive="endDrag"
                >
                    <PlayerFace
                        @collapse="requestDetent('collapsed')"
                        @show-queue="requestDetent('queue')"
                    />
                </section>
                <!-- touchmove deliberately NOT passive here: a claimed pull
                     preventDefaults so the list cannot fight the drag. -->
                <section
                    ref="queuePanelEl"
                    class="sheet-panel sheet-panel-queue"
                    @touchstart.passive="onQueueTouchStart"
                    @touchmove="moveDrag"
                    @touchend.passive="endDrag"
                    @touchcancel.passive="endDrag"
                >
                    <QueuePanel />
                </section>
            </div>
        </div>
        <!-- After the body in the DOM so it paints above the face's top edge
             while collapsed; inert + pointer-events off once it has faded. -->
        <div
            ref="strip"
            class="sheet-strip"
            :class="{ 'strip-hidden': stripHidden }"
            :style="stripStyle"
            :inert="stripHidden || undefined"
            @touchstart.passive="onStripTouchStart"
            @touchmove.passive="moveDrag"
            @touchend.passive="endDrag"
            @touchcancel.passive="endDrag"
        >
            <MiniPlayer @open="requestDetent('playing')" />
        </div>
    </div>
</template>

<style scoped>
.now-playing-sheet {
    /* Absolute against .player-shell (PlayerLayout gives it position:
       relative), NOT position: fixed: the shell already owns the one
       dvh-measured box (appShell.spec.ts), and a fixed box would re-measure
       the viewport on its own terms while the URL bar moves. Below 1000 so
       every PrimeVue overlay (dialogs, popovers, toasts) stays on top. */
    position: absolute;
    inset: 0;
    z-index: 100;
    overflow: hidden;
    /* A drag at the sheet's edge must never chain into the page: on Android
       the viewport is next in line and would pull-to-refresh mid-gesture. */
    overscroll-behavior: contain;
    /* Now Playing keeps the player-bar palette (the dark blue surface) in
       BOTH themes — the transport belongs to the player chrome, not the
       page. Everything inside (queue heading, queue rows, transport) colours
       itself with the app tokens, so remap those for the subtree rather than
       forking the children; custom properties inherit through the DOM, so
       this reaches their scoped rules.
       --app-accent is deliberately NOT remapped: it is the "this is playing"
       signal and already clears 5.2:1 on the player background in both
       themes. */
    background-color: var(--app-player-bg);
    color: var(--app-player-text);
    --app-text-primary: var(--app-player-text);
    --app-text-secondary: var(--app-player-dim);
    --app-hover: color-mix(in srgb, var(--app-player-text) 12%, transparent);
    --app-border: color-mix(in srgb, var(--app-player-text) 20%, transparent);
    /* The light-theme soft accent is mixed for a white surface and all but
       vanishes here; strengthen it so the now-playing strip stays readable. */
    --app-accent-soft: color-mix(in srgb, var(--app-accent) 20%, transparent);
    /* A release animates the transform; a finger owning it does not
       (.is-dragging below). The curve is scroll-snap's shape: quick to
       leave, settling at the end. */
    transition: transform 0.28s cubic-bezier(0.32, 0.72, 0, 1);
}

.sheet-body {
    position: absolute;
    inset: 0;
    overflow: hidden;
}

/* Two full-screen panels stacked in a 200%-tall track; shifting the track by
   half its own height (-50%) is exactly one panel. */
.sheet-track {
    height: 200%;
    display: flex;
    flex-direction: column;
    transition: transform 0.28s cubic-bezier(0.32, 0.72, 0, 1);
}

.sheet-panel {
    height: 50%;
    min-height: 0;
    display: flex;
    flex-direction: column;
}

.sheet-strip {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    z-index: 2;
    transition: opacity 0.15s linear;
}

.sheet-strip.strip-hidden {
    pointer-events: none;
}

/* The finger owns the motion: everything snaps to where it is put, frame for
   frame. Also covers the pre-first-paint mount so a reload lands with no
   entrance animation. */
.now-playing-sheet.is-dragging,
.now-playing-sheet.is-dragging .sheet-track,
.now-playing-sheet.is-dragging .sheet-strip {
    transition: none;
}

.now-playing-sheet.is-dragging {
    will-change: transform;
}

/* Nothing waits on transitionend, so reduced motion is pure CSS: the sheet
   simply jumps between detents. */
@media (prefers-reduced-motion: reduce) {
    .now-playing-sheet,
    .now-playing-sheet .sheet-track,
    .now-playing-sheet .sheet-strip {
        transition: none;
    }
}
</style>
