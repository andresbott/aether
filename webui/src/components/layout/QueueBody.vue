<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted } from 'vue'
import SongDetail from '@/components/library/SongDetail.vue'
import TrackFavoriteButton from '@/components/library/TrackFavoriteButton.vue'
import QueueRow from '@/components/layout/QueueRow.vue'
import TrackEditList from '@/components/layout/TrackEditList.vue'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueDrop } from '@/composables/useQueueDrop'
import { subsonicClient } from '@/lib/api/subsonic'

defineProps<{ variant: 'full' | 'sidebar'; editMode: boolean }>()

const player = usePlayer()

const currentBlockRef = ref<HTMLElement | null>(null)
const editListRef = ref<InstanceType<typeof TrackEditList> | null>(null)

const queueBodyRef = ref<HTMLElement | null>(null)
const {
    indicatorTop: dropIndicatorTop,
    indicatorCount: dropIndicatorCount,
    dragActive: dropActive,
    onDragOver: onQueueDragOver,
    onDragLeave: onQueueDragLeave,
    onDrop: onQueueDrop
} = useQueueDrop({ bodyRef: queueBodyRef, onInsert: () => editListRef.value?.clearSelection() })

const trackCount = computed(() => player.queue.value.length)

// The song is carried by REFERENCE, not spread into the row object: it is a
// reactive member of the queue, and the row's favorite toggle writes `starred`
// straight onto it (the queue is not query-backed, so nothing else would refresh
// the heart). A `{ ...song }` copy would take that write to a throwaway object
// and the heart would snap back on the next recompute.
const historyRows = computed(() =>
    player.queue.value
        .slice(0, player.currentIndex.value)
        .map((song, i) => ({ song, queueIndex: i }))
)

const upcomingRows = computed(() =>
    player.queue.value
        .slice(player.currentIndex.value + 1)
        .map((song, i) => ({ song, queueIndex: player.currentIndex.value + 1 + i }))
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

const removeIndices = (indices: number[]): void => {
    if (indices.length === 0) return
    if (indices.length > 1) player.removeManyFromQueue(indices)
    else player.removeFromQueue(indices[0])
}

const onReorder = (indices: number[], target: number): void => {
    player.moveInQueue(indices, target)
}

// Breathing room above the row when scrolling it to the top edge (the
// scroll-margin-top that applied back when this used scrollIntoView).
const CURRENT_BLOCK_TOP_MARGIN_PX = 16

// Scrolls ONLY the queue's own scroller — never scrollIntoView on the row,
// which also scrolls every scrollable ancestor to reveal it: inside
// NowPlayingSheet the queue panel can be collapsed or behind the player face,
// and revealing the row on mount would scroll the sheet instead of just the
// list, disrupting the detent.
const scrollCurrentIntoView = (block: 'center' | 'nearest'): void => {
    nextTick(() => {
        const row = currentBlockRef.value
        const scroller = queueBodyRef.value
        if (!row || !scroller) return
        const rowTop =
            row.getBoundingClientRect().top -
            scroller.getBoundingClientRect().top +
            scroller.scrollTop
        const rowHeight = row.offsetHeight
        let top: number | null = null
        if (block === 'center') {
            top = rowTop - (scroller.clientHeight - rowHeight) / 2
        } else if (rowTop - CURRENT_BLOCK_TOP_MARGIN_PX < scroller.scrollTop) {
            top = rowTop - CURRENT_BLOCK_TOP_MARGIN_PX
        } else if (rowTop + rowHeight > scroller.scrollTop + scroller.clientHeight) {
            top = rowTop + rowHeight - scroller.clientHeight
        }
        if (top === null) return
        scroller.scrollTo?.({ top: Math.max(0, top), behavior: 'smooth' })
    })
}

// On landing (e.g. navigating back), center the now-playing track so history
// and upcoming are both visible. During playback, scroll only the minimal
// amount to keep it in view as the queue advances.
watch(
    () => player.currentIndex.value,
    () => scrollCurrentIntoView('nearest')
)
onMounted(() => scrollCurrentIntoView('center'))
</script>

<template>
    <div
        v-if="trackCount === 0"
        ref="queueBodyRef"
        class="queue-empty"
        :class="{ 'queue-empty--drop-active': dropActive }"
        @dragover="onQueueDragOver"
        @dragleave="onQueueDragLeave"
        @drop="onQueueDrop"
    >
        <i
            :class="dropActive ? 'pi pi-plus-circle' : 'pi pi-play-circle'"
            style="font-size: 2.5rem"
        ></i>
        <p v-if="dropActive">Drop to add album</p>
        <p v-else>{{ variant === 'full' ? 'Nothing is playing' : 'Queue is empty' }}</p>
    </div>

    <div
        v-else
        ref="queueBodyRef"
        class="queue-body"
        :class="`queue-body--${variant}`"
        @dragover="onQueueDragOver"
        @dragleave="onQueueDragLeave"
        @drop="onQueueDrop"
    >
        <div
            v-if="dropIndicatorTop !== null"
            class="queue-drop-indicator"
            :style="{ top: dropIndicatorTop + 'px' }"
        >
            <span class="queue-drop-indicator__badge">+{{ dropIndicatorCount }}</span>
        </div>
        <!-- Edit mode: one flat reorderable list of every track, the
             now-playing one rendered as a row with a play toggle. -->
        <TrackEditList
            v-if="editMode"
            ref="editListRef"
            :songs="player.queue.value"
            :current-index="player.currentIndex.value"
            delete-label="Remove from queue"
            group="queue"
            :artist-column="variant === 'full'"
            @reorder="onReorder"
            @delete="removeIndices"
        />

        <!-- View mode: faded history, the now-playing card/strip, upcoming. -->
        <template v-else>
            <div v-if="historyRows.length" class="queue-history">
                <QueueRow
                    v-for="row in historyRows"
                    :key="row.song.id + ':' + row.queueIndex"
                    :song="row.song"
                    :queue-index="row.queueIndex"
                    :artist-column="variant === 'full'"
                    @play="onPlayRow(row.queueIndex)"
                />
            </div>

            <div
                ref="currentBlockRef"
                class="current-block"
                :data-queue-index="player.currentIndex.value"
            >
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
                        <!-- The now-playing track is a track like any other in the
                             list, so it gets the same heart — but on its own line
                             under the text rather than in a star column: the strip
                             is a stacked block, not a table row, and the sidebar is
                             too narrow to spend a column on it. The full variant
                             needs none, since SongDetail's card carries its own. -->
                        <span class="strip-star"><TrackFavoriteButton :song="currentSong" /></span>
                    </div>
                </div>
            </div>

            <div v-if="upcomingRows.length" class="queue-upcoming">
                <QueueRow
                    v-for="row in upcomingRows"
                    :key="row.song.id + ':' + row.queueIndex"
                    :song="row.song"
                    :queue-index="row.queueIndex"
                    :artist-column="variant === 'full'"
                    @play="onPlayRow(row.queueIndex)"
                />
            </div>
        </template>
    </div>
</template>

<style scoped>
/* The body fills its frame in both hosts: `height:100%` sizes it inside the
   ContentScaffold body (full variant); `flex:1` sizes it as a direct child of
   the sidebar's flex column, where flex-basis overrides the height. */
.queue-empty,
.queue-body {
    flex: 1;
    min-height: 0;
    height: 100%;
    box-sizing: border-box;
}

.queue-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.75rem;
    color: var(--app-text-secondary);
    /* Reserve the border so the drop-active state adds no layout shift. */
    border: 2px dashed transparent;
    border-radius: 8px;
    transition:
        border-color 0.15s,
        background-color 0.15s,
        color 0.15s;
}

