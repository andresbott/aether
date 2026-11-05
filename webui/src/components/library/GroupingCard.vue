<script setup lang="ts">
import { computed } from 'vue'
import type { Grouping } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { versionedCoverUrl } from '@/composables/useCoverVersion'

const props = defineProps<{ grouping: Grouping }>()

const coverUrl = computed(() => {
    if (!props.grouping.coverArt || !subsonicClient.isConfigured()) return null
    return versionedCoverUrl(
        subsonicClient.getCoverArtUrl(props.grouping.coverArt, 200),
        props.grouping.coverArt
    )
})

const subtitle = computed(() => {
    const n = props.grouping.albumCount
    return n === undefined ? '' : `${n} ${n === 1 ? 'album' : 'albums'}`
})
</script>

<template>
    <router-link :to="{ name: 'grouping', params: { id: grouping.id } }" class="grouping-card">
        <div class="card-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="grouping.name" draggable="false" />
            <div v-else class="cover-placeholder">
                <i class="ms-collections-bookmark" style="font-size: 2rem"></i>
            </div>
        </div>
        <div class="card-info">
            <div class="card-title">{{ grouping.name }}</div>
            <div class="card-subtitle">{{ subtitle || ' ' }}</div>
        </div>
    </router-link>
</template>

<style scoped>
.grouping-card {
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

.grouping-card:hover {
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
    min-width: 0;
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
