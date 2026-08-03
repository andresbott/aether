<script setup lang="ts">
import { computed } from 'vue'
import type { Album } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { useAlbumDrag } from '@/composables/useAlbumDrag'
import { usePlayer } from '@/composables/usePlayer'
import { useToggleStar } from '@/composables/useSubsonicQueries'

const props = defineProps<{
    album?: Album
}>()

const albumDrag = useAlbumDrag()
const player = usePlayer()
const toggleStar = useToggleStar()

const isStarred = computed(() => !!props.album?.starred)

// The card is a router-link, so the toggle has to swallow the click or it would
// navigate to the album detail view.
const onStar = (event: Event): void => {
    event.preventDefault()
    event.stopPropagation()
    if (!props.album) return
    toggleStar.mutate({ id: props.album.id, starred: isStarred.value })
}

// Play from the card without navigating: album summaries carry no tracks, so
// fetch the full album first, then queue it.
const onPlay = async (event: Event): Promise<void> => {
    event.preventDefault()
    event.stopPropagation()
    if (!props.album) return
    const full = await subsonicClient.getAlbum(props.album.id)
    if (full?.song?.length) player.playAlbum(full.song)
}

const coverUrl = computed(() => {
    const art = props.album?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 200)
})

const onCardDragStart = (event: DragEvent): void => {
    if (props.album) albumDrag.start(event, props.album, coverUrl.value)
}
</script>

<template>
    <div v-if="!album" class="album-card placeholder" aria-hidden="true">
        <div class="card-cover"></div>
        <div class="card-info">
            <div class="card-text">
                <div class="card-title"></div>
                <div class="card-subtitle"></div>
            </div>
        </div>
    </div>
    <router-link
        v-else
        :to="{ name: 'album', params: { id: album.id } }"
        class="album-card"
        draggable="true"
        @dragstart="onCardDragStart"
        @dragend="albumDrag.end"
    >
        <div class="card-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="album.name" draggable="false" />
            <div v-else class="cover-placeholder">
                <i class="pi pi-music" style="font-size: 2rem"></i>
            </div>
        </div>
        <div class="card-info">
            <div class="card-text">
                <div class="card-title">{{ album.name }}</div>
                <div class="card-subtitle">{{ album.artist }}</div>
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
            <button class="card-play" type="button" aria-label="Play album" @click="onPlay">
                <i class="pi pi-play"></i>
            </button>
        </div>
    </router-link>
</template>

<style scoped>
.album-card {
    position: relative;
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
.album-card:hover {
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
 * opacity. Mirrors PlaylistCard.
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

.album-card:hover .card-star {
    opacity: 1;
}

/* A favorite reads as favorite by the FILL alone, not by colour — see
   TrackFavoriteButton and unified-play-experience.md. */
.card-star:hover {
    color: var(--app-text-primary);
}

/* Inline play icon spanning the height of both text lines, dimmed until hover. */
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

.album-card:hover .card-play {
    opacity: 1;
}

.card-play:hover {
    color: var(--app-accent);
}

.album-card.placeholder {
    cursor: default;
}

.album-card.placeholder .card-cover {
    background: var(--app-hover, rgba(127, 127, 127, 0.08));
}

.album-card.placeholder .card-title,
.album-card.placeholder .card-subtitle {
    height: 0.9em;
    margin: 0.15rem 0;
    border-radius: 3px;
    background: var(--app-hover, rgba(127, 127, 127, 0.08));
}
</style>
