<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import AlbumCard from '@/components/library/AlbumCard.vue'
import ArtistEditDialog from '@/components/library/ArtistEditDialog.vue'
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

const editDialogVisible = ref(false)
const cacheBust = ref(0)

const coverUrl = computed(() => {
    if (!artist.value?.coverArt || !subsonicClient.isConfigured()) return null
    const base = subsonicClient.getCoverArtUrl(artist.value.coverArt, 250)
    return cacheBust.value > 0 ? `${base}&_cb=${cacheBust.value}` : base
})

const summary = computed(() => {
    const n = artist.value?.albumCount ?? 0
    if (n === 0) return ''
    return `${n} ${n === 1 ? 'album' : 'albums'}`
})

const sortedAlbums = computed(() => {
    if (!artist.value?.album) return []
    return [...artist.value.album].sort((a, b) => (b.year || 0) - (a.year || 0))
})
</script>

<template>
    <div class="artist-view">
        <div class="back-row">
            <Button icon="pi pi-arrow-left" text rounded @click="router.back()" />
        </div>

        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="error" class="error">
            <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
            <p>{{ error.message }}</p>
        </div>

        <ContentScaffold v-else-if="artist" :title="artist.name" :summary="summary">
            <template #actions>
                <Button
                    :icon="artist?.starred ? 'pi pi-star-fill' : 'pi pi-star'"
                    text
                    rounded
                    title="Toggle star"
                    @click="handleStar"
                />
                <Button
                    icon="pi pi-pencil"
                    text
                    rounded
                    title="Edit MusicBrainz match"
                    @click="editDialogVisible = true"
                />
            </template>

            <div class="artist-scroll">
                <div class="artist-body">
                    <div class="artist-hero">
                        <div class="artist-image">
                            <img v-if="coverUrl" :src="coverUrl" :alt="artist.name" />
                            <div v-else class="image-placeholder">
                                <i class="pi pi-user" style="font-size: 3rem"></i>
                            </div>
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
        </ContentScaffold>

        <ArtistEditDialog
            v-if="artist"
            v-model:visible="editDialogVisible"
            :artist-id="artist.id"
            :artist-name="artist.name"
            @saved="cacheBust++"
        />
    </div>
</template>

<style scoped>
.artist-view {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
}

.back-row {
    flex-shrink: 0;
    padding: 0.5rem 2rem 0;
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

.artist-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
}

.artist-body {
    max-width: var(--app-content-max-width);
    margin: 0 auto;
    padding: 1rem;
}

.artist-hero {
    display: flex;
    gap: 2rem;
    margin-bottom: 2rem;
}

.artist-image {
    width: 250px;
    height: 250px;
    flex-shrink: 0;
    border-radius: 8px;
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
