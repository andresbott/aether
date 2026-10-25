<script setup lang="ts">
import { computed } from 'vue'
import type { Artist } from '@/types/subsonic'
import ListCard from '@/components/library/ListCard.vue'
import { subsonicClient } from '@/lib/api/subsonic'
import { versionedCoverUrl } from '@/composables/useCoverVersion'
import { useToggleStar } from '@/composables/useSubsonicQueries'

const props = defineProps<{ artist?: Artist }>()

const toggleStar = useToggleStar()

const coverUrl = computed(() => {
    const art = props.artist?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    // Same cache-bust as the detail view, so editing a cover updates the list too.
    return versionedCoverUrl(subsonicClient.getCoverArtUrl(art, 200), art)
})

const meta = computed(() => {
    const count = props.artist?.albumCount
    return count === undefined ? [] : [`${count} ${count === 1 ? 'album' : 'albums'}`]
})

const isStarred = computed(() => !!props.artist?.starred)

// Artist cards are query-backed, so the refetch `useToggleStar` triggers is what
// updates the heart.
const onStar = (): void => {
    if (!props.artist) return
    toggleStar.mutate({ id: props.artist.id, starred: isStarred.value })
}
</script>

<template>
    <ListCard v-if="!artist" class="artist-row" round />
    <ListCard
        v-else
        class="artist-row"
        :to="{ name: 'artist', params: { id: artist.id } }"
        :title="artist.name"
        :meta="meta"
        :cover-url="coverUrl"
        placeholder-icon="ms-person"
        round
        starrable
        :starred="isStarred"
        @star="onStar"
    />
</template>
