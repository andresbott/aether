<script setup lang="ts">
import AlbumCard from '@/components/library/AlbumCard.vue'
import AlbumRow from '@/components/library/AlbumRow.vue'
import PlaylistCard from '@/components/library/PlaylistCard.vue'
import PlaylistRow from '@/components/library/PlaylistRow.vue'
import type { DiscoveryFeedEntry } from '@/types/subsonic'

// Dispatches one feed entry to the right card or row. `entry.reason` is deliberately
// NOT rendered: the server still reports why an item surfaced (it is part of the
// `discovery` extension and other clients may use it), but showing it on every item
// added noise rather than information — on a lightly-played library nearly every
// item reads the same. The feed's ordering already expresses the ranking.
defineProps<{ entry: DiscoveryFeedEntry; layout: 'grid' | 'list' }>()
</script>

<template>
    <div class="discovery-feed-item" :class="layout">
        <!-- Both types honour the layout: cards in grid, rows in list. A playlist
             rendered as a card inside a list would tower over the album rows beside
             it, which is why PlaylistRow exists. -->
        <template v-if="entry.type === 'album'">
            <AlbumCard v-if="layout === 'grid'" :album="entry.album" />
            <AlbumRow v-else :album="entry.album" />
        </template>
        <template v-else>
            <PlaylistCard v-if="layout === 'grid'" :playlist="entry.playlist" />
            <PlaylistRow v-else :playlist="entry.playlist" />
        </template>
    </div>
</template>

<style scoped>
.discovery-feed-item {
    position: relative;
    /* This wrapper sits between CardGrid's cell and the card, so it has to PASS THE
       SHRINK PERMISSION THROUGH. Without these the wrapper adopts its content's
       intrinsic width — a long title then pushes the card wider than its 1fr cell
       and scales the square cover with it, so one item renders visibly bigger than
       its neighbours. The cards already truncate with text-overflow: ellipsis, but
       that only engages once the box is actually constrained. */
    min-width: 0;
    max-width: 100%;
}

/* And the card itself must not exceed the wrapper. A card is a flex CONTAINER whose
   own min-width resolves to `auto` (min-content) as a layout child, so without this
   it sizes to its longest unbreakable text and overflows the 1fr cell — which is
   what made one item render bigger than its neighbours. The cards are shared with
   /library and must not be edited, so the constraint is applied from here. */
.discovery-feed-item.grid > * {
    min-width: 0;
    max-width: 100%;
}

/* In list layout the row fills the feed's width. */
.discovery-feed-item.list > * {
    min-width: 0;
}
</style>
