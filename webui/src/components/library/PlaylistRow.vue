<script setup lang="ts">
import { computed } from 'vue'
import type { Playlist } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { useTogglePlaylistStar } from '@/composables/useSubsonicQueries'
import { formatDuration } from '@/utils/formatDuration'

/**
 * One playlist as a list row — the playlist counterpart to `AlbumRow`, and the
 * single row rendering for playlists in the app.
 *
 * It deliberately mirrors AlbumRow's grid template (48px 2fr 1.5fr 2rem 4rem 5rem) so a
 * feed that interleaves albums and playlists lines its columns up. `PlaylistListView`
 * supplies its own header for those columns; the Discovery feed has no header, which
 * is fine because each cell still reads as label + value.
 */
const props = defineProps<{ playlist?: Playlist }>()

const toggleStar = useTogglePlaylistStar()

const coverUrl = computed(() => {
    const art = props.playlist?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 80)
})

const isStarred = computed(() => !!props.playlist?.starred)

// The row is a router-link, so the toggle must swallow the click or it would
// navigate to the playlist detail view instead.
const onStar = (event: Event): void => {
    event.preventDefault()
    event.stopPropagation()
    if (!props.playlist) return
    toggleStar.mutate({ id: props.playlist.id, starred: isStarred.value })
}
</script>

<template>
    <div v-if="!playlist" class="playlist-row placeholder">
        <div class="col-cover"></div>
        <div class="col-title"></div>
    </div>
    <router-link
        v-else
        :to="{ name: 'playlist-detail', params: { id: playlist.id } }"
        class="playlist-row"
    >
        <div class="col-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="playlist.name" draggable="false" />
            <div v-else class="cover-placeholder"><i class="pi pi-list"></i></div>
        </div>
        <div class="col-title">{{ playlist.name }}</div>
        <!-- Empty, standing in for AlbumRow's artist column (a playlist has no
             artist) so the two row types keep identical column edges in a mixed
             feed. There is deliberately no play button here: AlbumRow has none
             either, and the row itself opens the playlist. -->
        <div class="col-artist"></div>
        <div class="col-star">
            <button
                class="row-star"
                :class="{ 'is-starred': isStarred }"
                type="button"
                :aria-label="isStarred ? 'Remove from favorites' : 'Add to favorites'"
                @click="onStar"
            >
                <i :class="isStarred ? 'pi pi-heart-fill' : 'pi pi-heart'"></i>
            </button>
        </div>
        <div class="col-songs">{{ playlist.songCount }}</div>
        <div class="col-duration">{{ formatDuration(playlist.duration) }}</div>
    </router-link>
</template>

<style scoped>
/* Same template as AlbumRow so albums and playlists align in a mixed list —
   including the 2rem favorite column. Change one, change both (plus the two list
   headers); see unified-play-experience.md. */
.playlist-row {
    display: grid;
    grid-template-columns: 48px 2fr 1.5fr 2rem 4rem 5rem;
    align-items: center;
    gap: 1rem;
    height: 56px;
    padding: 0 0.5rem;
    text-decoration: none;
    color: inherit;
    border-bottom: 1px solid var(--p-content-border-color);
    cursor: pointer;
    transition: background-color 0.15s;
}

.playlist-row:hover:not(.placeholder) {
    background-color: var(--app-hover);
}

.col-cover {
    width: 40px;
    height: 40px;
    border-radius: 4px;
    overflow: hidden;
}

.col-cover img {
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
    color: rgba(255, 255, 255, 0.85);
}

.col-title {
    min-width: 0;
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

/* The heart's column, matching AlbumRow's. */
.col-star {
    display: flex;
    align-items: center;
    justify-content: center;
}

.row-star {
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: none;
    padding: 0;
    line-height: 1;
    color: var(--app-text-secondary);
    font-size: 1rem;
    cursor: pointer;
    opacity: 0;
    transition:
        opacity 0.15s,
        color 0.15s;
}

.playlist-row:hover .row-star,
.row-star.is-starred {
    opacity: 1;
}

/* Grey-but-filled, no accent: a favorite reads as favorite by the FILL alone —
   see TrackFavoriteButton and unified-play-experience.md. */
.row-star:hover,
.row-star:focus-visible {
    opacity: 1;
    color: var(--app-text-primary);
}

.col-songs,
.col-duration {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
    text-align: right;
}
</style>
