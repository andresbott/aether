<script setup lang="ts">
import { computed, ref, watch, nextTick, onMounted } from 'vue'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import SongDetail from '@/components/library/SongDetail.vue'
import { usePlayer } from '@/composables/usePlayer'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Song } from '@/types/subsonic'

const player = usePlayer()

const currentCardRef = ref<HTMLElement | null>(null)

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
        currentCardRef.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
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
            <div v-if="historyRows.length > 0" class="queue-section history-section">
                <p class="section-label">History</p>
                <DataTable
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
            </div>

            <div ref="currentCardRef" class="current-card-wrapper">
                <SongDetail
                    v-if="player.currentTrack.value"
                    :song="player.currentTrack.value"
                    card
                    @play="player.togglePlayPause"
                />
            </div>

            <div v-if="upcomingRows.length > 0" class="queue-section upcoming-section">
                <p class="section-label">Up next</p>
                <DataTable
                    :value="upcomingRows"
                    class="queue-table"
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

.queue-section {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.section-label {
    margin: 0;
    font-size: 0.75rem;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--app-text-secondary);
}

.history-section {
    opacity: 0.45;
}

.current-card-wrapper {
    scroll-margin-top: 1rem;
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
