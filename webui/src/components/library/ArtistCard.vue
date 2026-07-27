<script setup lang="ts">
import { computed } from 'vue'
import type { Artist } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { versionedCoverUrl } from '@/composables/useCoverVersion'

const props = defineProps<{
    artist: Artist
}>()

const coverUrl = computed(() => {
    if (!props.artist.coverArt || !subsonicClient.isConfigured()) return null
    const base = subsonicClient.getCoverArtUrl(props.artist.coverArt, 200)
    // Same cache-bust as the detail view, so editing a cover updates the grid too.
    return versionedCoverUrl(base, props.artist.coverArt)
})
</script>

<template>
    <router-link :to="{ name: 'artist', params: { id: artist.id } }" class="artist-card">
        <div class="card-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="artist.name" />
            <div v-else class="cover-placeholder">
                <i class="pi pi-user" style="font-size: 2rem"></i>
            </div>
        </div>
        <div class="card-info">
            <div class="card-title">{{ artist.name }}</div>
            <div class="card-subtitle">
                <template v-if="artist.albumCount != null">{{ artist.albumCount }} {{ artist.albumCount === 1 ? 'album' : 'albums' }}</template>
                <template v-else>&nbsp;</template>
            </div>
        </div>
    </router-link>
</template>

<style scoped>
.artist-card {
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

/* Border + accent tint wrap the whole card (cover + text) on hover. */
.artist-card:hover {
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
