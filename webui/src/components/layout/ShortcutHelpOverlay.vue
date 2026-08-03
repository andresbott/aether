<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useShortcutHelp } from '@/composables/useShortcutHelp'
import { OVERLAY_SHORTCUTS, type Shortcut } from '@/utils/shortcuts'

const help = useShortcutHelp()

interface Badge {
    anchor: string
    /** Every action on this anchor, so a shared control shows both directions. */
    actions: string[]
    keys: string[]
    left: number
    top: number
    side: boolean
}

const badges = ref<Badge[]>([])

// How far from the control the badge floats, and how far it must stay from the
// viewport edges. A floating badge is centred on its control by a CSS transform,
// so `left` is the control's centre and the clamp is approximate by half a badge.
const BADGE_GAP = 12
const EDGE_MARGIN = 8
const HALF_BADGE = 34

// Actions grouped by the control they share, in registry order — so the volume
// rail gets one badge reading "+ −" rather than two badges fighting for the same
// spot, and likewise "← →" on the progress bar.
const byAnchor = (): Map<string, Shortcut[]> => {
    const groups = new Map<string, Shortcut[]>()
    for (const shortcut of OVERLAY_SHORTCUTS) {
        if (!shortcut.anchor) continue
        const group = groups.get(shortcut.anchor)
        if (group) group.push(shortcut)
        else groups.set(shortcut.anchor, [shortcut])
    }
    return groups
}

// Positions are read from the live layout rather than declared per breakpoint:
// whatever the player bar looks like at this width, the badges land on it. The
// three media queries in PlayerControls (and any future move) need no change
// here, and a control the layout has hidden reports a zero rect, so its badge is
// simply skipped and the panel carries the key instead.
const measure = (): void => {
    const found: Badge[] = []
    for (const [anchor, group] of byAnchor()) {
        const el = document.querySelector(`[data-shortcut="${anchor}"]`)
        if (!el) continue
        const rect = el.getBoundingClientRect()
        // display:none and not-yet-laid-out both report 0×0.
        if (rect.width === 0 || rect.height === 0) continue
        // A side badge is placed off the control's right edge and centred on its
        // row; a floating one off the control's centre and lifted above it. The
        // CSS transform per placement completes the job (see the style block).
        const side = group[0]?.place === 'side'
        const left = side
            ? rect.right + BADGE_GAP
            : Math.min(
                  window.innerWidth - EDGE_MARGIN - HALF_BADGE,
                  Math.max(EDGE_MARGIN + HALF_BADGE, rect.left + rect.width / 2)
              )
        found.push({
            anchor,
            actions: group.map((s) => s.action),
            keys: group.map((s) => s.keys[0] ?? ''),
            left,
            top: side ? rect.top + rect.height / 2 : rect.top - BADGE_GAP,
            side
        })
    }
    badges.value = found
}

const anchored = computed(() => new Set(badges.value.flatMap((b) => b.actions)))

// Anything without a badge on screen — no control of its own, or a control this
// width has hidden — is only discoverable from the panel.
const listed = computed<Shortcut[]>(() =>
    OVERLAY_SHORTCUTS.filter((s) => !anchored.value.has(s.action))
)

watch(
    help.open,
    async (open) => {
        if (!open) {
            window.removeEventListener('resize', measure)
            badges.value = []
            return
        }
        // Measure after the backdrop has rendered; the controls themselves are
        // untouched by it, but this keeps the read in one place per open.
        await nextTick()
        measure()
        window.addEventListener('resize', measure)
    },
    { immediate: true }
)

onBeforeUnmount(() => window.removeEventListener('resize', measure))
</script>

