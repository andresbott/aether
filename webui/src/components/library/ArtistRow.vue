<script setup lang="ts">
import { computed } from 'vue'
import type { Artist } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { versionedCoverUrl } from '@/composables/useCoverVersion'
import { useToggleStar } from '@/composables/useSubsonicQueries'

const props = defineProps<{ artist?: Artist }>()

const toggleStar = useToggleStar()

const coverUrl = computed(() => {
    const art = props.artist?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    // Same cache-bust as the detail view, so editing a cover updates the list too.
    return versionedCoverUrl(subsonicClient.getCoverArtUrl(art, 80), art)
})

const isStarred = computed(() => !!props.artist?.starred)

// Mirrors ArtistCard's toggle — the row is a router-link, so the click has to be
// swallowed or it would navigate to the artist detail view. Artist rows are
// query-backed, so the refetch `useToggleStar` triggers is what updates the heart.
const onStar = (event: Event): void => {
    event.preventDefault()
    event.stopPropagation()
    if (!props.artist) return
    toggleStar.mutate({ id: props.artist.id, starred: isStarred.value })
}
</script>

<template>
    <div v-if="!artist" class="artist-row placeholder">
        <div class="col-avatar"></div>
        <div class="col-name"></div>
    </div>
    <router-link
        v-else
        :to="{ name: 'artist', params: { id: artist.id } }"
        class="artist-row"
    >
        <div class="col-avatar">
            <img v-if="coverUrl" :src="coverUrl" :alt="artist.name" />
            <div v-else class="avatar-placeholder"><i class="pi pi-user"></i></div>
        </div>
        <div class="col-name">{{ artist.name }}</div>
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
        <!-- Always rendered so the count column keeps its grid slot; without it a
             countless artist would let the heart slide right into that column. -->
        <div class="col-count">
            <template v-if="artist.albumCount !== undefined">
                {{ artist.albumCount }} {{ artist.albumCount === 1 ? 'album' : 'albums' }}
            </template>
        </div>
    </router-link>
</template>

<style scoped>
/* The 2rem favorite column matches AlbumRow's, so a heart sits at the same size on
   every library row. ArtistListView's header mirrors this template — change one,
   change both. */
.artist-row {
    display: grid;
    grid-template-columns: 48px 1fr 2rem 7rem;
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

.artist-row:hover:not(.placeholder) {
    background-color: var(--app-hover);
}

.col-avatar {
    width: 40px;
    height: 40px;
    border-radius: 50%;
    overflow: hidden;
}

.col-avatar img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.avatar-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: rgba(255, 255, 255, 0.8);
}

.col-name {
    font-weight: 600;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

/* Hover-revealed, pinned visible while favorited, and grey-but-filled rather than
   accented — the same rules as the cards and the other rows. */
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

.artist-row:hover .row-star,
.row-star.is-starred {
    opacity: 1;
}

.row-star:hover,
.row-star:focus-visible {
    opacity: 1;
    color: var(--app-text-primary);
}

.col-count {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
    text-align: right;
}
</style>
