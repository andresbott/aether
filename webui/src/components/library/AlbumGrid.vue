<script setup lang="ts">
import { computed, toRef } from 'vue'
import VirtualCardGrid from '@/components/library/VirtualCardGrid.vue'
import AlbumCard from '@/components/library/AlbumCard.vue'
import { ALBUM_PAGE_SIZE } from '@/composables/useAlbumTable'
import { useAlbumSource } from '@/composables/useLibrarySource'

const props = defineProps<{ folderId?: number; favoritesOnly?: boolean }>()

const { total, letters, items, isLoading, error, ensureRange } = useAlbumSource(
    toRef(props, 'folderId'),
    computed(() => props.favoritesOnly === true)
)

function onLazyLoad(first: number, last: number): void {
    void ensureRange(first, last)
}
</script>

<template>
    <div class="album-grid-view">
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>
        <div v-else-if="error" class="empty-state">
            <i class="pi pi-exclamation-triangle" style="font-size: 3rem"></i>
            <p>Could not load albums</p>
        </div>
        <div v-else-if="total === 0" class="empty-state">
            <i :class="favoritesOnly ? 'pi pi-heart' : 'pi pi-music'" style="font-size: 3rem"></i>
            <p v-if="favoritesOnly">No favorite albums yet</p>
            <p v-else>No albums found</p>
        </div>
        <!-- Keyed on the source too: switching between all albums and favorites
             swaps the whole dataset, so the grid must remeasure rather than keep
             the previous scroll offset and row range. -->
        <VirtualCardGrid
            v-else
            :key="`${folderId ?? 'all'}-${favoritesOnly ? 'fav' : 'all'}`"
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
