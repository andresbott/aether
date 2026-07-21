<script setup lang="ts">
import { computed } from 'vue'
import type { Genre } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'

const props = defineProps<{ genre?: Genre }>()

const coverUrl = computed(() => {
    const art = props.genre?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 200)
})

const subtitle = computed(() => {
    if (!props.genre) return ''
    const parts: string[] = []
    const { albumCount, songCount } = props.genre
    if (albumCount > 0) parts.push(`${albumCount} ${albumCount === 1 ? 'album' : 'albums'}`)
    if (songCount > 0) parts.push(`${songCount} ${songCount === 1 ? 'song' : 'songs'}`)
    return parts.join(' • ')
})
</script>

<template>
    <div v-if="!genre" class="genre-card placeholder" aria-hidden="true">
        <div class="card-cover"></div>
        <div class="card-info">
            <div class="card-title"></div>
            <div class="card-subtitle"></div>
        </div>
    </div>
    <router-link
        v-else
        :to="{ name: 'genre-detail', params: { name: genre.value } }"
        class="genre-card"
    >
        <div class="card-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="genre.value" draggable="false" />
            <div v-else class="cover-placeholder">
                <i class="pi pi-tags" style="font-size: 2rem"></i>
            </div>
        </div>
        <div class="card-info">
            <div class="card-title">{{ genre.value }}</div>
            <div class="card-subtitle">
                <template v-if="subtitle">{{ subtitle }}</template>
                <template v-else>&nbsp;</template>
            </div>
        </div>
    </router-link>
</template>

<style scoped>
.genre-card {
    display: flex;
    flex-direction: column;
    text-decoration: none;
    color: inherit;
    /* Transparent border reserved so the hover border never shifts layout. */
    border: 1px solid transparent;
    border-radius: 10px;
    padding: 0.5rem;
    transition: border-color 0.2s, background 0.2s, box-shadow 0.2s;
    cursor: pointer;
}

.genre-card:hover:not(.placeholder) {
    border-color: var(--app-accent);
    background: var(--app-accent-soft);
    box-shadow: 0 6px 18px rgba(0, 0, 0, 0.12);
}

.card-cover {
    width: 100%;
    aspect-ratio: 1;
    border-radius: 8px;
    overflow: hidden;
    background: var(--app-bg-subtle);
}

.card-cover img {
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

.card-info {
    padding: 0.5rem 0.15rem 0.1rem;
}

.card-title {
    font-size: 0.9rem;
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.card-subtitle {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
</style>
