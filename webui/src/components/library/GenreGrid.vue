<script setup lang="ts">
import { computed } from 'vue'
import VirtualCardGrid from '@/components/library/VirtualCardGrid.vue'
import GenreCard from '@/components/library/GenreCard.vue'
import { useGenres } from '@/composables/useSubsonicQueries'

const { data: genres, isLoading, error } = useGenres()

// VirtualCardGrid requires items with an `id`; genres are keyed by name.
const items = computed(() => (genres.value ?? []).map((g) => ({ ...g, id: g.value })))
</script>

<template>
    <div class="genre-grid-view">
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>
        <div v-else-if="error" class="empty-state">
            <i class="pi pi-exclamation-triangle" style="font-size: 3rem"></i>
            <p>Could not load genres</p>
        </div>
        <div v-else-if="!genres?.length" class="empty-state">
            <i class="pi pi-tags" style="font-size: 3rem"></i>
            <p>No genres found</p>
        </div>
        <VirtualCardGrid
            v-else
            :items="items"
            :letters="[]"
            :total="items.length"
            :showRail="false"
        >
            <template #card="{ item }">
                <GenreCard v-if="item" :genre="item" />
            </template>
        </VirtualCardGrid>
    </div>
</template>

<style scoped>
.genre-grid-view {
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
