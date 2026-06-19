<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Menu from 'primevue/menu'
import type { MenuItem } from 'primevue/menuitem'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueSidebar } from '@/composables/useQueueSidebar'
import { useQueueActions } from '@/composables/useQueueActions'
import SavePlaylistDialog from '@/components/layout/SavePlaylistDialog.vue'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Song } from '@/types/subsonic'

const router = useRouter()
const player = usePlayer()
const { sidebarCollapsed, sidebarWidth, toggleSidebar, setSidebarWidth } = useQueueSidebar()
const { showSaveDialog, playlistName, openSaveDialog, handleSave, isSaving, clearQueue } = useQueueActions()

const headerMenu = ref()
const hoveredTrackIndex = ref<number | null>(null)
const isResizing = ref(false)

const queueWithIndex = computed(() =>
    player.queue.value.map((song, index) => ({
        ...song,
        index,
        isCurrent: index === player.currentIndex.value
    }))
)

const trackCount = computed(() => player.queue.value.length)

const totalDuration = computed(() => {
    const total = player.queue.value.reduce((sum, song) => sum + (song.duration || 0), 0)
    if (!total) return '0 min'
    const hours = Math.floor(total / 3600)
    const mins = Math.floor((total % 3600) / 60)
    return hours > 0 ? `${hours} hr ${mins} min` : `${mins} min`
})

