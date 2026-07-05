<script setup lang="ts">
import { toRef } from 'vue'
import VirtualCardGrid from '@/components/library/VirtualCardGrid.vue'
import AlbumCard from '@/components/library/AlbumCard.vue'
import { useAlbumTable, ALBUM_PAGE_SIZE } from '@/composables/useAlbumTable'

const props = defineProps<{ folderId?: number }>()

const { total, letters, items, isLoading, error, ensureRange } = useAlbumTable(
    toRef(props, 'folderId')
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
            <i class="pi pi-music" style="font-size: 3rem"></i>
            <p>No albums found</p>
        </div>
        <VirtualCardGrid
            v-else
            :key="folderId"
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
