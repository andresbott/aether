<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import SongDetail from '@/components/library/SongDetail.vue'
import Button from 'primevue/button'
import Menu from 'primevue/menu'
import type { MenuItem } from 'primevue/menuitem'
import SavePlaylistDialog from '@/components/layout/SavePlaylistDialog.vue'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueActions } from '@/composables/useQueueActions'

const props = defineProps<{ index: string }>()
const router = useRouter()
const player = usePlayer()
const { showSaveDialog, playlistName, openSaveDialog, handleSave, isSaving, clearQueue } = useQueueActions()

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

const songIndex = computed(() => parseInt(props.index, 10))

const song = computed(() => {
    const idx = songIndex.value
    if (isNaN(idx) || idx < 0 || idx >= player.queue.value.length) return null
    return player.queue.value[idx]
})

const hasPrev = computed(() => songIndex.value > 0)
const hasNext = computed(() => songIndex.value < player.queue.value.length - 1)

const goToPrev = () => {
    if (hasPrev.value) {
        router.replace({ name: 'song', params: { index: String(songIndex.value - 1) } })
    }
}

const goToNext = () => {
    if (hasNext.value) {
        router.replace({ name: 'song', params: { index: String(songIndex.value + 1) } })
    }
}

const playCurrent = () => {
    player.playQueueItem(songIndex.value)
}
</script>

<template>
    <div class="song-view">
        <div class="song-nav">
            <Button icon="pi pi-arrow-left" text rounded @click="router.back()" />
            <div v-if="player.queue.value.length > 0" class="nav-controls">
                <Button icon="pi pi-chevron-left" text rounded :disabled="!hasPrev" @click="goToPrev" />
                <span class="queue-position">{{ songIndex + 1 }} / {{ player.queue.value.length }}</span>
                <Button icon="pi pi-chevron-right" text rounded :disabled="!hasNext" @click="goToNext" />
            </div>
            <Button
                icon="pi pi-ellipsis-v"
                text
                rounded
                :disabled="player.queue.value.length === 0"
                @click="toggleActionsMenu"
                v-tooltip.bottom="'Queue options'"
            />
        </div>

        <SongDetail v-if="song" :song="song" @play="playCurrent" />

        <div v-else class="empty-state">
            <i class="pi pi-music" style="font-size: 3rem"></i>
            <p>No song at this position in the queue</p>
        </div>

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
.song-view { max-width: 1000px; margin: 0 auto; }
.song-nav { display: flex; align-items: center; justify-content: space-between; margin-bottom: 1.5rem; }
.nav-controls { display: flex; align-items: center; gap: 0.5rem; }
.queue-position { font-size: 0.9rem; color: var(--app-text-secondary); min-width: 60px; text-align: center; }
.empty-state { display: flex; flex-direction: column; align-items: center; padding: 4rem; gap: 1rem; color: var(--app-text-secondary); }
</style>