const getCoverUrl = (coverArt?: string): string | null => {
    if (!coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(coverArt, 48)
}

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const onRowClick = (event: { data: Song & { index: number } }): void => {
    router.push({ name: 'song', params: { index: event.data.index.toString() } })
}

const handlePlayPauseClick = (event: Event, index: number): void => {
    event.stopPropagation()
    if (index === player.currentIndex.value) {
        player.togglePlayPause()
    } else {
        player.playQueueItem(index)
    }
}

const toggleHeaderMenu = (event: Event): void => {
    headerMenu.value.toggle(event)
}

const headerMenuItems = computed<MenuItem[]>(() => [
    {
        label: 'Clear Queue',
        icon: 'pi pi-trash',
        command: () => clearQueue()
    },
    {
        label: 'Save as Playlist',
        icon: 'pi pi-save',
        command: () => openSaveDialog()
    },
    {
        label: 'Shuffle Queue',
        icon: 'pi pi-random',
        command: () => {
            console.log('Shuffle queue')
        }
    }
])

const startResize = (event: MouseEvent) => {
    event.preventDefault()
    isResizing.value = true
    const startX = event.clientX
    const startWidth = sidebarWidth.value

    const onMouseMove = (e: MouseEvent) => {
        const delta = startX - e.clientX
        setSidebarWidth(startWidth + delta)
    }

    const onMouseUp = () => {
        isResizing.value = false
        document.removeEventListener('mousemove', onMouseMove)
        document.removeEventListener('mouseup', onMouseUp)
        document.body.style.cursor = ''
        document.body.style.userSelect = ''
    }

    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
    document.body.style.cursor = 'ew-resize'
    document.body.style.userSelect = 'none'
}
</script>

<template>
    <aside
        class="queue-sidebar"
        :class="{ resizing: isResizing, collapsed: sidebarCollapsed }"
        :style="sidebarCollapsed ? undefined : { width: sidebarWidth + 'px' }"
    >
        <div v-if="!sidebarCollapsed" class="resize-handle" @mousedown="startResize"></div>

        <div class="queue-header">
            <button
                class="collapse-btn"
                type="button"
                :aria-label="sidebarCollapsed ? 'Expand queue' : 'Collapse queue'"
                v-tooltip.left="sidebarCollapsed ? 'Expand queue' : 'Collapse'"
                @click="toggleSidebar"
            >
                <i :class="sidebarCollapsed ? 'pi pi-angle-left' : 'pi pi-angle-right'"></i>
            </button>
            <template v-if="!sidebarCollapsed">
                <div class="header-content">
                    <h3>
                        Queue
                        <span v-if="trackCount > 0" class="queue-info">
                            {{ trackCount }} {{ trackCount === 1 ? 'track' : 'tracks' }} &bull;
                            {{ totalDuration }}
                        </span>
                    </h3>
                </div>
                <Button
                    icon="pi pi-ellipsis-v"
                    text
                    rounded
                    size="small"
                    severity="secondary"
                    :disabled="trackCount === 0"
                    @click="toggleHeaderMenu"
                    v-tooltip.left="'Queue options'"
                />
            </template>
        </div>

        <div v-if="sidebarCollapsed" class="queue-collapsed">
            <div v-if="trackCount === 0" class="empty-state-collapsed">
                <i class="pi pi-list"></i>
            </div>
            <div v-else class="collapsed-list">
                <button
                    v-for="item in queueWithIndex"
                    :key="item.index"
                    type="button"
                    class="collapsed-item"
                    :class="{ current: item.isCurrent }"
                    v-tooltip.left="`${item.title} — ${item.artist || 'Unknown'}`"
                    @click="player.playQueueItem(item.index)"
                >
                    <div class="collapsed-cover">
                        <img
                            v-if="getCoverUrl(item.coverArt)"
                            :src="getCoverUrl(item.coverArt)!"
                            alt=""
                        />
                        <i v-else class="pi pi-music"></i>
                    </div>
                </button>
            </div>
        </div>

        <div v-else-if="trackCount === 0" class="empty-state">
            <i class="pi pi-list" style="font-size: 2rem"></i>
            <p>Queue is empty</p>
        </div>

        <div v-else class="queue-content">
            <DataTable
                :value="queueWithIndex"
                scrollable
                scroll-height="flex"
                :row-class="(data) => (data.isCurrent ? 'current-track' : '')"
                @row-click="onRowClick"
                class="queue-table"
            >
                <Column style="width: 48px" body-class="play-indicator-cell">
                    <template #body="slotProps">
                        <div
                            class="play-indicator-container"
                            @mouseenter="hoveredTrackIndex = slotProps.data.index"
                            @mouseleave="hoveredTrackIndex = null"
                            @click.stop="handlePlayPauseClick($event, slotProps.data.index)"
                        >
                            <i
                                v-if="
                                    hoveredTrackIndex === slotProps.data.index &&
                                    (!slotProps.data.isCurrent || !player.isPlaying.value)
                                "
                                class="pi pi-play play-hover-icon"
                            ></i>
                            <i
                                v-else-if="
                                    hoveredTrackIndex === slotProps.data.index &&
                                    slotProps.data.isCurrent &&
                                    player.isPlaying.value
                                "
                                class="pi pi-pause play-hover-icon"
                            ></i>
                            <i
                                v-else-if="slotProps.data.isCurrent && player.isPlaying.value"
                                class="pi pi-volume-up playing-icon"
                            ></i>
                            <i
                                v-else-if="slotProps.data.isCurrent"
                                class="pi pi-pause playing-icon"
                            ></i>
                            <span v-else class="track-number">{{
                                slotProps.data.index + 1
                            }}</span>
                        </div>
                    </template>
                </Column>

                <Column field="title" style="min-width: 0">
                    <template #body="slotProps">
                        <div class="track-cell">
                            <div class="track-cover">
                                <img
                                    v-if="getCoverUrl(slotProps.data.coverArt)"
                                    :src="getCoverUrl(slotProps.data.coverArt)!"
                                    alt=""
                                />
                                <i v-else class="pi pi-music"></i>
                            </div>
                            <div class="track-info">
                                <div class="track-title">{{ slotProps.data.title }}</div>
                                <div class="track-artist">
                                    {{ slotProps.data.artist || 'Unknown' }}
                                </div>
                            </div>
                        </div>
                    </template>
                </Column>

                <Column style="width: 64px" body-class="duration-cell">
                    <template #body="slotProps">
                        <span class="track-duration">{{
                            formatDuration(slotProps.data.duration)
                        }}</span>
                        <Button
                            icon="pi pi-trash"
                            text
                            rounded
                            size="small"
                            severity="secondary"
                            class="remove-button"
                            @click.stop="player.removeFromQueue(slotProps.data.index)"
                            v-tooltip.left="'Remove from queue'"
                        />
                    </template>
                </Column>

            </DataTable>
        </div>

        <Menu ref="headerMenu" :model="headerMenuItems" :popup="true" />

        <SavePlaylistDialog
            v-model:visible="showSaveDialog"
            v-model:name="playlistName"
            :saving="isSaving"
            @save="handleSave"
        />
    </aside>
</template>

<style scoped>
.queue-sidebar {
    height: 100%;
    background-color: var(--app-surface);
    border-left: 1px solid var(--app-border);
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    position: relative;
    overflow: hidden;
    transition: width 0.3s ease;
}

.queue-sidebar.resizing {
    transition: none;
}

.queue-sidebar.collapsed {
    width: var(--app-sidebar-collapsed-width);
}

.queue-sidebar.collapsed .queue-header {
    justify-content: center;
    padding: 0.5rem;
}

.queue-collapsed {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
    overflow-x: hidden;
    padding: 0.5rem 0;
    gap: 0.4rem;
}

.collapsed-list {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
}

.collapsed-item {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0.25rem 0;
    border: none;
    background: none;
    cursor: pointer;
    transition: background-color 0.15s;
    width: 100%;
}

.collapsed-item:hover {
    background-color: var(--app-background);
}

.collapsed-item.current {
    background-color: #eef2ff;
    box-shadow: inset 3px 0 0 var(--app-accent);
}

.collapsed-item.current:hover {
    background-color: #e0e7ff;
}

.collapsed-cover {
    width: 40px;
    height: 40px;
    border-radius: 4px;
    overflow: hidden;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    display: flex;
    align-items: center;
    justify-content: center;
}

.collapsed-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.collapsed-cover i {
    font-size: 0.9rem;
    color: rgba(255, 255, 255, 0.85);
}

.empty-state-collapsed {
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1rem 0;
    color: var(--app-text-secondary);
    font-size: 1.2rem;
}

.resize-handle {
    position: absolute;
    top: 0;
    left: 0;
    width: 6px;
    height: 100%;
    cursor: col-resize;
    z-index: 10;
}

.resize-handle:hover {
    background-color: var(--app-accent);
    opacity: 0.3;
}

.queue-header {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.75rem 0.5rem 0.5rem;
    min-height: 3rem;
    box-sizing: border-box;
    flex-shrink: 0;
}

.collapse-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2rem;
    height: 2rem;
    border: none;
    background: transparent;
    color: var(--app-text-secondary);
    cursor: pointer;
    border-radius: 50%;
    flex-shrink: 0;
    transition: background-color 0.15s, color 0.15s;
}

