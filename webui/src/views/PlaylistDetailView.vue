<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import TrackEditList from '@/components/layout/TrackEditList.vue'
import {
    usePlaylist,
    useUpdatePlaylist,
    useDeletePlaylist,
    useReplacePlaylistTracks
} from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { reorderQueue } from '@/utils/queueReorder'
import type { Song } from '@/types/subsonic'

const props = defineProps<{ id: string }>()
const router = useRouter()
const player = usePlayer()

const { data: playlist, isLoading, error } = usePlaylist(props.id)
const updatePlaylist = useUpdatePlaylist()
const deletePlaylist = useDeletePlaylist()
const replaceTracks = useReplacePlaylistTracks()

// Inline rename state.
const renaming = ref(false)
const renameValue = ref('')

// Batched edit state: a local working copy of the entries.
const editMode = ref(false)
const working = ref<Song[]>([])

const summary = computed(() => {
    if (!playlist.value) return ''
    const parts: string[] = []
    const n = editMode.value
        ? working.value.length
        : (playlist.value.songCount ?? playlist.value.entry?.length ?? 0)
    if (n > 0) parts.push(`${n} ${n === 1 ? 'song' : 'songs'}`)
    if (!editMode.value && playlist.value.duration)
        parts.push(`${Math.floor(playlist.value.duration / 60)} min`)
    return parts.join(' • ')
})

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const playAll = (): void => {
    if (playlist.value?.entry) player.playAlbum(playlist.value.entry)
}

const playFromTrack = (index: number): void => {
    if (playlist.value?.entry) player.playAlbum(playlist.value.entry, index)
}

// --- Inline rename ---
const openRename = (): void => {
    renameValue.value = playlist.value?.name ?? ''
    renaming.value = true
}
const cancelRename = (): void => {
    renaming.value = false
}
const submitRename = (): void => {
    const name = renameValue.value.trim()
    if (!name) return
    updatePlaylist.mutate(
        { playlistId: props.id, name },
        { onSuccess: () => (renaming.value = false) }
    )
}

// --- Batched track edit ---
const enterEdit = (): void => {
    working.value = [...(playlist.value?.entry ?? [])]
    editMode.value = true
}
const cancelEdit = (): void => {
    editMode.value = false
    working.value = []
}
const onReorder = (indices: number[], target: number): void => {
    working.value = reorderQueue(working.value, indices, target)
}
const onDelete = (indices: number[]): void => {
    const drop = new Set(indices)
    working.value = working.value.filter((_, i) => !drop.has(i))
}
const saveEdit = (): void => {
    replaceTracks.mutate(
        { playlistId: props.id, songIds: working.value.map((s) => s.id) },
        { onSuccess: () => cancelEdit() }
    )
}

const handleDelete = (): void => {
    deletePlaylist.mutate(props.id, { onSuccess: () => router.push({ name: 'playlists' }) })
}

// Leaving edit mode / switching playlists drops any working copy and rename draft.
const resetOnIdChange = (): void => {
    cancelEdit()
    cancelRename()
}
watch(() => props.id, resetOnIdChange)
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

        <ContentScaffold v-else-if="playlist" :title="renaming ? '' : playlist.name" :summary="summary">
            <template #title-actions>
                <span v-if="renaming" class="rename-input">
                    <InputText
                        v-model="renameValue"
                        autofocus
                        @keyup.enter="submitRename"
                        @keyup.esc="cancelRename"
                    />
                    <Button icon="pi pi-check" text rounded size="small" @click="submitRename" />
                    <Button icon="pi pi-times" text rounded size="small" @click="cancelRename" />
                </span>
                <Button
                    v-else
                    class="rename-toggle"
                    icon="pi pi-pencil"
                    text
                    rounded
                    size="small"
                    v-tooltip.bottom="'Rename playlist'"
                    @click="openRename"
                />
            </template>

            <template #actions>
                <template v-if="editMode">
                    <Button
                        class="edit-save"
                        label="Save"
                        icon="pi pi-check"
                        :loading="replaceTracks.isPending.value"
                        @click="saveEdit"
                    />
                    <Button
                        class="edit-cancel"
                        label="Cancel"
                        text
                        severity="secondary"
                        :disabled="replaceTracks.isPending.value"
                        @click="cancelEdit"
                    />
                </template>
                <template v-else>
                    <Button class="play-all" label="Play" icon="pi pi-play" @click="playAll" />
                    <Button
                        class="edit-toggle"
                        icon="pi pi-list"
                        text
                        rounded
                        :disabled="!playlist.entry || playlist.entry.length === 0"
                        v-tooltip.bottom="'Edit tracks'"
                        @click="enterEdit"
                    />
                    <Button
                        icon="pi pi-trash"
                        text
                        rounded
                        severity="danger"
                        v-tooltip.bottom="'Delete playlist'"
                        @click="handleDelete"
                    />
                </template>
            </template>

            <div class="playlist-scroll">
                <div class="playlist-body">
                    <TrackEditList
                        v-if="editMode"
                        :songs="working"
                        delete-label="Remove from playlist"
                        group="playlist"
                        @reorder="onReorder"
                        @delete="onDelete"
                    />

                    <DataTable
                        v-else-if="playlist.entry && playlist.entry.length > 0"
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
                            <template #body="{ data }">{{ formatDuration(data.duration) }}</template>
                        </Column>
                    </DataTable>

                    <div v-else class="empty-tracks">
                        <p>This playlist is empty</p>
                    </div>
                </div>
            </div>
        </ContentScaffold>
    </div>
</template>

<style scoped>
.playlist-detail-view { height: 100%; display: flex; flex-direction: column; min-height: 0; }
.back-row { flex-shrink: 0; padding: 0.5rem 2rem 0; }
.loading, .error { display: flex; flex-direction: column; align-items: center; padding: 3rem; gap: 1rem; color: var(--app-text-secondary); }
.error { color: #ef4444; }
.playlist-scroll { height: 100%; overflow-y: auto; scrollbar-gutter: stable; }
.playlist-body { max-width: var(--app-content-max-width); margin: 0 auto; padding: 1rem; }
.rename-input { display: inline-flex; align-items: center; gap: 0.25rem; }
.track-table :deep(.clickable-row) { cursor: pointer; }
.track-table :deep(.clickable-row:hover) { background-color: var(--app-hover) !important; }
.empty-tracks { padding: 3rem; text-align: center; color: var(--app-text-secondary); }
</style>