<template>
    <!-- Click anywhere dismisses; the badges and panel are pointer-transparent so
         a click through them still lands on the backdrop. -->
    <div v-if="help.open.value" class="shortcut-overlay" @click="help.close()">
        <div class="shortcut-badges" aria-hidden="true">
            <span
                v-for="badge in badges"
                :key="badge.anchor"
                class="shortcut-badge"
                :class="{ 'shortcut-badge--side': badge.side }"
                :style="{ left: badge.left + 'px', top: badge.top + 'px' }"
            >
                <span v-for="key in badge.keys" :key="key" class="shortcut-badge-key">
                    {{ key }}
                </span>
            </span>
        </div>

        <div class="shortcut-panel">
            <h2 class="shortcut-panel-title">Keyboard shortcuts</h2>
            <!-- With every control on screen there is nothing left to list, so the
                 list is dropped rather than rendered empty under the title. -->
            <ul v-if="listed.length" class="shortcut-list">
                <li v-for="entry in listed" :key="entry.action" class="shortcut-row">
                    <span class="shortcut-keys">
                        <kbd v-for="key in entry.keys" :key="key">{{ key }}</kbd>
                    </span>
                    <span class="shortcut-label">{{ entry.label }}</span>
                </li>
            </ul>
            <p class="shortcut-hint">Press <kbd>Esc</kbd> to close</p>
        </div>
    </div>
</template>

<style scoped>
.shortcut-overlay {
    position: fixed;
    inset: 0;
    z-index: 2000;
    background: rgba(0, 0, 0, 0.72);
    cursor: default;
}

/* The badges are placed in viewport coordinates, so the layer must not
   participate in any layout of its own. */
.shortcut-badges {
    position: fixed;
    inset: 0;
    pointer-events: none;
}

/* translate(-50%, -100%) is what actually centres the badge on its control and
   lifts it clear: the measured `left` is the control's centre and `top` is its
   upper edge minus the gap, so the badge's own size never has to be known in JS.
   The measured position also has to survive a resize, which is why the component
   re-measures rather than encoding offsets per breakpoint. */
.shortcut-badge {
    position: absolute;
    transform: translate(-50%, -100%);
    display: flex;
    align-items: center;
    /* The pair sits inside one badge, so a shared rail reads as a single
       affordance driven two ways rather than two competing badges. */
    gap: 0.3rem;
    padding: 0.2rem 0.5rem;
    border-radius: 5px;
    background: var(--app-accent);
    color: #ffffff;
    font-size: 0.75rem;
    font-weight: 600;
    white-space: nowrap;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.45);
}

/* Beside the control rather than above it: the measured `left` already sits past
   its right edge, so there is no X offset to apply, and -50% on Y centres the
   badge on the entry's row. A sidebar nav item has another nav item directly
   above it, so the floating placement would cover that neighbour instead of
   labelling this one. */
.shortcut-badge--side {
    transform: translateY(-50%);
}

/* Top-right corner, out of the way of everything the overlay is explaining: the
   player bar it badges runs along the bottom and the nav entries it badges run
   down the left edge, both of which a centred panel covered. The max-height keeps
   it clear of the player bar even with a full list on a short viewport. */
.shortcut-panel {
    position: fixed;
    top: 1.25rem;
    right: 1.25rem;
    max-width: min(420px, calc(100vw - 2.5rem));
    max-height: calc(100vh - var(--app-player-height) - 2.5rem);
    overflow-y: auto;
    padding: 1.25rem 1.5rem;
    border-radius: 10px;
    background: var(--app-player-bg);
    color: var(--app-player-text);
    box-shadow: 0 12px 40px rgba(0, 0, 0, 0.5);
    pointer-events: none;
}

.shortcut-panel-title {
    margin: 0 0 1rem;
    font-size: 1.05rem;
    font-weight: 600;
}

.shortcut-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.45rem;
}

.shortcut-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
}

.shortcut-keys {
    display: flex;
    gap: 0.25rem;
    flex-shrink: 0;
    min-width: 4.5rem;
}

.shortcut-label {
    color: var(--app-player-dim);
    font-size: 0.85rem;
}

.shortcut-hint {
    margin: 1rem 0 0;
    color: var(--app-player-dim);
    font-size: 0.75rem;
}

kbd {
    display: inline-block;
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    border: 1px solid rgba(255, 255, 255, 0.18);
    background: rgba(255, 255, 255, 0.08);
    font-family: var(--app-player-time-font);
    font-size: 0.75rem;
    line-height: 1.4;
}
</style>
