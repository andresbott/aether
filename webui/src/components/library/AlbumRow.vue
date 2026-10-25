<script setup lang="ts">
import { computed } from 'vue'
import type { Album } from '@/types/subsonic'
import ListCard from '@/components/library/ListCard.vue'
import { subsonicClient } from '@/lib/api/subsonic'
import { useAlbumDrag } from '@/composables/useAlbumDrag'
import { useToggleStar } from '@/composables/useSubsonicQueries'
import { versionedCoverUrl } from '@/composables/useCoverVersion'
import { formatTotalDuration } from '@/utils/formatDuration'
import { releaseTypeTag } from '@/lib/releaseTypes'

const props = defineProps<{ album?: Album }>()
const albumDrag = useAlbumDrag()
const toggleStar = useToggleStar()

// The grid card's size, so switching between the two layouts reuses one image.
const coverUrl = computed(() => {
    const art = props.album?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    const base = subsonicClient.getCoverArtUrl(art, 200)
    return versionedCoverUrl(base, art)
})

// The hero's meta row: the year first, so the years line up down the list, then
// the album's size.
const meta = computed(() => {
    const album = props.album
    if (!album) return []
    const parts: string[] = []
    // The server sends year 0 for an album without one.
    if (album.year) parts.push(String(album.year))
    if (album.songCount !== undefined) {
        parts.push(`${album.songCount} ${album.songCount === 1 ? 'song' : 'songs'}`)
    }
    parts.push(formatTotalDuration(album.duration))
    return parts
})

// What the album is: the release type the hero's eyebrow names (here only when it
// is not a plain album) and the genre.
const details = computed(() => {
    const album = props.album
    if (!album) return []
    return [releaseTypeTag(album.releaseTypes, album.isCompilation), album.genre ?? '']
})

const isStarred = computed(() => !!props.album?.starred)

// Album cards are query-backed, so the refetch `useToggleStar` triggers is what
// updates the heart; no optimistic flip is needed (unlike the track rows, whose
// queue is not).
const onStar = (): void => {
    if (!props.album) return
    toggleStar.mutate({ id: props.album.id, starred: isStarred.value })
}

function onDragStart(event: DragEvent): void {
    if (props.album) albumDrag.start(event, props.album, coverUrl.value)
}
</script>

<template>
    <ListCard v-if="!album" class="album-row" />
    <ListCard
        v-else
        class="album-row"
        :to="{ name: 'album', params: { id: album.id } }"
        :title="album.name"
        :subtitle="album.artist"
        :meta="meta"
        :details="details"
        :cover-url="coverUrl"
        placeholder-icon="ms-music-note"
        starrable
        :starred="isStarred"
        draggable="true"
        @star="onStar"
        @dragstart="onDragStart"
        @dragend="albumDrag.end"
    />
</template>
