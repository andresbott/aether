<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import DataTable from 'primevue/datatable'
import Column from 'primevue/column'
import Tag from 'primevue/tag'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import { usePodcastChannel } from '@/composables/useSubsonicQueries'
import { subsonicClient } from '@/lib/api/subsonic'

const props = defineProps<{ id: string }>()
const router = useRouter()

const { data: channel, isLoading, error } = usePodcastChannel(props.id)

const coverUrl = computed(() => {
    if (!channel.value?.coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(channel.value.coverArt, 250)
})

const summary = computed(() => {
    const n = channel.value?.episode?.length ?? 0
    if (n === 0) return ''
    return `${n} ${n === 1 ? 'episode' : 'episodes'}`
})

const formatDate = (dateStr?: string): string => {
    if (!dateStr) return ''
    return new Date(dateStr).toLocaleDateString()
}

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    return `${mins} min`
}

const statusSeverity = (status: string): 'success' | 'info' | 'danger' | 'secondary' => {
    switch (status) {
        case 'completed': return 'success'
        case 'downloading': return 'info'
        case 'error': return 'danger'
        default: return 'secondary'
    }
}
</script>

<template>
    <div class="podcast-channel-view">
        <div class="back-row">
            <Button icon="pi pi-arrow-left" text rounded @click="router.back()" />
        </div>

        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="error" class="error">
            <p>{{ error.message }}</p>
        </div>

        <ContentScaffold v-else-if="channel" :title="channel.title" :summary="summary">
            <div class="channel-scroll">
                <div class="channel-body">
                    <div class="channel-hero">
                        <div class="channel-cover">
                            <img v-if="coverUrl" :src="coverUrl" :alt="channel.title" />
                            <div v-else class="cover-placeholder">
                                <i class="pi pi-microphone" style="font-size: 3rem"></i>
                            </div>
                        </div>
                        <div class="channel-info">
                            <p v-if="channel.description" class="description">{{ channel.description }}</p>
                        </div>
                    </div>

                    <h2>Episodes</h2>
                    <DataTable
                        v-if="channel.episode && channel.episode.length > 0"
                        :value="channel.episode"
                        stripedRows
                    >
                        <Column field="title" header="Title" />
                        <Column header="Published" style="width: 120px">
                            <template #body="{ data }">{{ formatDate(data.publishDate) }}</template>
                        </Column>
                        <Column header="Duration" style="width: 100px">
                            <template #body="{ data }">{{ formatDuration(data.duration) }}</template>
                        </Column>
                        <Column header="Status" style="width: 110px">
                            <template #body="{ data }">
                                <Tag :value="data.status" :severity="statusSeverity(data.status)" />
                            </template>
                        </Column>
                    </DataTable>
                    <div v-else class="empty-episodes">
                        <p>No episodes available</p>
                    </div>
                </div>
            </div>
        </ContentScaffold>
    </div>
</template>

<style scoped>
.podcast-channel-view { height: 100%; display: flex; flex-direction: column; min-height: 0; }
.back-row { flex-shrink: 0; padding: 0.5rem 2rem 0; }
.loading, .error { display: flex; flex-direction: column; align-items: center; padding: 3rem; gap: 1rem; color: var(--app-text-secondary); }
.error { color: #ef4444; }
.channel-scroll { height: 100%; overflow-y: auto; scrollbar-gutter: stable; }
.channel-body { max-width: var(--app-content-max-width); margin: 0 auto; padding: 1rem; }
.channel-hero { display: flex; gap: 2rem; margin-bottom: 2rem; }
.channel-cover { width: 250px; height: 250px; flex-shrink: 0; border-radius: 8px; overflow: hidden; }
.channel-cover img { width: 100%; height: 100%; object-fit: cover; }
.cover-placeholder { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: rgba(255, 255, 255, 0.8); }
.channel-info { flex: 1; display: flex; flex-direction: column; gap: 0.5rem; }
.description { color: var(--app-text-secondary); margin: 0; line-height: 1.6; }
h2 { font-size: 1.5rem; font-weight: 600; margin-bottom: 1rem; }
.empty-episodes { text-align: center; padding: 2rem; color: var(--app-text-secondary); }
</style>
