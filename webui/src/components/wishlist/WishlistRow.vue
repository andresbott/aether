<script setup lang="ts">
import { computed } from 'vue'
import ListCard, { type ListCardMetaPart } from '@/components/library/ListCard.vue'
import { subsonicClient } from '@/lib/api/subsonic'
import { isManualEntry, wishlistDetails, wishlistKind, wishlistLibraryId, wishlistSubtitle } from '@/lib/wishlist'
import type { WishlistItem } from '@/types/wishlist'

/**
 * One wishlist entry in the list: a `ListCard` opening the entry's page. A found
 * entry wears the library's own cover and an "In library" check; one still
 * missing shows its kind's placeholder — there is no cover for music the server
 * does not have. An entry added by hand shows its description where a
 * MusicBrainz pick shows its artist.
 */
const props = defineProps<{ item?: WishlistItem }>()

// The library cover of what the entry matched: the artist, the album, or the
// album a song is on.
const coverUrl = computed(() => {
    const id = props.item ? wishlistLibraryId(props.item) : null
    return id && subsonicClient.isConfigured() ? subsonicClient.getCoverArtUrl(id, 200) : null
})

const meta = computed<ListCardMetaPart[]>(() =>
    props.item?.in_library
        ? [{ text: 'In library', icon: 'ms-check-circle', iconLabel: 'Found in your library' }]
        : [{ text: 'Wanted' }]
)
const details = computed(() => (props.item ? wishlistDetails(props.item) : []))
const placeholderIcon = computed(() => (props.item ? wishlistKind(props.item.kind).icon : 'ms-music-note'))
</script>

<template>
    <ListCard v-if="!item" class="wishlist-row" />
    <ListCard
        v-else
        class="wishlist-row"
        :class="{ found: item.in_library }"
        :to="{ name: 'wishlist-item', params: { id: item.id } }"
        :title="item.title"
        :subtitle="isManualEntry(item) ? item.note : wishlistSubtitle(item)"
        :meta="meta"
        :details="details"
        :cover-url="coverUrl"
        :placeholder-icon="placeholderIcon"
        :round="item.kind === 'artist'"
    />
</template>

<style scoped>
.wishlist-row.found :deep(.meta-icon) {
    color: var(--app-accent);
}
</style>
