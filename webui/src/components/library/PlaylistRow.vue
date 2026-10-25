<script setup lang="ts">
import { computed } from 'vue'
import type { Playlist } from '@/types/subsonic'
import ListCard, { type ListCardMetaPart } from '@/components/library/ListCard.vue'
import { subsonicClient } from '@/lib/api/subsonic'
import { useTogglePlaylistStar } from '@/composables/useSubsonicQueries'
import { useIsPlaylistOwner } from '@/composables/usePlaylistOwnership'
import { formatTotalDuration } from '@/utils/formatDuration'
import { formatRelativeDate } from '@/utils/formatRelativeDate'
import { versionedCoverUrl } from '@/composables/useCoverVersion'

/**
 * One playlist in the list layout — the playlist counterpart to `AlbumRow`, and
 * the single list rendering for playlists in the app (`PlaylistListView` and the
 * Discovery feed, which interleaves the two). Both are `ListCard`s, so they share
 * one size and one shape in a mixed feed. There is deliberately no play button:
 * the card opens the playlist, and play lives on the grid cards and the hero.
 */
const props = defineProps<{ playlist?: Playlist }>()

const toggleStar = useTogglePlaylistStar()
const isOwner = useIsPlaylistOwner(() => props.playlist?.owner)

const coverUrl = computed(() => {
    const art = props.playlist?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    const base = subsonicClient.getCoverArtUrl(art, 200)
    return versionedCoverUrl(base, art)
})

const meta = computed(() => {
    const playlist = props.playlist
    if (!playlist) return []
    const n = playlist.songCount
    return [`${n} ${n === 1 ? 'song' : 'songs'}`, formatTotalDuration(playlist.duration)]
})

// Whose it is first — named only when it is someone else's, beside the lock that
// marks it view-only — then when it last changed and was last played.
const details = computed(() => {
    const playlist = props.playlist
    if (!playlist) return []
    const parts: Array<string | ListCardMetaPart> = []
    if (!isOwner.value) {
        const owner = playlist.owner ?? 'another user'
        parts.push({
            text: `by ${owner}`,
            icon: 'ms-lock',
            iconLabel: `Shared by ${owner} — view only`
        })
    }
    const changed = formatRelativeDate(playlist.changed)
    if (changed) parts.push(`updated ${changed}`)
    const played = formatRelativeDate(playlist.played)
    if (played) parts.push(`played ${played}`)
    return parts
})

const isStarred = computed(() => !!props.playlist?.starred)

const onStar = (): void => {
    if (!props.playlist) return
    toggleStar.mutate({ id: props.playlist.id, starred: isStarred.value })
}
</script>

<template>
    <ListCard v-if="!playlist" class="playlist-row" />
    <ListCard
        v-else
        class="playlist-row"
        :to="{ name: 'playlist-detail', params: { id: playlist.id } }"
        :title="playlist.name"
        :subtitle="playlist.comment?.trim()"
        :meta="meta"
        :details="details"
        :cover-url="coverUrl"
        placeholder-icon="ms-queue-music"
        starrable
        :starred="isStarred"
        @star="onStar"
    />
</template>
