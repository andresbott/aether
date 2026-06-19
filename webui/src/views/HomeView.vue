<script setup lang="ts">
import { computed, ref, watch, nextTick, onMounted } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Button from 'primevue/button'
import Menu from 'primevue/menu'
import type { MenuItem } from 'primevue/menuitem'
import SongDetail from '@/components/library/SongDetail.vue'
import SavePlaylistDialog from '@/components/layout/SavePlaylistDialog.vue'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueActions } from '@/composables/useQueueActions'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Song } from '@/types/subsonic'

const player = usePlayer()
const { showSaveDialog, playlistName, openSaveDialog, handleSave, isSaving, clearQueue } = useQueueActions()

const currentCardRef = ref<HTMLElement | null>(null)
const headerRef = ref<HTMLElement | null>(null)

const trackCount = computed(() => player.queue.value.length)

const totalDuration = computed(() => {
    const total = player.queue.value.reduce((sum, song) => sum + (song.duration || 0), 0)
    if (!total) return ''
    const hours = Math.floor(total / 3600)
    const mins = Math.floor((total % 3600) / 60)
    return hours > 0 ? `${hours} hr ${mins} min` : `${mins} min`
})

const actionsMenu = ref()
const toggleActionsMenu = (event: Event): void => {
    actionsMenu.value.toggle(event)
}

const actionsMenuItems = computed<MenuItem[]>(() => [
    {
        label: 'Clear Queue',
        icon: 'pi pi-trash',
        command: () => clearQueue()
    },
    {
        label: 'Save as Playlist',
        icon: 'pi pi-save',
        command: () => openSaveDialog()
    }
])

const historyRows = computed(() =>
    player.queue.value
        .slice(0, player.currentIndex.value)
        .map((song, i) => ({ ...song, _queueIndex: i }))
)

const upcomingRows = computed(() =>
    player.queue.value
        .slice(player.currentIndex.value + 1)
        .map((song, i) => ({ ...song, _queueIndex: player.currentIndex.value + 1 + i }))
)

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const getCoverUrl = (coverArt?: string): string | null => {
    if (!coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(coverArt, 40)
}

const onRowClick = (event: { data: Song & { _queueIndex: number } }) => {
    player.playQueueItem(event.data._queueIndex)
}

const scrollCurrentIntoView = () => {
    nextTick(() => {
        const el = currentCardRef.value
        if (!el) return
        // Offset the scroll target by the sticky header's rendered height plus
        // .main-content's 1rem top padding, so the current song lands just below
        // the header instead of behind it.
        const headerHeight = headerRef.value?.offsetHeight ?? 0
        el.style.scrollMarginTop = `${headerHeight + 24}px`
        el.scrollIntoView({ behavior: 'smooth', block: 'start' })
    })
}

watch(() => player.currentIndex.value, scrollCurrentIntoView)
onMounted(scrollCurrentIntoView)
</script>

<template>
    <div class="now-playing-view">
        <div v-if="player.queue.value.length === 0" class="empty-state">
            <i class="pi pi-play-circle" style="font-size: 4rem"></i>
            <h2>Nothing is playing</h2>
            <p>Browse your library and start playing music</p>
        </div>

        <template v-else>
            <div ref="headerRef" class="now-playing-header">
                <div class="np-title">
                    <h2>Now Playing</h2>
                    <span class="np-meta">
                        {{ trackCount }} {{ trackCount === 1 ? 'track' : 'tracks' }}
                        <template v-if="totalDuration"> &bull; {{ totalDuration }}</template>
                    </span>
                </div>
                <Button
                    icon="pi pi-ellipsis-v"
                    text
                    rounded
                    severity="secondary"
                    @click="toggleActionsMenu"
                    v-tooltip.bottom="'Queue options'"
                />
            </div>

            <div class="now-playing-panel">
                <DataTable
                    v-if="historyRows.length > 0"
                    :value="historyRows"
                    class="queue-table history-table"
                    @row-click="onRowClick"
                >
                    <Column style="width: 40px">
                        <template #body="{ data }">
                            <div class="cover-thumb">
                                <img
                                    v-if="getCoverUrl(data.coverArt)"
                                    :src="getCoverUrl(data.coverArt)!"
                                    alt=""
                                />
                                <i v-else class="pi pi-music"></i>
                            </div>
                        </template>
                    </Column>
                    <Column style="min-width: 0">
                        <template #body="{ data }">
                            <div class="track-info">
                                <span class="track-title">{{ data.title }}</span>
                                <span class="track-artist">{{ data.artist || 'Unknown' }}</span>
                            </div>
                        </template>
                    </Column>
                    <Column style="width: 64px; text-align: right">
                        <template #body="{ data }">
                            <span class="track-duration">{{ formatDuration(data.duration) }}</span>
                        </template>
                    </Column>
                </DataTable>

                <div ref="currentCardRef" class="current-song">
                    <SongDetail
                        v-if="player.currentTrack.value"
                        :song="player.currentTrack.value"
                        card
                        @play="player.togglePlayPause"
                    />
                </div>

                <DataTable
                    v-if="upcomingRows.length > 0"
                    :value="upcomingRows"
                    class="queue-table upcoming-table"
                    @row-click="onRowClick"
                >
                    <Column style="width: 40px">
                        <template #body="{ data }">
                            <div class="cover-thumb">
                                <img
                                    v-if="getCoverUrl(data.coverArt)"
                                    :src="getCoverUrl(data.coverArt)!"
                                    alt=""
                                />
                                <i v-else class="pi pi-music"></i>
                            </div>
                        </template>
                    </Column>
                    <Column style="min-width: 0">
                        <template #body="{ data }">
                            <div class="track-info">
                                <span class="track-title">{{ data.title }}</span>
                                <span class="track-artist">{{ data.artist || 'Unknown' }}</span>
                            </div>
                        </template>
                    </Column>
                    <Column style="width: 64px; text-align: right">
                        <template #body="{ data }">
                            <span class="track-duration">{{ formatDuration(data.duration) }}</span>
                        </template>
                    </Column>
                </DataTable>
            </div>
        </template>

        <Menu ref="actionsMenu" :model="actionsMenuItems" :popup="true" />

        <SavePlaylistDialog
            v-model:visible="showSaveDialog"
            v-model:name="playlistName"
            :saving="isSaving"
            @save="handleSave"
        />
    </div>
</template>

<style scoped>
.now-playing-view {
    max-width: 1100px;
    margin: 0 auto;
    display: flex;
    flex-direction: column;
    gap: 2rem;
}

.now-playing-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
    position: sticky;
    top: 0;
    z-index: 5;
    background-color: var(--app-background);
    padding-bottom: 0.75rem;
    border-bottom: 1px solid var(--app-border);
}

