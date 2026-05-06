<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import SelectButton from 'primevue/selectbutton'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import AlbumCard from '@/components/library/AlbumCard.vue'
import ArtistCard from '@/components/library/ArtistCard.vue'
import {
    useAlbumList,
    useArtists,
    useMusicFolders,
    useRandomSongs
} from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'

const route = useRoute()

const viewMode = ref('albums')
const viewOptions = [
    { label: 'Albums', value: 'albums' },
    { label: 'Artists', value: 'artists' },
    { label: 'Songs', value: 'songs' }
]

const folderId = computed<number | undefined>(() => {
    const raw = route.params.folderId
    const value = Array.isArray(raw) ? raw[0] : raw
    if (!value) return undefined
    const num = Number(value)
    return Number.isFinite(num) ? num : undefined
})

const { data: folders } = useMusicFolders()
const folderName = computed(() => {
    if (folderId.value === undefined) return 'Library'
    return folders.value?.find((f) => f.id === folderId.value)?.name ?? 'Library'
})

const { data: albums, isLoading: albumsLoading } = useAlbumList('newest', 50, 0, folderId)
const { data: artists, isLoading: artistsLoading } = useArtists(folderId)
const { data: songs, isLoading: songsLoading } = useRandomSongs(100, folderId)
const player = usePlayer()

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const playSong = (index: number) => {
    if (songs.value) {
        player.playAlbum(songs.value, index)
    }
}
</script>

<template>
    <div class="library-view">
        <div class="library-header">
            <h1>{{ folderName }}</h1>
            <SelectButton
                v-model="viewMode"
                :options="viewOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
            />
        </div>

        <div v-if="viewMode === 'albums'">
            <div v-if="albumsLoading" class="loading">
                <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
            </div>
            <div v-else-if="albums && albums.length > 0" class="album-grid">
                <AlbumCard v-for="album in albums" :key="album.id" :album="album" />
            </div>
            <div v-else class="empty-state">
                <i class="pi pi-music" style="font-size: 3rem"></i>
                <p>No albums found</p>
            </div>
        </div>

        <div v-else-if="viewMode === 'artists'">
            <div v-if="artistsLoading" class="loading">
                <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
            </div>
            <div v-else-if="artists && artists.length > 0" class="artist-grid">
                <ArtistCard
                    v-for="artist in artists"
                    :key="artist.id"
                    :artist="artist"
                />
            </div>
            <div v-else class="empty-state">
                <i class="pi pi-users" style="font-size: 3rem"></i>
                <p>No artists found</p>
            </div>
        </div>

        <div v-else-if="viewMode === 'songs'">
            <div v-if="songsLoading" class="loading">
                <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
            </div>
            <DataTable
                v-else-if="songs && songs.length > 0"
                :value="songs"
                stripedRows
                @row-click="(e: any) => playSong(e.index)"
                class="track-table"
                :rowClass="() => 'clickable-row'"
            >
                <Column field="title" header="Title" />
                <Column field="artist" header="Artist" />
                <Column field="album" header="Album" />
                <Column header="Duration" style="width: 80px">
                    <template #body="{ data }">
                        {{ formatDuration(data.duration) }}
                    </template>
                </Column>
            </DataTable>
            <div v-else class="empty-state">
                <i class="pi pi-list" style="font-size: 3rem"></i>
                <p>No songs found</p>
            </div>
        </div>
    </div>
</template>

<style scoped>
.library-view {
    max-width: 1400px;
    margin: 0 auto;
}

.library-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 2rem;
}

.library-header h1 {
    font-size: 2rem;
    font-weight: 700;
    margin: 0;
}

.album-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 2rem;
}

.artist-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 2rem;
    justify-items: center;
}

.loading {
    display: flex;
    justify-content: center;
    padding: 3rem;
    color: var(--app-text-secondary);
}

.empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 4rem;
    gap: 1rem;
    color: var(--app-text-secondary);
}

.track-table :deep(.clickable-row) {
    cursor: pointer;
}

.track-table :deep(.clickable-row:hover) {
    background-color: #f9fafb !important;
}
</style>