.collapse-btn:hover {
    background-color: var(--app-background);
    color: var(--app-text-primary);
}

.header-content {
    flex: 1;
    min-width: 0;
}

.header-content h3 {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
}

.queue-info {
    margin-left: 0.5rem;
    font-size: 0.8rem;
    font-weight: 400;
    color: var(--app-text-secondary);
}

.empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    flex: 1;
    gap: 0.75rem;
    color: var(--app-text-secondary);
}

.queue-content {
    flex: 1;
    overflow: hidden;
    display: flex;
    flex-direction: column;
}

.queue-table {
    flex: 1;
}

.queue-table :deep(.p-datatable-wrapper) {
    height: 100%;
}

.queue-table :deep(.p-datatable-thead) {
    display: none;
}

.queue-table :deep(.p-datatable-tbody > tr) {
    cursor: pointer;
    transition: background-color 0.15s;
}

.queue-table :deep(.p-datatable-tbody > tr > td) {
    padding: 0.5rem 0.25rem;
    border: none;
}

.queue-table :deep(.p-datatable-tbody > tr > td:first-child) {
    padding-left: 0.5rem;
}

.queue-table :deep(.p-datatable-tbody > tr > td:last-child) {
    padding-right: 0.5rem;
}

.queue-table :deep(.p-datatable-tbody > tr:hover) {
    background-color: var(--app-background);
}

.queue-table :deep(.current-track) {
    background-color: #eef2ff;
}

.queue-table :deep(.current-track:hover) {
    background-color: #e0e7ff;
}

.queue-table :deep(.current-track td:first-child) {
    box-shadow: inset 3px 0 0 var(--app-accent);
}

.play-indicator-cell {
    text-align: center;
}

.play-indicator-container {
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    width: 100%;
    height: 40px;
}

.playing-icon {
    color: var(--app-accent);
    font-size: 1rem;
}

.play-hover-icon {
    color: var(--app-text-primary);
    font-size: 1rem;
    transition: color 0.15s, transform 0.15s;
}

.play-hover-icon:hover {
    color: var(--app-accent);
    transform: scale(1.1);
}

.track-number {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    font-weight: 500;
}

.track-cell {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    min-width: 0;
}

.track-cover {
    width: 40px;
    height: 40px;
    flex-shrink: 0;
    border-radius: 4px;
    overflow: hidden;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    display: flex;
    align-items: center;
    justify-content: center;
}

.track-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.track-cover i {
    font-size: 0.9rem;
    color: rgba(255, 255, 255, 0.85);
}

.track-info {
    min-width: 0;
    flex: 1;
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

.current-track .track-title {
    color: var(--app-accent-hover);
    font-weight: 600;
}

.current-track .track-artist {
    color: var(--app-accent);
}

.duration-cell {
    text-align: right;
    position: relative;
}

.track-duration {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}

.queue-table :deep(.p-datatable-tbody > tr:hover .track-duration) {
    visibility: hidden;
}

.queue-table :deep(.remove-button) {
    position: absolute;
    top: 50%;
    right: 0.25rem;
    transform: translateY(-50%);
    opacity: 0;
    pointer-events: none;
    transition: opacity 0.15s;
}

.queue-table :deep(.p-datatable-tbody > tr:hover .remove-button) {
    opacity: 1;
    pointer-events: auto;
}

@media (max-width: 1200px) {
    .queue-sidebar {
        display: none;
    }
}

</style>
