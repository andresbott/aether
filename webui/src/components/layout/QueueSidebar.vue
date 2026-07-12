<script setup lang="ts">
import { ref, computed } from 'vue'
import QueueView from '@/components/layout/QueueView.vue'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueSidebar } from '@/composables/useQueueSidebar'
import { subsonicClient } from '@/lib/api/subsonic'

const player = usePlayer()
const { sidebarCollapsed, sidebarWidth, toggleSidebar, setSidebarWidth } = useQueueSidebar()

const isResizing = ref(false)

const collapsedItems = computed(() =>
    player.queue.value.map((song, index) => ({
        ...song,
        index,
        isCurrent: index === player.currentIndex.value
    }))
)

const trackCount = computed(() => player.queue.value.length)

const getCoverUrl = (coverArt?: string): string | null => {
    if (!coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(coverArt, 48)
}

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

        <template v-if="sidebarCollapsed">
            <div class="collapsed-header">
                <button
                    class="collapse-btn"
                    type="button"
                    aria-label="Expand queue"
                    v-tooltip.left="'Expand queue'"
                    @click="toggleSidebar"
                >
                    <i class="pi pi-angle-left"></i>
                </button>
            </div>
            <div class="queue-collapsed">
                <div v-if="trackCount === 0" class="empty-state-collapsed">
                    <i class="pi pi-list"></i>
                </div>
                <div v-else class="collapsed-list">
                    <button
                        v-for="item in collapsedItems"
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
        </template>

        <QueueView v-else variant="sidebar">
            <template #header-start>
                <button
                    class="collapse-btn"
                    type="button"
                    aria-label="Collapse queue"
                    v-tooltip.left="'Collapse'"
                    @click="toggleSidebar"
                >
                    <i class="pi pi-angle-right"></i>
                </button>
            </template>
        </QueueView>
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

.collapsed-header {
    display: flex;
    justify-content: center;
    padding: 0.5rem;
    min-height: 3rem;
    box-sizing: border-box;
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
    background-color: var(--app-accent-soft);
    box-shadow: inset 3px 0 0 var(--app-accent);
}

.collapsed-item.current:hover {
    background-color: var(--app-accent-soft-hover);
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

@media (max-width: 1200px) {
    .queue-sidebar {
        display: none;
    }
}
</style>
