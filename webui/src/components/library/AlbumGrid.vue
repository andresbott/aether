<script setup lang="ts">
import { computed, toRef } from 'vue'
import VirtualCardGrid from '@/components/library/VirtualCardGrid.vue'
import AlbumCard from '@/components/library/AlbumCard.vue'
import { ALBUM_PAGE_SIZE } from '@/composables/useAlbumTable'
import { useAlbumSource } from '@/composables/useLibrarySource'

const props = defineProps<{ folderId?: number; favoritesOnly?: boolean; releaseType?: string }>()

const { total, letters, items, isLoading, error, ensureRange } = useAlbumSource(
    toRef(props, 'folderId'),
    computed(() => props.favoritesOnly === true),
    computed(() => props.releaseType)
)

function onLazyLoad(first: number, last: number): void {
    void ensureRange(first, last)
}
</script>

<template>
    <div class="album-grid-view">
        <div v-if="isLoading" class="loading">
            <i class="icon-spin ms-progress-activity" style="font-size: 2rem"></i>
        </div>
        <div v-else-if="error" class="empty-state">
            <i class="ms-warning" style="font-size: 3rem"></i>
            <p>Could not load albums</p>
        </div>
        <div v-else-if="total === 0" class="empty-state">
            <i :class="favoritesOnly ? 'ms-favorite' : 'ms-music-note'" style="font-size: 3rem"></i>
            <p v-if="favoritesOnly">No favorite albums yet</p>
            <p v-else>No albums found</p>
        </div>
        <!-- Keyed on the source too: switching between all albums, favorites, or a
             release-type tab swaps the whole dataset, so the grid must remeasure
             rather than keep the previous scroll offset and row range. -->
        <VirtualCardGrid
            v-else
            :key="`${folderId ?? 'all'}-${favoritesOnly ? 'fav' : 'all'}-${releaseType ?? 'all'}`"
            :items="items"
            :letters="letters"
            :total="total"
            :pageSize="ALBUM_PAGE_SIZE"
            @lazyLoad="onLazyLoad"
        >
            <template #card="{ item }">
                <AlbumCard :album="item" />
            </template>
        </VirtualCardGrid>
    </div>
</template>

<style scoped>
.album-grid-view {
    height: 100%;
    min-height: 0;
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
