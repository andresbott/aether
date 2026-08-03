<script setup lang="ts">
import { computed, toRef } from 'vue'
import VirtualCardGrid from '@/components/library/VirtualCardGrid.vue'
import ArtistCard from '@/components/library/ArtistCard.vue'
import { useArtistSource } from '@/composables/useLibrarySource'

const props = defineProps<{ folderId?: number; favoritesOnly?: boolean }>()

const { total, letters, items, isLoading, error } = useArtistSource(
    toRef(props, 'folderId'),
    computed(() => props.favoritesOnly === true)
)
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
            <i :class="favoritesOnly ? 'pi pi-heart' : 'pi pi-users'" style="font-size: 3rem"></i>
            <p v-if="favoritesOnly">No favorite artists yet</p>
            <p v-else>No artists found</p>
        </div>
        <!-- lazyLoad not handled: both artist sources return a fully-materialized
             Artist[] with no paging. Keyed on the source as well as the folder, so
             switching to favorites remeasures instead of keeping the old range. -->
        <VirtualCardGrid
            v-else
            :key="`${folderId ?? 'all'}-${favoritesOnly ? 'fav' : 'all'}`"
            :items="items"
            :letters="letters"
            :total="total"
        >
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
