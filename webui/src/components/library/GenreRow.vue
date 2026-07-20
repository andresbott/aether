<script setup lang="ts">
import { computed } from 'vue'
import type { Genre } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'

const props = defineProps<{ genre?: Genre }>()

const coverUrl = computed(() => {
    const art = props.genre?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 80)
})
</script>

<template>
    <div v-if="!genre" class="genre-row placeholder">
        <div class="col-cover"></div>
        <div class="col-name"></div>
    </div>
    <router-link
        v-else
        :to="{ name: 'genre-detail', params: { name: genre.value } }"
        class="genre-row"
    >
        <div class="col-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="genre.value" draggable="false" />
            <div v-else class="cover-placeholder"><i class="pi pi-tags"></i></div>
        </div>
        <div class="col-name">{{ genre.value }}</div>
        <div class="col-albums">{{ genre.albumCount || '' }}</div>
        <div class="col-songs">{{ genre.songCount || '' }}</div>
    </router-link>
</template>

<style scoped>
.genre-row {
    display: grid;
    grid-template-columns: 48px 1fr 4rem 4rem;
    align-items: center;
    gap: 1rem;
    height: 56px;
    padding: 0 0.5rem;
    text-decoration: none;
    color: inherit;
    border-bottom: 1px solid var(--p-content-border-color);
    cursor: pointer;
    transition: background-color 0.15s;
}

.genre-row:hover:not(.placeholder) {
    background-color: var(--app-hover);
}

.col-cover {
    width: 40px;
    height: 40px;
    border-radius: 4px;
    overflow: hidden;
}

.col-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.cover-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: rgba(255, 255, 255, 0.8);
}

.col-name {
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.col-albums,
.col-songs {
    text-align: right;
    color: var(--app-text-secondary);
    font-size: 0.85rem;
    font-variant-numeric: tabular-nums;
}
</style>
