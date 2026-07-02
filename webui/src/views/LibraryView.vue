<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SelectButton from 'primevue/selectbutton'
import AlbumCard from '@/components/library/AlbumCard.vue'
import ArtistCard from '@/components/library/ArtistCard.vue'
import AlbumListView from '@/components/library/AlbumListView.vue'
import {
    useAlbumList,
    useArtists,
    useMusicFolders
} from '@/composables/useSubsonicQueries'

type ViewMode = 'albums' | 'artists'
type AlbumLayout = 'grid' | 'list'

const route = useRoute()
const router = useRouter()

const viewOptions = [
    { label: 'Albums', value: 'albums' },
    { label: 'Artists', value: 'artists' }
]

const folderId = computed<number | undefined>(() => {
    const raw = route.params.folderId
    const value = Array.isArray(raw) ? raw[0] : raw
    if (!value) return undefined
    const num = Number(value)
    return Number.isFinite(num) ? num : undefined
})

const { data: folders } = useMusicFolders()
const folder = computed(() =>
    folders.value?.find((f) => f.id === folderId.value)
)
const folderName = computed(() => {
    if (folderId.value === undefined) return 'Library'
    return folder.value?.name ?? 'Library'
})

const serverDefault = computed<ViewMode>(
    () => folder.value?.defaultView ?? 'albums'
)

const hashView = computed<ViewMode | null>(() => {
    const h = route.hash.replace('#', '')
    return h === 'albums' || h === 'artists' ? h : null
})

const viewMode = computed<ViewMode>({
    get: () => hashView.value ?? serverDefault.value,
    set: (v) => {
        router.replace({ hash: `#${v}`, query: route.query })
    }
})

const layoutOptions = [
    { label: 'Grid', value: 'grid', icon: 'pi pi-th-large' },
    { label: 'List', value: 'list', icon: 'pi pi-list' }
]

const albumLayout = computed<AlbumLayout>({
    get: () => (route.query.view === 'list' ? 'list' : 'grid'),
    set: (v) => {
        const query = { ...route.query }
        if (v === 'list') query.view = 'list'
        else delete query.view
        router.replace({ hash: route.hash, query })
    }
})

const { data: albums, isLoading: albumsLoading } = useAlbumList('newest', 50, 0, folderId)
const { data: artists, isLoading: artistsLoading } = useArtists(folderId)
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
            <SelectButton
                v-if="viewMode === 'albums'"
                v-model="albumLayout"
                :options="layoutOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
                dataKey="value"
                aria-label="Album layout"
            >
                <template #option="slotProps">
                    <i :class="slotProps.option.icon"></i>
                </template>
            </SelectButton>
        </div>

        <div v-if="viewMode === 'albums'">
            <AlbumListView v-if="albumLayout === 'list'" :folderId="folderId" />
            <template v-else>
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
            </template>
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
</style>
