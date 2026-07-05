<script setup lang="ts">
import Button from 'primevue/button'
import { usePodcasts } from '@/composables/useSubsonicQueries'
import { subsonicClient } from '@/lib/api/subsonic'

const { data: channels, isLoading } = usePodcasts(true)

const getCoverUrl = (coverArt?: string): string | null => {
    if (!coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(coverArt, 200)
}
</script>

<template>
    <div class="podcasts-view">
        <div class="view-header">
            <h1>Podcasts</h1>
            <Button label="Subscribe" icon="pi pi-plus" disabled />
        </div>

        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="channels && channels.length > 0" class="channel-grid">
            <router-link
                v-for="ch in channels"
                :key="ch.id"
                :to="{ name: 'podcast-channel', params: { id: ch.id } }"
                class="channel-card"
            >
                <div class="channel-cover">
                    <img v-if="getCoverUrl(ch.coverArt)" :src="getCoverUrl(ch.coverArt)!" alt="" />
                    <div v-else class="cover-placeholder">
                        <i class="pi pi-microphone" style="font-size: 2rem"></i>
                    </div>
                </div>
                <div class="channel-info">
                    <div class="channel-title">{{ ch.title }}</div>
                    <div v-if="ch.episode" class="channel-meta">
                        {{ ch.episode.length }} episodes
                    </div>
                </div>
            </router-link>
        </div>

        <div v-else class="empty-state">
            <i class="pi pi-microphone" style="font-size: 3rem"></i>
            <p>No podcasts</p>
        </div>
    </div>
</template>

<style scoped>
.podcasts-view { max-width: var(--app-content-max-width); margin: 0 auto; }
.view-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 2rem; }
.view-header h1 { font-size: 2rem; font-weight: 700; margin: 0; }
.loading { display: flex; justify-content: center; padding: 3rem; color: var(--app-text-secondary); }
.channel-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 2rem; }
.channel-card { display: flex; flex-direction: column; text-decoration: none; color: inherit; transition: transform 0.2s; }
.channel-card:hover { transform: translateY(-2px); }
.channel-cover { width: 100%; aspect-ratio: 1; border-radius: 8px; overflow: hidden; }
.channel-cover img { width: 100%; height: 100%; object-fit: cover; }
.cover-placeholder { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: rgba(255, 255, 255, 0.8); }
.channel-info { padding: 0.5rem 0.25rem; }
.channel-title { font-size: 0.9rem; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.channel-meta { font-size: 0.8rem; color: var(--app-text-secondary); }
.empty-state { display: flex; flex-direction: column; align-items: center; padding: 4rem; gap: 1rem; color: var(--app-text-secondary); }
</style>
