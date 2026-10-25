<script setup lang="ts">
import { computed } from 'vue'
import type { Genre } from '@/types/subsonic'
import ListCard from '@/components/library/ListCard.vue'
import { subsonicClient } from '@/lib/api/subsonic'
import { versionedCoverUrl } from '@/composables/useCoverVersion'

const props = defineProps<{ genre?: Genre }>()

const coverUrl = computed(() => {
    const art = props.genre?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    const base = subsonicClient.getCoverArtUrl(art, 200)
    return versionedCoverUrl(base, art)
})

// A count of zero says nothing about a genre, so it is left out.
const meta = computed(() => {
    const genre = props.genre
    if (!genre) return []
    const parts: string[] = []
    if (genre.albumCount)
        parts.push(`${genre.albumCount} ${genre.albumCount === 1 ? 'album' : 'albums'}`)
    if (genre.songCount)
        parts.push(`${genre.songCount} ${genre.songCount === 1 ? 'song' : 'songs'}`)
    return parts
})
</script>

<template>
    <ListCard v-if="!genre" class="genre-row" />
    <ListCard
        v-else
        class="genre-row"
        :to="{ name: 'genre-detail', params: { name: genre.value } }"
        :title="genre.value"
        :meta="meta"
        :cover-url="coverUrl"
        placeholder-icon="ms-genres"
    />
</template>