.queue-empty--drop-active {
    border-color: var(--app-accent);
    background-color: var(--app-accent-soft);
    color: var(--app-accent);
}

.queue-body {
    overflow-y: auto;
    overflow-x: hidden;
    position: relative;
    scrollbar-gutter: stable;
}

/* Recipe B: the full (Now Playing) scroll area reserves the uniform rail
   clearance on the right; blocks center on the shared content column. The
   sidebar variant is side-panel chrome and stays untouched. */
.queue-body--full {
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
}

.queue-body--full .queue-history,
.queue-body--full .current-block,
.queue-body--full .queue-upcoming,
.queue-body--full :deep(.queue-edit-list) {
    width: 100%;
    max-width: calc(var(--app-content-max-width) + 2 * var(--app-content-gutter));
    margin-left: auto;
    margin-right: auto;
    padding-left: var(--app-content-gutter);
    padding-right: var(--app-content-gutter);
    box-sizing: border-box;
}

.queue-drop-indicator {
    position: absolute;
    left: 0;
    right: 0;
    height: 2px;
    background: var(--app-accent);
    pointer-events: none;
    z-index: 5;
}

.queue-drop-indicator__badge {
    position: absolute;
    left: 0.5rem;
    top: -0.7rem;
    padding: 0 6px;
    height: 1.1rem;
    display: inline-flex;
    align-items: center;
    background: var(--app-accent);
    color: #ffffff;
    border-radius: 0.55rem;
    font-size: 0.7rem;
    font-weight: 700;
}

/* Already-played tracks are faded. */
.queue-history {
    opacity: 0.45;
}

.current-block {
    /* Top clearance lives in scrollCurrentIntoView (CURRENT_BLOCK_TOP_MARGIN_PX):
       manual scrolling doesn't honor scroll-margin. */
    padding: 0.5rem 0;
}

.queue-body--full .current-block {
    /* Longhand so it doesn't reset the horizontal gutter set in the rule above. */
    padding-top: 1rem;
    padding-bottom: 1rem;
}

.now-playing-strip {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 0.75rem 0.5rem;
    background-color: var(--app-accent-soft);
    box-shadow: inset 3px 0 0 var(--app-accent);
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
    transition:
        color 0.15s,
        transform 0.15s;
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
    color: var(--app-accent);
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

/* Own line below the title/artist/album stack, left-aligned with them. Always
   visible — there is no row to hover here, and the strip is a single persistent
   block rather than one of a list, so a hidden-until-hover heart would just be
   hard to find. */
.strip-star {
    display: flex;
    align-items: center;
    margin-top: 0.35rem;
}

.strip-star :deep(.row-star) {
    opacity: 1;
    font-size: 1.1rem;
}
</style>
