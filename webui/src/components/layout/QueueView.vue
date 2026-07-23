<script setup lang="ts">
import { computed } from 'vue'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import SavePlaylistDialog from '@/components/layout/SavePlaylistDialog.vue'
import QueueBody from '@/components/layout/QueueBody.vue'
import QueueHeaderActions from '@/components/layout/QueueHeaderActions.vue'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueActions } from '@/composables/useQueueActions'
import { useQueueEdit } from '@/composables/useQueueEdit'

const props = defineProps<{ variant: 'full' | 'sidebar' }>()

const player = usePlayer()
const { showSaveDialog, playlistName, openSaveDialog, handleSave, isSaving, clearQueue } =
    useQueueActions()
const { editMode, toggleEditMode } = useQueueEdit()

const title = computed(() => (props.variant === 'full' ? 'Now Playing' : 'Queue'))
const trackCount = computed(() => player.queue.value.length)

const totalDuration = computed(() => {
    const total = player.queue.value.reduce((sum, s) => sum + (s.duration || 0), 0)
    if (!total) return ''
    const hours = Math.floor(total / 3600)
    const mins = Math.floor((total % 3600) / 60)
    return hours > 0 ? `${hours} hr ${mins} min` : `${mins} min`
})

// Pre-built as one string so the header has no stray whitespace between the
// count and the unit word (keeps `.text()` assertions reliable in tests).
// Empty at zero so ContentScaffold omits the summary element entirely.
const summary = computed(() => {
    if (trackCount.value === 0) return ''
    const tracks = `${trackCount.value} ${trackCount.value === 1 ? 'track' : 'tracks'}`
    return totalDuration.value ? `${tracks} • ${totalDuration.value}` : tracks
})
</script>

<template>
    <!-- Full variant (Now Playing, route `/`): the canonical ContentScaffold
         header per docs/architecture/main-content-view-layout.md. -->
    <ContentScaffold v-if="variant === 'full'" class="queue-view" :title="title" :summary="summary">
        <template #actions>
            <QueueHeaderActions
                :edit-mode="editMode"
                :disabled="trackCount === 0"
                @toggle-edit="toggleEditMode"
                @save="openSaveDialog"
                @clear="clearQueue"
            />
        </template>

        <QueueBody variant="full" :edit-mode="editMode" />

        <SavePlaylistDialog
            v-model:visible="showSaveDialog"
            v-model:name="playlistName"
            :saving="isSaving"
            @save="handleSave"
        />
    </ContentScaffold>

    <!-- Sidebar variant: side-panel chrome with a compact header — not
         governed by the main-content-view layout guidance. -->
    <div v-else class="queue-view queue-view--sidebar">
        <div class="queue-view-header">
            <slot name="header-start" />
            <div class="header-title">
                <h3>{{ title }}</h3>
                <span v-if="trackCount > 0" class="queue-info">{{ summary }}</span>
            </div>
            <div class="header-actions">
                <QueueHeaderActions
                    :edit-mode="editMode"
                    :disabled="trackCount === 0"
                    size="small"
                    @toggle-edit="toggleEditMode"
                    @save="openSaveDialog"
                    @clear="clearQueue"
                />
            </div>
        </div>

        <QueueBody variant="sidebar" :edit-mode="editMode" />

        <SavePlaylistDialog
            v-model:visible="showSaveDialog"
            v-model:name="playlistName"
            :saving="isSaving"
            @save="handleSave"
        />
    </div>
</template>

<style scoped>
.queue-view {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    width: 100%;
}

/* The scaffold body is full-width; QueueBody centers its content on the shared
   content column and keeps the queue's scrollbar flush right. */
.queue-view :deep(.content-scaffold-body) {
    display: flex;
    flex-direction: column;
    min-height: 0;
}

/* --- Sidebar compact header (side-panel chrome, not ContentScaffold) --- */

.queue-view-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.75rem;
    min-height: 3rem;
    box-sizing: border-box;
    flex-shrink: 0;
}

.header-title {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: 0.5rem;
}

.header-title h3 {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
}

/* The count/duration reads as a cyan pill badge (mockup's .queue-count). The
   full Now Playing header keeps the plain secondary summary from the shared
   ContentScaffold header. */
.queue-info {
    font-size: 0.72rem;
    font-weight: 700;
    color: var(--app-accent);
    background: var(--app-accent-soft);
    padding: 2px 9px;
    border-radius: 99px;
}

.header-actions {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    flex-shrink: 0;
}
</style>
