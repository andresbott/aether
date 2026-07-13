<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import {
    usePlaylist,
    useUpdatePlaylist,
    useDeletePlaylist
} from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'

const props = defineProps<{ id: string }>()
const router = useRouter()
const player = usePlayer()

const { data: playlist, isLoading, error } = usePlaylist(props.id)
const updatePlaylist = useUpdatePlaylist()
const deletePlaylist = useDeletePlaylist()

const showRenameDialog = ref(false)
const newName = ref('')

const summary = computed(() => {
    if (!playlist.value) return ''
    const parts: string[] = []
    const n = playlist.value.songCount ?? playlist.value.entry?.length ?? 0
    if (n > 0) parts.push(`${n} ${n === 1 ? 'song' : 'songs'}`)
    if (playlist.value.duration) parts.push(`${Math.floor(playlist.value.duration / 60)} min`)
    return parts.join(' • ')
})

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const playAll = () => {
    if (playlist.value?.entry) {
        player.playAlbum(playlist.value.entry)
    }
}

const playFromTrack = (index: number) => {
    if (playlist.value?.entry) {
        player.playAlbum(playlist.value.entry, index)
    }
}

const removeTrack = (index: number) => {
    updatePlaylist.mutate({ playlistId: props.id, songIndexesToRemove: [index] })
}

const openRename = () => {
    newName.value = playlist.value?.name || ''
    showRenameDialog.value = true
}

const handleRename = () => {
    if (!newName.value.trim()) return
    updatePlaylist.mutate(
        { playlistId: props.id, name: newName.value.trim() },
        { onSuccess: () => { showRenameDialog.value = false } }
    )
}

const handleDelete = () => {
    deletePlaylist.mutate(props.id, {
        onSuccess: () => router.push({ name: 'playlists' })
    })
}
</script>

<template>
    <div class="playlist-detail-view">
        <div class="back-row">
            <Button icon="pi pi-arrow-left" text rounded @click="router.back()" />
        </div>

        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="error" class="error">
            <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
            <p>{{ error.message }}</p>
        </div>

        <ContentScaffold v-else-if="playlist" :title="playlist.name" :summary="summary">
            <template #actions>
                <Button label="Play" icon="pi pi-play" @click="playAll" />
                <Button icon="pi pi-pencil" text rounded @click="openRename" />
                <Button icon="pi pi-trash" text rounded severity="danger" @click="handleDelete" />
            </template>

            <div class="playlist-scroll">
                <div class="playlist-body">
                    <DataTable
                        v-if="playlist.entry && playlist.entry.length > 0"
                        :value="playlist.entry"
                        stripedRows
                        @row-click="(e: any) => playFromTrack(e.index)"
                        class="track-table"
                        :rowClass="() => 'clickable-row'"
                    >
                        <Column header="#" style="width: 60px">
                            <template #body="{ index }">{{ index + 1 }}</template>
                        </Column>
                        <Column field="title" header="Title" />
                        <Column field="artist" header="Artist" />
                        <Column field="album" header="Album" />
                        <Column header="Duration" style="width: 80px">
                            <template #body="{ data }">
                                {{ formatDuration(data.duration) }}
                            </template>
                        </Column>
                        <Column style="width: 60px">
                            <template #body="{ index }">
                                <Button
                                    icon="pi pi-times"
                                    text
                                    rounded
                                    size="small"
                                    severity="danger"
                                    @click.stop="removeTrack(index)"
                                />
                            </template>
                        </Column>
                    </DataTable>

                    <div v-else class="empty-tracks">
                        <p>This playlist is empty</p>
                    </div>
                </div>
            </div>
        </ContentScaffold>

        <Dialog
            v-model:visible="showRenameDialog"
            header="Rename Playlist"
            :modal="true"
            :style="{ width: '400px' }"
        >
            <InputText
                v-model="newName"
                class="w-full"
                @keyup.enter="handleRename"
            />
            <template #footer>
                <Button label="Cancel" text @click="showRenameDialog = false" />
                <Button label="Save" @click="handleRename" />
            </template>
        </Dialog>
    </div>
</template>

<style scoped>
.playlist-detail-view { height: 100%; display: flex; flex-direction: column; min-height: 0; }
.back-row { flex-shrink: 0; padding: 0.5rem 2rem 0; }
.loading, .error { display: flex; flex-direction: column; align-items: center; padding: 3rem; gap: 1rem; color: var(--app-text-secondary); }
.error { color: #ef4444; }
.playlist-scroll { height: 100%; overflow-y: auto; scrollbar-gutter: stable; }
.playlist-body { max-width: var(--app-content-max-width); margin: 0 auto; padding: 1rem; }
.track-table :deep(.clickable-row) { cursor: pointer; }
.track-table :deep(.clickable-row:hover) { background-color: #f9fafb !important; }
.empty-tracks { padding: 3rem; text-align: center; color: var(--app-text-secondary); }
</style>