/* .main-content has 1rem top padding, so the sticky header pins just below it,
   leaving a band where scrolling tracks would show. This strip moves with the
   pinned header and paints over that band. */
.now-playing-header::before {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    bottom: 100%;
    height: 1rem;
    background-color: var(--app-background);
}

.np-title {
    display: flex;
    align-items: baseline;
    gap: 0.75rem;
    min-width: 0;
}

.np-title h2 {
    margin: 0;
    font-size: 1.25rem;
    font-weight: 600;
    color: var(--app-text-primary);
}

.np-meta {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
}

.empty-state {
    min-height: 60vh;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.75rem;
    color: var(--app-text-secondary);
    text-align: center;
}

.empty-state h2 {
    margin: 0;
    font-size: 1.5rem;
    color: var(--app-text-primary);
}

.empty-state p {
    margin: 0;
}

.now-playing-panel {
    background-color: var(--app-surface);
    border: 1px solid var(--app-border);
    border-radius: 12px;
    overflow: hidden;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

/* SongDetail's own card chrome is dropped so it merges into the single panel. */
.now-playing-panel :deep(.song-detail--card) {
    max-width: none;
    background: transparent;
    border: none;
    border-radius: 0;
    box-shadow: none;
}

.current-song {
    scroll-margin-top: 1rem;
}

/* Already-played tracks: faded, above the current song inside the panel. */
.history-table {
    opacity: 0.45;
    border-bottom: 1px solid var(--app-border);
    padding: 0.5rem;
}

.upcoming-table {
    border-top: 1px solid var(--app-border);
    padding: 0.5rem;
}

.queue-table :deep(.p-datatable-thead) {
    display: none;
}

.queue-table :deep(.p-datatable-tbody > tr) {
    cursor: pointer;
    transition: background-color 0.15s;
}

.queue-table :deep(.p-datatable-tbody > tr > td) {
    padding: 0.4rem 0.5rem;
    border: none;
}

.queue-table :deep(.p-datatable-tbody > tr:hover) {
    background-color: var(--app-background);
}

.cover-thumb {
    width: 36px;
    height: 36px;
    border-radius: 4px;
    overflow: hidden;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
}

.cover-thumb img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.cover-thumb i {
    font-size: 0.8rem;
    color: rgba(255, 255, 255, 0.85);
}

.track-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
}

.track-title {
    font-size: 0.9rem;
    font-weight: 500;
    color: var(--app-text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.track-artist {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.track-duration {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}
</style>
