<script setup lang="ts">
import { computed } from 'vue'
import Button from 'primevue/button'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import { usePodcasts } from '@/composables/useSubsonicQueries'
import { subsonicClient } from '@/lib/api/subsonic'

const { data: channels, isLoading } = usePodcasts(true)

const summary = computed(() => {
    const count = channels.value?.length ?? 0
    if (count === 0) return ''
    return `${count} ${count === 1 ? 'podcast' : 'podcasts'}`
})

const getCoverUrl = (coverArt?: string): string | null => {
    if (!coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(coverArt, 200)
}
</script>

<template>
    <ContentScaffold title="Podcasts" :summary="summary">
        <template #actions>
            <Button label="Subscribe" icon="pi pi-plus" disabled />
        </template>

        <div class="podcasts-scroll">
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
    </ContentScaffold>
</template>

<style scoped>
.podcasts-scroll { height: 100%; overflow-y: auto; scrollbar-gutter: stable; }
.loading { display: flex; justify-content: center; padding: 3rem; color: var(--app-text-secondary); }
.channel-grid { max-width: var(--app-content-max-width); margin: 0 auto; padding: 1rem; display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 2rem; }
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
