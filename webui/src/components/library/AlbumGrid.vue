<script setup lang="ts">
import { toRef } from 'vue'
import AlbumCard from '@/components/library/AlbumCard.vue'
import { useAlbumList } from '@/composables/useSubsonicQueries'

const props = defineProps<{ folderId?: number }>()
const { data: albums, isLoading } = useAlbumList('newest', 50, 0, toRef(props, 'folderId'))
</script>

<template>
    <div class="grid-scroll">
        <div v-if="isLoading" class="loading">
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
</template>

<style scoped>
.grid-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
}

.album-grid {
    max-width: 1400px;
    margin: 0 auto;
    padding: 1rem;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 2rem;
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
