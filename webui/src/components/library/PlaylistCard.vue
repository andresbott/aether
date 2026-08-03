<script setup lang="ts">
import { computed } from 'vue'
import type { Playlist } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { usePlayer } from '@/composables/usePlayer'
import { useTogglePlaylistStar } from '@/composables/useSubsonicQueries'

const props = defineProps<{ playlist: Playlist }>()
const player = usePlayer()
const toggleStar = useTogglePlaylistStar()

const coverUrl = computed(() => {
    const art = props.playlist.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 200)
})

const isStarred = computed(() => !!props.playlist.starred)

const onStar = (event: Event): void => {
    event.preventDefault()
    event.stopPropagation()
    toggleStar.mutate({ id: props.playlist.id, starred: isStarred.value })
}

const onPlay = async (event: Event): Promise<void> => {
    event.preventDefault()
    event.stopPropagation()
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
    <router-link
        :to="{ name: 'playlist-detail', params: { id: playlist.id } }"
        class="playlist-card"
    >
        <div class="card-cover">
            <img v-if="coverUrl" :src="coverUrl" :alt="playlist.name" draggable="false" />
            <div v-else class="cover-placeholder">
                <i class="pi pi-list" style="font-size: 2rem"></i>
            </div>
        </div>
        <div class="card-info">
            <div class="card-text">
                <div class="card-title">{{ playlist.name }}</div>
                <div class="card-subtitle">{{ playlist.songCount }} songs</div>
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
            <button class="card-play" type="button" aria-label="Play playlist" @click="onPlay">
                <i class="pi pi-play"></i>
            </button>
        </div>
    </router-link>
</template>

<style scoped>
.playlist-card { position: relative; display: flex; flex-direction: column; text-decoration: none; color: inherit; border: 1px solid transparent; border-radius: 10px; padding: 0.5rem; transition: border-color 0.2s, background 0.2s, box-shadow 0.2s; cursor: pointer; }
.playlist-card:hover { border-color: var(--app-accent); background: var(--app-accent-soft); box-shadow: 0 6px 18px rgba(0, 0, 0, 0.12); }
.card-cover { width: 100%; aspect-ratio: 1; border-radius: 8px; overflow: hidden; background: var(--app-bg-subtle); }
.card-cover img { width: 100%; height: 100%; object-fit: cover; }
.cover-placeholder { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: rgba(255, 255, 255, 0.8); }
.card-info { display: flex; align-items: stretch; gap: 0.5rem; padding: 0.5rem 0.15rem 0.1rem; }
.card-text { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.card-title { font-size: 0.9rem; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.card-subtitle { font-size: 0.8rem; color: var(--app-text-secondary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.card-star { flex-shrink: 0; display: flex; align-items: center; justify-content: center; border: none; background: none; padding: 0 0.15rem; line-height: 1; color: var(--app-text-secondary); font-size: 1.1rem; cursor: pointer; opacity: 0; transition: opacity 0.15s, color 0.15s; }
.playlist-card:hover .card-star,
.card-star.is-starred { opacity: 1; }
/* A favorite reads as favorite by the FILL alone, not by colour — see
   TrackFavoriteButton and unified-play-experience.md. */
.card-star:hover { color: var(--app-text-primary); }
.card-play { flex-shrink: 0; display: flex; align-items: center; justify-content: center; border: none; background: none; padding: 0 0.15rem; line-height: 1; color: var(--app-text-secondary); font-size: 2rem; cursor: pointer; opacity: 0; transition: opacity 0.15s, color 0.15s; }
.playlist-card:hover .card-play { opacity: 1; }
.card-play:hover { color: var(--app-accent); }
</style>
