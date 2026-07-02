<script setup lang="ts">
import { toRef } from 'vue'
import ArtistCard from '@/components/library/ArtistCard.vue'
import { useArtistTable } from '@/composables/useArtistTable'

const props = defineProps<{ folderId?: number }>()
const { items, isLoading } = useArtistTable(toRef(props, 'folderId'))
</script>

<template>
    <div class="grid-scroll">
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>
        <div v-else-if="items.length > 0" class="artist-grid">
            <ArtistCard v-for="artist in items" :key="artist.id" :artist="artist" />
        </div>
        <div v-else class="empty-state">
            <i class="pi pi-users" style="font-size: 3rem"></i>
            <p>No artists found</p>
        </div>
    </div>
</template>

<style scoped>
.grid-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
}

.artist-grid {
    max-width: 1400px;
    margin: 0 auto;
    padding: 1rem;
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
