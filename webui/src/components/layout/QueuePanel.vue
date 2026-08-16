<script setup lang="ts">
import { ref, watch } from 'vue'
import Button from 'primevue/button'
import Popover from 'primevue/popover'
import QueueBody from '@/components/layout/QueueBody.vue'
import QueueHeaderActions from '@/components/layout/QueueHeaderActions.vue'
import SavePlaylistDialog from '@/components/layout/SavePlaylistDialog.vue'
import { useNowPlayingSheet } from '@/composables/useNowPlayingSheet'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueActions } from '@/composables/useQueueActions'
import { useQueueEdit } from '@/composables/useQueueEdit'
import { useQueueSummary } from '@/composables/useQueueSummary'

// The Now Playing sheet's queue panel (NowPlayingSheet.vue): the heading with
// the queue's own actions, and the shared QueueBody rows. The heading rides
// in with the panel — no fixed bar over the player face, nothing to fade.
// Gesture-free on purpose: the sheet owns the drags and reaches the heading /
// list by class (queue-heading / play-queue-list) through event delegation.
const player = usePlayer()
const { showSaveDialog, playlistName, openSaveDialog, handleSave, isSaving, clearQueue } =
    useQueueActions()
const { editMode, toggleEditMode, exitEditMode } = useQueueEdit()
const { trackCount, summary } = useQueueSummary()
const { detent } = useNowPlayingSheet()

// Edit mode is queue-panel UI: leaving the queue detent — by swipe, hint
// button or back button, all of which move the sheet — ends the editing
// session, so returning to the queue never lands on a stale selection.
watch(detent, (d) => {
    if (d !== 'queue') {
        exitEditMode()
        overflowRef.value?.hide()
    }
})

// The queue-management trio (edit/save/clear) behind the heading's ⋮ — three
// more bare glyphs next to shuffle/repeat would not read as a toolbar on a
// phone. Labeled inside the popover, since tooltips don't exist on touch.
const overflowRef = ref<InstanceType<typeof Popover> | null>(null)
const toggleOverflow = (event: Event) => overflowRef.value?.toggle(event)
</script>

<template>
    <div class="play-queue">
        <header class="queue-heading">
            <div class="queue-heading-text">
                <h2>Queue</h2>
                <span class="queue-heading-summary">{{ summary }}</span>
            </div>
            <!-- Shuffle and repeat are QUEUE behaviour, so they sit with the
                 queue heading rather than in the face's transport row. -->
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
        <div class="play-queue-list">
            <QueueBody variant="sidebar" :edit-mode="editMode" />
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
.play-queue {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
}

/* The queue's own heading, inside its panel: it slides in with the queue.
   Reserves the TOP inset — it is the topmost surface on this panel, and in a
   standalone launch the status bar overlaps it. */
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

/* h2, not h1: the page heading on this surface is the track on the player
   face, and the queue is a section of it. Sized like the scaffold's phone
   title. */
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

/* Same "pressed" convention as the queue pencil (QueueHeaderActions): a
   toggle that is on carries the soft accent fill. */
.queue-action-shuffle.is-active,
.queue-action-repeat.is-active {
    background: var(--app-accent-soft);
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
    /* Bottom-most surface while the queue is up (no mini strip on screen), so
       the list reserves the home-indicator inset itself. */
    padding-bottom: env(safe-area-inset-bottom);
}

/* The list (and the QueueBody scroller inside it) CONTAINS its overscroll: a
   hard fling to the list's top must not chain into the page, and the
   deliberate queue → face pull is the sheet's own gesture (which
   preventDefaults once claimed), not native chaining. */
.play-queue-list,
.play-queue-list :deep(.queue-body) {
    overscroll-behavior-y: contain;
}
</style>
