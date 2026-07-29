<script setup lang="ts">
import type { Playlist } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { usePlayer } from '@/composables/usePlayer'
import { useTogglePlaylistStar } from '@/composables/useSubsonicQueries'

defineProps<{ playlists: Playlist[] }>()
const player = usePlayer()
const toggleStar = useTogglePlaylistStar()

const coverUrl = (art?: string): string | null => {
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 48)
}

const star = (event: Event, pl: Playlist): void => {
    event.preventDefault()
    event.stopPropagation()
    toggleStar.mutate({ id: pl.id, starred: !!pl.starred })
}

const play = async (event: Event, id: string): Promise<void> => {
    event.preventDefault()
    event.stopPropagation()
    const full = await subsonicClient.getPlaylist(id)
    if (full?.entry?.length) {
        player.playAlbum(full.entry)
        void subsonicClient.scrobble(id)
    }
}
</script>

<template>
    <div class="playlist-list content-col">
        <div class="list-header">
            <span class="col-cover"></span>
            <span class="col-name">Playlist</span>
            <span class="col-count">Songs</span>
            <span class="col-star"></span>
            <span class="col-play"></span>
        </div>
        <router-link
            v-for="pl in playlists"
            :key="pl.id"
            :to="{ name: 'playlist-detail', params: { id: pl.id } }"
            class="playlist-row"
        >
            <span class="col-cover">
                <img v-if="coverUrl(pl.coverArt)" :src="coverUrl(pl.coverArt)!" alt="" />
                <i v-else class="pi pi-list"></i>
            </span>
            <span class="col-name">{{ pl.name }}</span>
            <span class="col-count">{{ pl.songCount }}</span>
            <span class="col-star">
                <button
                    class="row-star"
                    type="button"
                    :aria-label="pl.starred ? 'Unstar playlist' : 'Star playlist'"
                    @click="star($event, pl)"
                >
                    <i :class="pl.starred ? 'pi pi-star-fill' : 'pi pi-star'"></i>
                </button>
            </span>
            <span class="col-play">
                <button class="row-play" type="button" aria-label="Play playlist" @click="play($event, pl.id)">
                    <i class="pi pi-play"></i>
                </button>
            </span>
        </router-link>
    </div>
</template>

<style scoped>
.playlist-list { padding-top: 0; padding-bottom: 0; }
.list-header, .playlist-row { display: grid; grid-template-columns: 48px 1fr 4rem 3rem 3rem; align-items: center; gap: 1rem; padding: 0 0.5rem; }
.list-header { height: 36px; font-size: 0.75rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; color: var(--app-text-secondary); border-bottom: 1px solid var(--p-content-border-color); }
.list-header .col-count { text-align: right; }
.playlist-row { height: 56px; text-decoration: none; color: inherit; border-radius: 6px; }
.playlist-row:hover { background: var(--app-hover); }
.playlist-row .col-cover { width: 40px; height: 40px; border-radius: 4px; overflow: hidden; display: flex; align-items: center; justify-content: center; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: rgba(255, 255, 255, 0.85); }
.playlist-row .col-cover img { width: 100%; height: 100%; object-fit: cover; }
.col-name { min-width: 0; font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.col-count { text-align: right; color: var(--app-text-secondary); font-size: 0.85rem; }
.col-star { display: flex; justify-content: center; }
.row-star { border: none; background: none; color: var(--app-text-secondary); font-size: 1rem; cursor: pointer; opacity: 0; transition: opacity 0.15s, color 0.15s; }
.playlist-row:hover .row-star { opacity: 1; }
.row-star:hover { color: var(--app-accent); }
.row-star .pi-star-fill { opacity: 1; color: var(--app-accent); }
.col-play { display: flex; justify-content: center; }
.row-play { border: none; background: none; color: var(--app-text-secondary); font-size: 1.1rem; cursor: pointer; opacity: 0; transition: opacity 0.15s, color 0.15s; }
.playlist-row:hover .row-play { opacity: 1; }
.row-play:hover { color: var(--app-accent); }
</style>
