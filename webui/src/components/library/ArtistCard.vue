<script setup lang="ts">
import { computed } from 'vue'
import type { Artist } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'

const props = defineProps<{
    artist: Artist
}>()

const coverUrl = computed(() => {
    if (!props.artist.coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(props.artist.coverArt, 180)
})
</script>

<template>
    <router-link :to="{ name: 'artist', params: { id: artist.id } }" class="artist-card">
        <div class="card-avatar">
            <img v-if="coverUrl" :src="coverUrl" :alt="artist.name" />
            <div v-else class="avatar-placeholder">
                <i class="pi pi-user" style="font-size: 2rem"></i>
            </div>
        </div>
        <div class="card-name">{{ artist.name }}</div>
        <div v-if="artist.albumCount" class="card-count">
            <i class="pi pi-disc"></i> {{ artist.albumCount }}
        </div>
    </router-link>
</template>

<style scoped>
.artist-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-decoration: none;
    color: inherit;
    cursor: pointer;
    transition: transform 0.2s;
}

.artist-card:hover {
    transform: translateY(-2px);
}

.card-avatar {
    width: 180px;
    height: 180px;
    border-radius: 50%;
    overflow: hidden;
}

.card-avatar img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.avatar-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: rgba(255, 255, 255, 0.8);
}

.card-name {
    margin-top: 0.5rem;
    font-size: 0.9rem;
    font-weight: 600;
    text-align: center;
}

.card-count {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    display: flex;
    align-items: center;
    gap: 0.25rem;
}
</style>
