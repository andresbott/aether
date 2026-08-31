<script setup lang="ts">
import { computed } from 'vue'
import type { Artist } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { versionedCoverUrl } from '@/composables/useCoverVersion'
import { useToggleStar } from '@/composables/useSubsonicQueries'
import { useArtistDrag } from '@/composables/useArtistDrag'
import { usePlayer } from '@/composables/usePlayer'
import { pickRandomAlbum } from '@/utils/artistPlayback'

const props = defineProps<{
    artist: Artist
}>()

const toggleStar = useToggleStar()
const artistDrag = useArtistDrag()
const player = usePlayer()

const coverUrl = computed(() => {
    if (!props.artist.coverArt || !subsonicClient.isConfigured()) return null
    const base = subsonicClient.getCoverArtUrl(props.artist.coverArt, 200)
    // Same cache-bust as the detail view, so editing a cover updates the grid too.
    return versionedCoverUrl(base, props.artist.coverArt)
})

const isStarred = computed(() => !!props.artist.starred)

// The card is a router-link, so the toggle has to swallow the click or it would
// navigate to the artist detail view.
const onStar = (event: Event): void => {
    event.preventDefault()
    event.stopPropagation()
    toggleStar.mutate({ id: props.artist.id, starred: isStarred.value })
}

// Play a random album from this artist: fetch the artist, pick one album at
// random, fetch its songs, and play from the start.
const onPlay = async (event: Event): Promise<void> => {
    event.preventDefault()
    event.stopPropagation()
    const full = await subsonicClient.getArtist(props.artist.id)
    if (!full?.album?.length) return
    const randomAlbum = pickRandomAlbum(full.album)
    if (!randomAlbum) return
    const albumWithSongs = await subsonicClient.getAlbum(randomAlbum.id)
    if (albumWithSongs?.song?.length) player.playAlbum(albumWithSongs.song)
}

const onCardDragStart = (event: DragEvent): void => {
    artistDrag.start(event, props.artist, coverUrl.value)
}
</script>

<template>
    <router-link
        :to="{ name: 'artist', params: { id: artist.id } }"
        class="artist-card"
        draggable="true"
        @dragstart="onCardDragStart"
        @dragend="artistDrag.end"
    >
        <div class="card-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="artist.name" draggable="false" />
            <div v-else class="cover-placeholder">
                <i class="pi pi-user" style="font-size: 2rem"></i>
            </div>
        </div>
        <div class="card-info">
            <div class="card-text">
                <div class="card-title">{{ artist.name }}</div>
                <div class="card-subtitle">
                    <template v-if="artist.albumCount != null">{{ artist.albumCount }} {{ artist.albumCount === 1 ? 'album' : 'albums' }}</template>
                    <template v-else>&nbsp;</template>
                </div>
            </div>
            <button
                class="card-star"
                :class="{ 'is-starred': isStarred }"
                type="button"
                :aria-label="isStarred ? 'Remove from favorites' : 'Add to favorites'"
                @click="onStar"
            >
                <i :class="isStarred ? 'pi pi-heart-fill' : 'pi pi-heart'"></i>
            </button>
            <button class="card-play" type="button" aria-label="Play random album" @click="onPlay">
                <i class="pi pi-play"></i>
            </button>
        </div>
    </router-link>
</template>

<style scoped>
.artist-card {
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

/* Border + accent tint wrap the whole card (cover + text) on hover. */
.artist-card:hover {
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
    display: flex;
    align-items: stretch;
    gap: 0.5rem;
    padding: 0.5rem 0.15rem 0.1rem;
}

.card-text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
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

/*
 * Favorite toggle, always present but dimmed like the play icon until the card
 * is hovered — a card whose actions only appear on hover doesn't advertise that
 * it has any. A favorite is dimmed too: the FILL alone tells it apart, at any
 * opacity. Mirrors AlbumCard and PlaylistCard.
 */
.card-star {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: none;
    padding: 0 0.15rem;
    line-height: 1;
    color: var(--app-text-secondary);
    font-size: 1.1rem;
    cursor: pointer;
    opacity: 0.4;
    transition: opacity 0.15s, color 0.15s;
}

.artist-card:hover .card-star {
    opacity: 1;
}

/* A favorite reads as favorite by the FILL alone, not by colour — see
   TrackFavoriteButton and unified-play-experience.md. */
.card-star:hover {
    color: var(--app-text-primary);
}

/* Inline play icon spanning the height of both text lines, dimmed until hover.
   Mirrors AlbumCard. */
.card-play {
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    border: none;
    background: none;
    padding: 0 0.15rem;
    line-height: 1;
    color: var(--app-text-secondary);
    font-size: 2rem;
    cursor: pointer;
    opacity: 0.4;
    transition: opacity 0.15s, color 0.15s;
}

.artist-card:hover .card-play {
    opacity: 1;
}

.card-play:hover {
    color: var(--app-accent);
}
</style>
