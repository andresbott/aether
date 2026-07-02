<script setup lang="ts">
import { computed } from 'vue'
import type { Artist } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'

const props = defineProps<{ artist?: Artist }>()

const coverUrl = computed(() => {
    const art = props.artist?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 80)
})
</script>

<template>
    <div v-if="!artist" class="artist-row placeholder">
        <div class="col-avatar"></div>
        <div class="col-name"></div>
    </div>
    <router-link
        v-else
        :to="{ name: 'artist', params: { id: artist.id } }"
        class="artist-row"
    >
        <div class="col-avatar">
            <img v-if="coverUrl" :src="coverUrl" :alt="artist.name" />
            <div v-else class="avatar-placeholder"><i class="pi pi-user"></i></div>
        </div>
        <div class="col-name">{{ artist.name }}</div>
        <div v-if="artist.albumCount !== undefined" class="col-count">
            {{ artist.albumCount }} {{ artist.albumCount === 1 ? 'album' : 'albums' }}
        </div>
    </router-link>
</template>

<style scoped>
.artist-row {
    display: grid;
    grid-template-columns: 48px 1fr 7rem;
    align-items: center;
    gap: 1rem;
    height: 56px;
    padding: 0 0.5rem;
    text-decoration: none;
    color: inherit;
    border-bottom: 1px solid var(--p-content-border-color);
}

.artist-row:hover:not(.placeholder) {
    background: var(--p-content-hover-background);
}

.col-avatar {
    width: 40px;
    height: 40px;
    border-radius: 50%;
    overflow: hidden;
}

.col-avatar img {
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

.col-name {
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.col-count {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
    text-align: right;
}
</style>
