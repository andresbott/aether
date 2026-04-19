<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import InputText from 'primevue/inputtext'
import { useSearch } from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Album, Artist, Song } from '@/types/subsonic'

const router = useRouter()
const { playNow } = usePlayer()

const query = ref('')
const showResults = ref(false)

const searchParams = computed(() => ({
    query: query.value,
    artistCount: 5,
    albumCount: 5,
    songCount: 10
}))

const { data: results, isLoading } = useSearch(searchParams)

const artists = computed<Artist[]>(() => results.value?.artist || [])
const albums = computed<Album[]>(() => results.value?.album || [])
const songs = computed<Song[]>(() => results.value?.song || [])

const hasResults = computed(
    () => artists.value.length > 0 || albums.value.length > 0 || songs.value.length > 0
)

const showDropdown = computed(
    () => showResults.value && query.value.length > 0
)

function onFocus() {
    showResults.value = true
}

function onBlur() {
    setTimeout(() => {
        showResults.value = false
    }, 200)
}

function goToArtist(artist: Artist) {
    router.push(`/artist/${artist.id}`)
    query.value = ''
    showResults.value = false
}

function goToAlbum(album: Album) {
    router.push(`/album/${album.id}`)
    query.value = ''
    showResults.value = false
}

function playSong(song: Song) {
    playNow(song)
    query.value = ''
    showResults.value = false
}

function getCoverUrl(id: string | undefined): string | null {
    if (!id) return null
    return subsonicClient.getCoverArtUrl(id, 40)
}
</script>

<template>
    <div class="search-container">
        <span class="search-input-wrapper">
            <i class="pi pi-search search-icon"></i>
            <InputText
                v-model="query"
                placeholder="Search..."
                class="search-input"
                @focus="onFocus"
                @blur="onBlur"
            />
        </span>

        <div v-if="showDropdown" class="search-dropdown">
            <div v-if="isLoading" class="search-loading">
                <i class="pi pi-spinner pi-spin"></i>
                <span>Searching...</span>
            </div>

            <div v-else-if="!hasResults && query.length > 0" class="search-empty">
                No results found
            </div>

            <template v-else>
                <div v-if="artists.length > 0" class="search-section">
                    <div class="search-section-label">Artists</div>
                    <div
                        v-for="artist in artists"
                        :key="artist.id"
                        class="search-result-item"
                        @mousedown.prevent="goToArtist(artist)"
                    >
                        <i class="pi pi-user result-icon"></i>
                        <span class="result-name">{{ artist.name }}</span>
                    </div>
                </div>

                <div v-if="albums.length > 0" class="search-section">
                    <div class="search-section-label">Albums</div>
                    <div
                        v-for="album in albums"
                        :key="album.id"
                        class="search-result-item"
                        @mousedown.prevent="goToAlbum(album)"
                    >
                        <img
                            v-if="getCoverUrl(album.coverArt)"
                            :src="getCoverUrl(album.coverArt)!"
                            class="result-cover"
                            alt=""
                        />
                        <i v-else class="pi pi-disc result-icon"></i>
                        <div class="result-info">
                            <span class="result-name">{{ album.name }}</span>
                            <span v-if="album.artist" class="result-secondary">{{ album.artist }}</span>
                        </div>
                    </div>
                </div>

                <div v-if="songs.length > 0" class="search-section">
                    <div class="search-section-label">Songs</div>
                    <div
                        v-for="song in songs"
                        :key="song.id"
                        class="search-result-item"
                        @mousedown.prevent="playSong(song)"
                    >
                        <img
                            v-if="getCoverUrl(song.coverArt)"
                            :src="getCoverUrl(song.coverArt)!"
                            class="result-cover"
                            alt=""
                        />
                        <i v-else class="pi pi-play result-icon"></i>
                        <div class="result-info">
                            <span class="result-name">{{ song.title }}</span>
                            <span v-if="song.artist" class="result-secondary">{{ song.artist }}</span>
                        </div>
                    </div>
                </div>
            </template>
        </div>
    </div>
</template>

<style scoped>
.search-container {
    position: relative;
    width: 300px;
}

.search-input-wrapper {
    position: relative;
    display: flex;
    align-items: center;
    width: 100%;
}

.search-icon {
    position: absolute;
    left: 0.75rem;
    color: var(--app-text-secondary);
    pointer-events: none;
    z-index: 1;
}

.search-input {
    width: 100%;
    padding-left: 2.25rem;
}

.search-dropdown {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    margin-top: 0.25rem;
    background: var(--app-surface);
    border: 1px solid var(--app-border);
    border-radius: 6px;
    max-height: 400px;
    overflow-y: auto;
    z-index: 1000;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.search-loading,
.search-empty {
    padding: 1rem;
    text-align: center;
    color: var(--app-text-secondary);
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
}

.search-section {
    padding: 0.25rem 0;
}

.search-section-label {
    padding: 0.5rem 0.75rem 0.25rem;
    font-size: 0.7rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--app-text-secondary);
}

.search-result-item {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding: 0.5rem 0.75rem;
    cursor: pointer;
    transition: background-color 0.15s;
}

.search-result-item:hover {
    background-color: var(--app-background);
}

.result-icon {
    font-size: 1rem;
    color: var(--app-text-secondary);
    width: 24px;
    text-align: center;
    flex-shrink: 0;
}

.result-cover {
    width: 32px;
    height: 32px;
    border-radius: 4px;
    object-fit: cover;
    flex-shrink: 0;
}

.result-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
}

.result-name {
    font-size: 0.875rem;
    color: var(--app-text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.result-secondary {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
</style>
