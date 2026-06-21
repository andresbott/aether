<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import Sortable from 'sortablejs'
import Button from 'primevue/button'
import SongDetail from '@/components/library/SongDetail.vue'
import SavePlaylistDialog from '@/components/layout/SavePlaylistDialog.vue'
import QueueRow from '@/components/layout/QueueRow.vue'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueActions } from '@/composables/useQueueActions'
import { useQueueEdit } from '@/composables/useQueueEdit'
import { computeDropTarget } from '@/utils/queueReorder'
import { subsonicClient } from '@/lib/api/subsonic'

const props = defineProps<{ variant: 'full' | 'sidebar' }>()

const player = usePlayer()
const { showSaveDialog, playlistName, openSaveDialog, handleSave, isSaving, clearQueue } =
    useQueueActions()
const {
    editMode,
    toggleEditMode,
    isSelected,
    onRowClick: onEditRowClick,
    toggleCheckbox,
    selectionForDrag,
    clearSelection
} = useQueueEdit()

const currentBlockRef = ref<HTMLElement | null>(null)
const historyListRef = ref<HTMLElement | null>(null)
const upcomingListRef = ref<HTMLElement | null>(null)
let sortables: Sortable[] = []

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
const summary = computed(() => {
    const tracks = `${trackCount.value} ${trackCount.value === 1 ? 'track' : 'tracks'}`
    return totalDuration.value ? `${tracks} • ${totalDuration.value}` : tracks
})

const historyRows = computed(() =>
    player.queue.value
        .slice(0, player.currentIndex.value)
        .map((song, i) => ({ ...song, queueIndex: i }))
)

const upcomingRows = computed(() =>
    player.queue.value
        .slice(player.currentIndex.value + 1)
        .map((song, i) => ({ ...song, queueIndex: player.currentIndex.value + 1 + i }))
)

const currentSong = computed(() => player.queue.value[player.currentIndex.value] ?? null)
const currentPosition = computed(() => player.currentIndex.value + 1)

const stripCoverUrl = computed<string | null>(() => {
    const art = currentSong.value?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 160)
})

const onPlayRow = (index: number): void => {
    player.playQueueItem(index)
}

const onDeleteRow = (index: number): void => {
    player.removeFromQueue(index)
    clearSelection()
}

const handleSortEnd = (evt: Sortable.SortableEvent): void => {
    const item = evt.item as HTMLElement
    const draggedIndex = Number(item.dataset.queueIndex)
    // The row that ends up right after the dropped item (in its destination
    // list) is the anchor to insert before; none → append at the end.
    const toList = evt.to as HTMLElement
    const after = toList.children[(evt.newIndex ?? 0) + 1] as HTMLElement | undefined
    const anchorRaw = after?.dataset.queueIndex
    const anchorIndex = anchorRaw !== undefined ? Number(anchorRaw) : undefined
    const isHistory = toList === historyListRef.value
    const targetIndex = computeDropTarget(
        anchorIndex,
        isHistory,
        player.currentIndex.value,
        player.queue.value.length
    )

    // Revert SortableJS's DOM mutation so Vue can re-render cleanly from state.
    const fromList = evt.from as HTMLElement
    const reference = fromList.children[evt.oldIndex ?? 0] ?? null
    fromList.insertBefore(item, reference)

    if (Number.isNaN(draggedIndex)) return
    player.moveInQueue(selectionForDrag(draggedIndex), targetIndex)
    clearSelection()
}

const destroySortables = (): void => {
    sortables.forEach((s) => s.destroy())
    sortables = []
}

const createSortables = (): void => {
    destroySortables()
    const options: Sortable.Options = {
        group: 'queue',
        handle: '.drag-handle',
        animation: 150,
        onEnd: handleSortEnd
    }
    if (historyListRef.value) sortables.push(Sortable.create(historyListRef.value, options))
    if (upcomingListRef.value) sortables.push(Sortable.create(upcomingListRef.value, options))
}

watch(editMode, (on) => {
    if (on) nextTick(createSortables)
    else destroySortables()
})

onUnmounted(destroySortables)

const scrollCurrentIntoView = (): void => {
    nextTick(() => {
        currentBlockRef.value?.scrollIntoView?.({ behavior: 'smooth', block: 'nearest' })
    })
}

watch(() => player.currentIndex.value, scrollCurrentIntoView)
onMounted(scrollCurrentIntoView)
</script>

