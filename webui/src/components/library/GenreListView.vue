<script setup lang="ts">
import VirtualScroller from 'primevue/virtualscroller'
import GenreRow from '@/components/library/GenreRow.vue'
import { useGenres } from '@/composables/useSubsonicQueries'
import { useListCardMetrics } from '@/composables/useListCardMetrics'

const { data: genres, isLoading, error } = useGenres()
const { itemSize } = useListCardMetrics()
</script>

<template>
    <div class="genre-list-view">
        <div v-if="isLoading" class="loading">
            <i class="icon-spin ms-progress-activity" style="font-size: 2rem"></i>
        </div>
        <div v-else-if="error" class="empty-state">
            <i class="ms-warning" style="font-size: 3rem"></i>
            <p>Could not load genres</p>
        </div>
        <div v-else-if="!genres?.length" class="empty-state">
            <i class="ms-genres" style="font-size: 3rem"></i>
            <p>No genres found</p>
        </div>
        <div v-else class="list-body">
            <VirtualScroller :items="genres" :itemSize="itemSize" class="list-scroller">
                <template #item="{ item }">
                    <GenreRow :genre="item" />
                </template>
            </VirtualScroller>
        </div>
    </div>
</template>

<style scoped>
.genre-list-view {
    height: 100%;
    min-height: 0;
}

.list-body {
    position: relative;
    height: 100%;
}

.list-scroller {
    height: 100%;
    width: 100%;
    scrollbar-gutter: stable;
}

/* Recipe C without a rail: the shared gutter on both sides; the top padding is
   the gap under the header (see AlbumListView). */
.list-body :deep(.p-virtualscroller-content) {
    box-sizing: border-box;
    padding-top: 1rem;
    padding-inline: var(--app-content-gutter);
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
