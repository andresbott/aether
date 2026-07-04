<script setup lang="ts">
import { toRef } from 'vue'
import VirtualCardGrid from '@/components/library/VirtualCardGrid.vue'
import ArtistCard from '@/components/library/ArtistCard.vue'
import { useArtistTable } from '@/composables/useArtistTable'

const props = defineProps<{ folderId?: number }>()

const { total, letters, items, isLoading, error } = useArtistTable(toRef(props, 'folderId'))
</script>

<template>
    <div class="artist-grid-view">
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>
        <div v-else-if="error" class="empty-state">
            <i class="pi pi-exclamation-triangle" style="font-size: 3rem"></i>
            <p>Could not load artists</p>
        </div>
        <div v-else-if="total === 0" class="empty-state">
            <i class="pi pi-users" style="font-size: 3rem"></i>
            <p>No artists found</p>
        </div>
        <!-- lazyLoad not handled: useArtistTable returns a fully-materialized Artist[] with no paging. -->
        <VirtualCardGrid v-else :key="folderId" :items="items" :letters="letters" :total="total">
            <template #card="{ item }">
                <ArtistCard v-if="item" :artist="item" />
            </template>
        </VirtualCardGrid>
    </div>
</template>

<style scoped>
.artist-grid-view {
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