<template>
    <div class="queue-view" :class="`queue-view--${variant}`">
        <div class="queue-view-header">
            <slot name="header-start" />
            <div class="header-title">
                <h3>{{ title }}</h3>
                <span v-if="trackCount > 0" class="queue-info">{{ summary }}</span>
            </div>
            <div class="header-actions">
                <Button
                    class="queue-action-edit"
                    icon="pi pi-pencil"
                    text
                    rounded
                    size="small"
                    :severity="editMode ? 'primary' : 'secondary'"
                    :class="{ 'is-active': editMode }"
                    :aria-pressed="editMode"
                    :disabled="trackCount === 0"
                    v-tooltip.bottom="editMode ? 'Done editing' : 'Edit queue'"
                    @click="toggleEditMode"
                />
                <Button
                    class="queue-action-save"
                    icon="pi pi-save"
                    text
                    rounded
                    size="small"
                    severity="secondary"
                    :disabled="trackCount === 0"
                    v-tooltip.bottom="'Save as playlist'"
                    @click="openSaveDialog"
                />
                <Button
                    class="queue-action-clear"
                    icon="pi pi-trash"
                    text
                    rounded
                    size="small"
                    severity="secondary"
                    :disabled="trackCount === 0"
                    v-tooltip.bottom="'Clear queue'"
                    @click="clearQueue"
                />
            </div>
        </div>

        <div v-if="trackCount === 0" class="queue-empty">
            <i class="pi pi-play-circle" style="font-size: 2.5rem"></i>
            <p>{{ variant === 'full' ? 'Nothing is playing' : 'Queue is empty' }}</p>
        </div>

        <div v-else class="queue-body">
            <div
                v-if="historyRows.length || editMode"
                ref="historyListRef"
                class="queue-history"
                :class="{ 'queue-list--drop-empty': editMode && historyRows.length === 0 }"
            >
                <QueueRow
                    v-for="row in historyRows"
                    :key="row.id + ':' + row.queueIndex"
                    :song="row"
                    :queue-index="row.queueIndex"
                    :editing="editMode"
                    :selected="isSelected(row.queueIndex)"
                    @play="onPlayRow(row.queueIndex)"
                    @select="(p) => onEditRowClick(row.queueIndex, p.additive)"
                    @toggle-check="toggleCheckbox(row.queueIndex)"
                    @delete="onDeleteRow(row.queueIndex)"
                />
            </div>

            <div ref="currentBlockRef" class="current-block">
                <SongDetail v-if="variant === 'full' && currentSong" :song="currentSong" card />
                <div v-else-if="currentSong" class="now-playing-strip">
                    <button
                        type="button"
                        class="strip-index"
                        :aria-label="player.isPlaying.value ? 'Pause' : 'Play'"
                        @click="player.togglePlayPause"
                    >
                        <span class="strip-number-value">{{ currentPosition }}</span>
                        <i
                            class="strip-toggle-icon"
                            :class="player.isPlaying.value ? 'pi pi-pause' : 'pi pi-play'"
                        ></i>
                    </button>
                    <div class="strip-cover">
                        <img v-if="stripCoverUrl" :src="stripCoverUrl" alt="" />
                        <i v-else class="pi pi-music"></i>
                    </div>
                    <div class="strip-info">
                        <div class="strip-title">{{ currentSong.title }}</div>
                        <div class="strip-artist">{{ currentSong.artist || 'Unknown' }}</div>
                        <div v-if="currentSong.album" class="strip-album">
                            {{ currentSong.album }}
                        </div>
                    </div>
                </div>
            </div>

            <div
                v-if="upcomingRows.length || editMode"
                ref="upcomingListRef"
                class="queue-upcoming"
                :class="{ 'queue-list--drop-empty': editMode && upcomingRows.length === 0 }"
            >
                <QueueRow
                    v-for="row in upcomingRows"
                    :key="row.id + ':' + row.queueIndex"
                    :song="row"
                    :queue-index="row.queueIndex"
                    :editing="editMode"
                    :selected="isSelected(row.queueIndex)"
                    @play="onPlayRow(row.queueIndex)"
                    @select="(p) => onEditRowClick(row.queueIndex, p.additive)"
                    @toggle-check="toggleCheckbox(row.queueIndex)"
                    @delete="onDeleteRow(row.queueIndex)"
                />
            </div>
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
.queue-view {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
}

.queue-view--full {
    max-width: 1100px;
    margin: 0 auto;
    width: 100%;
}

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

.queue-info {
    font-size: 0.8rem;
    font-weight: 400;
    color: var(--app-text-secondary);
}

.header-actions {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    flex-shrink: 0;
}

.queue-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    flex: 1;
    gap: 0.75rem;
    color: var(--app-text-secondary);
}

.queue-body {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overflow-x: hidden;
}

/* Already-played tracks are faded. */
.queue-history {
    opacity: 0.45;
}

/* In edit mode, an empty history/upcoming list still renders (with no visible
   chrome) as a droppable area so a track can be moved before the first or after
   the last when the playing track sits at that edge and the list would
   otherwise be absent. Just a drop target — no placeholder. */
.queue-list--drop-empty {
    min-height: 2.5rem;
}

.current-block {
    scroll-margin-top: 1rem;
    padding: 0.5rem 0;
}

.queue-view--full .current-block {
    padding: 1rem 0;
}

.now-playing-strip {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.75rem 0.5rem;
    border-top: 1px solid var(--app-border);
    border-bottom: 1px solid var(--app-border);
}

/* Left column aligns with QueueRow's .row-index so the current song's play
   affordance lines up with the other tracks: the position number by default,
   swapped for a play/pause toggle on hover. */
.now-playing-strip .strip-index {
    width: 1.75rem;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    border: none;
    background: none;
    cursor: pointer;
    color: inherit;
}

.strip-number-value {
    font-size: 0.85rem;
    font-weight: 500;
    color: var(--app-text-secondary);
}

.strip-toggle-icon {
    display: none;
    font-size: 1rem;
    color: var(--app-text-primary);
    transition: color 0.15s, transform 0.15s;
}

.now-playing-strip:hover .strip-number-value {
    display: none;
}

.now-playing-strip:hover .strip-toggle-icon {
    display: inline;
}

.strip-index:hover .strip-toggle-icon {
    color: var(--app-accent);
    transform: scale(1.1);
}

.strip-cover {
    width: 140px;
    height: 140px;
    flex-shrink: 0;
    border-radius: 8px;
    overflow: hidden;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    display: flex;
    align-items: center;
    justify-content: center;
}

.strip-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.strip-cover i {
    font-size: 2rem;
    color: rgba(255, 255, 255, 0.85);
}

.strip-info {
    flex: 1;
    min-width: 0;
}

.strip-title {
    font-size: 1.05rem;
    font-weight: 600;
    color: var(--app-text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.strip-artist {
    font-size: 0.9rem;
    color: var(--app-text-secondary);
}

.strip-album {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
}
</style>
