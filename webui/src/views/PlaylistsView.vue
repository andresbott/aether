<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import { usePlaylists, useCreatePlaylist } from '@/composables/useSubsonicQueries'
import { subsonicClient } from '@/lib/api/subsonic'

const router = useRouter()
const { data: playlists, isLoading } = usePlaylists()
const createPlaylist = useCreatePlaylist()

const showCreateDialog = ref(false)
const newPlaylistName = ref('')

const getCoverUrl = (coverArt?: string): string | null => {
    if (!coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(coverArt, 200)
}

const handleCreate = () => {
    if (!newPlaylistName.value.trim()) return
    createPlaylist.mutate(
        { name: newPlaylistName.value.trim() },
        {
            onSuccess: () => {
                showCreateDialog.value = false
                newPlaylistName.value = ''
            }
        }
    )
}

const openPlaylist = (id: string) => {
    router.push({ name: 'playlist-detail', params: { id } })
}
</script>

<template>
    <div class="playlists-view">
        <div class="view-header">
            <h1>Playlists</h1>
            <Button label="Create Playlist" icon="pi pi-plus" @click="showCreateDialog = true" />
        </div>

        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="playlists && playlists.length > 0" class="playlist-grid">
            <div
                v-for="pl in playlists"
                :key="pl.id"
                class="playlist-card"
                @click="openPlaylist(pl.id)"
            >
                <div class="playlist-cover">
                    <img v-if="getCoverUrl(pl.coverArt)" :src="getCoverUrl(pl.coverArt)!" alt="" />
                    <div v-else class="cover-placeholder">
                        <i class="pi pi-list" style="font-size: 2rem"></i>
                    </div>
                </div>
                <div class="playlist-info">
                    <div class="playlist-name">{{ pl.name }}</div>
                    <div class="playlist-meta">{{ pl.songCount }} songs</div>
                </div>
            </div>
        </div>

        <div v-else class="empty-state">
            <i class="pi pi-list" style="font-size: 3rem"></i>
            <p>No playlists</p>
        </div>

        <Dialog
            v-model:visible="showCreateDialog"
            header="Create Playlist"
            :modal="true"
            :style="{ width: '400px' }"
        >
            <div class="create-form">
                <InputText
                    v-model="newPlaylistName"
                    placeholder="Playlist name"
                    class="w-full"
                    @keyup.enter="handleCreate"
                />
            </div>
            <template #footer>
                <Button label="Cancel" text @click="showCreateDialog = false" />
                <Button
                    label="Create"
                    :loading="createPlaylist.isPending.value"
                    @click="handleCreate"
                />
            </template>
        </Dialog>
    </div>
</template>

<style scoped>
.playlists-view { max-width: 1400px; margin: 0 auto; }
.view-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 2rem; }
.view-header h1 { font-size: 2rem; font-weight: 700; margin: 0; }
.loading { display: flex; justify-content: center; padding: 3rem; color: var(--app-text-secondary); }
.playlist-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 2rem; }
.playlist-card { cursor: pointer; transition: transform 0.2s; }
.playlist-card:hover { transform: translateY(-2px); }
.playlist-cover { width: 100%; aspect-ratio: 1; border-radius: 8px; overflow: hidden; }
.playlist-cover img { width: 100%; height: 100%; object-fit: cover; }
.cover-placeholder { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: rgba(255, 255, 255, 0.8); }
.playlist-info { padding: 0.5rem 0.25rem; }
.playlist-name { font-size: 0.9rem; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.playlist-meta { font-size: 0.8rem; color: var(--app-text-secondary); }
.empty-state { display: flex; flex-direction: column; align-items: center; padding: 4rem; gap: 1rem; color: var(--app-text-secondary); }
.create-form { padding: 1rem 0; }
</style>
