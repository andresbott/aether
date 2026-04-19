<script setup lang="ts">
import { ref, computed } from 'vue'
import Button from 'primevue/button'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueSidebar } from '@/composables/useQueueSidebar'
import { subsonicClient } from '@/lib/api/subsonic'

const player = usePlayer()
const { sidebarCollapsed, sidebarWidth, setSidebarWidth, MIN_WIDTH, MAX_WIDTH } =
    useQueueSidebar()

const isResizing = ref(false)

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
    }

    document.addEventListener('mousemove', onMouseMove)
    document.addEventListener('mouseup', onMouseUp)
}

const trackCount = computed(() => player.queue.value.length)

const totalDuration = computed(() => {
    const total = player.queue.value.reduce((sum, song) => sum + (song.duration || 0), 0)
    const mins = Math.floor(total / 60)
    return `${mins} min`
})

const getCoverUrl = (coverArt?: string): string | null => {
    if (!coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(coverArt, 40)
}

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}
</script>

<template>
    <aside
        v-if="!sidebarCollapsed"
        class="queue-sidebar"
        :style="{ width: sidebarWidth + 'px' }"
    >
        <div class="resize-handle" @mousedown="startResize"></div>

        <div class="queue-header">
            <div>
                <h3>Current Queue</h3>
                <span v-if="trackCount > 0" class="queue-info">
                    {{ trackCount }} tracks &bull; {{ totalDuration }}
                </span>
            </div>
            <Button
                v-if="trackCount > 0"
                icon="pi pi-trash"
                text
                rounded
                size="small"
                severity="secondary"
                @click="player.clearQueue"
                v-tooltip="'Clear queue'"
            />
        </div>

        <div v-if="trackCount === 0" class="empty-state">
            <i class="pi pi-list" style="font-size: 2rem"></i>
            <p>Queue is empty</p>
        </div>

        <div v-else class="queue-list">
            <div
                v-for="(song, index) in player.queue.value"
                :key="index"
                class="queue-item"
                :class="{ active: index === player.currentIndex.value }"
                @click="player.playQueueItem(index)"
            >
                <div class="queue-item-cover">
                    <img
                        v-if="getCoverUrl(song.coverArt)"
                        :src="getCoverUrl(song.coverArt)!"
                        alt=""
                    />
                    <div v-else class="cover-mini-placeholder">
                        <i class="pi pi-music"></i>
                    </div>
                </div>
                <div class="queue-item-info">
                    <div class="queue-item-title">{{ song.title }}</div>
                    <div class="queue-item-artist">{{ song.artist }}</div>
                </div>
                <span class="queue-item-duration">{{ formatDuration(song.duration) }}</span>
            </div>
        </div>
    </aside>
</template>

<style scoped>
.queue-sidebar {
    height: calc(100vh - var(--app-player-height));
    background-color: var(--app-surface);
    border-left: 1px solid var(--app-border);
    display: flex;
    flex-direction: column;
    flex-shrink: 0;
    position: relative;
    overflow: hidden;
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
    justify-content: space-between;
    padding: 1rem 1.25rem;
    border-bottom: 1px solid var(--app-border);
    flex-shrink: 0;
}

.queue-header h3 {
    margin: 0;
    font-size: 1rem;
    font-weight: 600;
}

.queue-info {
    font-size: 0.8rem;
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

.queue-list {
    flex: 1;
    overflow-y: auto;
    padding: 0.5rem;
}

.queue-item {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.5rem 0.75rem;
    border-radius: 6px;
    cursor: pointer;
    transition: background-color 0.15s;
}

.queue-item:hover {
    background-color: var(--app-background);
}

.queue-item.active {
    background-color: #eef2ff;
    border-left: 3px solid var(--app-accent);
}

.queue-item-cover {
    width: 40px;
    height: 40px;
    border-radius: 4px;
    overflow: hidden;
    flex-shrink: 0;
}

.queue-item-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.cover-mini-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: rgba(255, 255, 255, 0.8);
    font-size: 0.8rem;
}

.queue-item-info {
    flex: 1;
    min-width: 0;
}

.queue-item-title {
    font-size: 0.85rem;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.queue-item-artist {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.queue-item-duration {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
    flex-shrink: 0;
}

@media (max-width: 1200px) {
    .queue-sidebar {
        display: none;
    }
}
</style>
