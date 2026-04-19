<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import AlbumCard from '@/components/library/AlbumCard.vue'
import { useArtist, useToggleStar } from '@/composables/useSubsonicQueries'
import { subsonicClient } from '@/lib/api/subsonic'

const props = defineProps<{ id: string }>()
const router = useRouter()
const toggleStar = useToggleStar()

const handleStar = () => {
    if (!artist.value) return
    toggleStar.mutate({ id: artist.value.id, starred: !!artist.value.starred })
}

const { data: artist, isLoading, error } = useArtist(props.id)

const coverUrl = computed(() => {
    if (!artist.value?.coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(artist.value.coverArt, 250)
})

const sortedAlbums = computed(() => {
    if (!artist.value?.album) return []
    return [...artist.value.album].sort((a, b) => (b.year || 0) - (a.year || 0))
})
</script>

<template>
    <div class="artist-view">
        <Button icon="pi pi-arrow-left" text rounded @click="router.back()" />

        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="error" class="error">
            <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
            <p>{{ error.message }}</p>
        </div>

        <div v-else-if="artist" class="artist-content">
            <div class="artist-header">
                <div class="artist-image">
                    <img v-if="coverUrl" :src="coverUrl" :alt="artist.name" />
                    <div v-else class="image-placeholder">
                        <i class="pi pi-user" style="font-size: 3rem"></i>
                    </div>
                </div>
                <div class="artist-info">
                    <h1>{{ artist.name }}</h1>
                    <p v-if="artist.albumCount" class="album-count">
                        {{ artist.albumCount }} albums
                    </p>
                    <Button
                        :icon="artist?.starred ? 'pi pi-star-fill' : 'pi pi-star'"
                        text
                        rounded
                        @click="handleStar"
                    />
                </div>
            </div>

            <section v-if="sortedAlbums.length > 0" class="discography">
                <h2>Albums</h2>
                <div class="album-grid">
                    <AlbumCard
                        v-for="album in sortedAlbums"
                        :key="album.id"
                        :album="album"
                    />
                </div>
            </section>
        </div>
    </div>
</template>

<style scoped>
.artist-view {
    max-width: 1400px;
    margin: 0 auto;
}

.loading,
.error {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 3rem;
    gap: 1rem;
    color: var(--app-text-secondary);
}

.error {
    color: #ef4444;
}

.artist-header {
    display: flex;
    gap: 2rem;
    margin: 1.5rem 0 2.5rem;
    align-items: center;
}

.artist-image {
    width: 200px;
    height: 200px;
    flex-shrink: 0;
    border-radius: 50%;
    overflow: hidden;
}

.artist-image img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.image-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: rgba(255, 255, 255, 0.8);
}

.artist-info h1 {
    font-size: 2.5rem;
    font-weight: 700;
    margin: 0;
}

.album-count {
    color: var(--app-text-secondary);
    margin: 0.25rem 0 0;
}

.discography h2 {
    font-size: 1.5rem;
    font-weight: 600;
    margin-bottom: 1.5rem;
}

.album-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 2rem;
}
</style>
