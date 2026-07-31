<script setup lang="ts">
import { computed } from 'vue'
import type { Playlist } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { usePlayer } from '@/composables/usePlayer'
import { useTogglePlaylistStar } from '@/composables/useSubsonicQueries'
import { formatDuration } from '@/utils/formatDuration'

/**
 * One playlist as a list row — the playlist counterpart to `AlbumRow`, and the
 * single row rendering for playlists in the app.
 *
 * It deliberately mirrors AlbumRow's grid template (48px 2fr 1.5fr 4rem 5rem) so a
 * feed that interleaves albums and playlists lines its columns up. `PlaylistListView`
 * supplies its own header for those columns; the Discovery feed has no header, which
 * is fine because each cell still reads as label + value.
 */
const props = defineProps<{ playlist?: Playlist }>()

const player = usePlayer()
const toggleStar = useTogglePlaylistStar()

const coverUrl = computed(() => {
    const art = props.playlist?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 80)
})

const isStarred = computed(() => !!props.playlist?.starred)

// The row is a router-link, so both actions must swallow the click or they would
// navigate to the playlist detail view instead.
const onStar = (event: Event): void => {
    event.preventDefault()
    event.stopPropagation()
    if (!props.playlist) return
    toggleStar.mutate({ id: props.playlist.id, starred: isStarred.value })
}

const onPlay = async (event: Event): Promise<void> => {
    event.preventDefault()
    event.stopPropagation()
    if (!props.playlist) return
    const full = await subsonicClient.getPlaylist(props.playlist.id)
    if (full?.entry?.length) {
        player.playAlbum(full.entry)
        // Playlist plays are counted per playlist, so the play site records it —
        // usePlayer only ever sees a flat Song[].
        void subsonicClient.scrobble(props.playlist.id)
    }
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
        <div class="col-actions">
            <button
                class="row-star"
                :class="{ 'is-starred': isStarred }"
                type="button"
                :aria-label="isStarred ? 'Remove from favorites' : 'Add to favorites'"
                @click="onStar"
            >
                <i :class="isStarred ? 'pi pi-heart-fill' : 'pi pi-heart'"></i>
            </button>
            <button class="row-play" type="button" aria-label="Play playlist" @click="onPlay">
                <i class="pi pi-play"></i>
            </button>
        </div>
        <div class="col-songs">{{ playlist.songCount }}</div>
        <div class="col-duration">{{ formatDuration(playlist.duration) }}</div>
    </router-link>
</template>

<style scoped>
/* Same template as AlbumRow so albums and playlists align in a mixed list. */
.playlist-row {
    display: grid;
    grid-template-columns: 48px 2fr 1.5fr 4rem 5rem;
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

/* Occupies AlbumRow's artist column: a playlist has no artist, so the row's
   actions live here rather than adding a sixth column that albums would leave empty. */
.col-actions {
    display: flex;
    align-items: center;
    gap: 0.25rem;
}

.row-star,
.row-play {
    border: none;
    background: none;
    color: var(--app-text-secondary);
    cursor: pointer;
    opacity: 0;
    transition: opacity 0.15s, color 0.15s;
}

.row-star {
    font-size: 1rem;
}

.row-play {
    font-size: 1.1rem;
}

.playlist-row:hover .row-star,
.playlist-row:hover .row-play,
.row-star.is-starred {
    opacity: 1;
}

.row-star.is-starred,
.row-star:hover,
.row-play:hover {
    color: var(--app-accent);
}

.col-songs,
.col-duration {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
    text-align: right;
}
</style>
