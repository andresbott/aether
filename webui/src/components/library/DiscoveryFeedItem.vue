<script setup lang="ts">
import { computed } from 'vue'
import AlbumCard from '@/components/library/AlbumCard.vue'
import AlbumRow from '@/components/library/AlbumRow.vue'
import PlaylistCard from '@/components/library/PlaylistCard.vue'
import type { DiscoveryFeedEntry, DiscoveryReason } from '@/types/subsonic'

const props = defineProps<{ entry: DiscoveryFeedEntry; layout: 'grid' | 'list' }>()

// With the five themed shelves gone, the badge is the only thing left telling
// the user why an item surfaced. The wrapper owns it so no discovery concept
// leaks into cards shared with /library and /playlists.
const REASON_LABELS: Record<DiscoveryReason, string> = {
    favorite: 'Favorite',
    recentlyAdded: 'Recently added',
    mostPlayed: 'Most played',
    recentlyPlayed: 'Recently played',
    genreMatch: 'Your genres',
    rediscover: 'Rediscover'
}

const reasonLabel = computed(() => REASON_LABELS[props.entry.reason])
</script>

<template>
    <div class="discovery-feed-item" :class="layout">
        <span class="discovery-reason-badge">{{ reasonLabel }}</span>
        <template v-if="entry.type === 'album'">
            <AlbumCard v-if="layout === 'grid'" :album="entry.album" />
            <AlbumRow v-else :album="entry.album" />
        </template>
        <PlaylistCard v-else :playlist="entry.playlist" />
    </div>
</template>

<style scoped>
.discovery-feed-item {
    position: relative;
}

.discovery-reason-badge {
    position: absolute;
    top: 0.4rem;
    left: 0.4rem;
    z-index: 1;
    padding: 0.15rem 0.45rem;
    border-radius: 999px;
    background: color-mix(in srgb, var(--app-surface) 80%, transparent);
    color: var(--app-text-secondary);
    font-size: 0.7rem;
    font-weight: 600;
    line-height: 1.4;
    pointer-events: none;
}

/* In list layout the row is full-width and short, so the badge sits inline at
   the right instead of over the cover. */
.discovery-feed-item.list {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}

.discovery-feed-item.list > :not(.discovery-reason-badge) {
    flex: 1;
    min-width: 0;
}

.discovery-feed-item.list .discovery-reason-badge {
    position: static;
    order: 2;
    flex-shrink: 0;
}
</style>
