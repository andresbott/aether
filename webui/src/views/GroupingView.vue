<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import HeroHeader from '@/components/layout/HeroHeader.vue'
import HeroActions from '@/components/layout/HeroActions.vue'
import AlbumCard from '@/components/library/AlbumCard.vue'
import { useGrouping } from '@/composables/useSubsonicQueries'
import { versionedCoverUrl } from '@/composables/useCoverVersion'
import { usePlayer } from '@/composables/usePlayer'
import { subsonicClient } from '@/lib/api/subsonic'
import { apiErrorMessage } from '@/lib/apiError'
import type { Song } from '@/types/subsonic'

const props = defineProps<{ id: string }>()
const router = useRouter()
const player = usePlayer()

const { data: grouping, isLoading, error } = useGrouping(() => props.id)

const albums = computed(() => grouping.value?.album ?? [])

// The grouping has no art of its own: it wears its first album's cover, which is
// what the server resolves its coverArt id to.
const coverUrl = computed(() => {
    const art = grouping.value?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return versionedCoverUrl(subsonicClient.getCoverArtUrl(art, 250), art)
})

const heroMeta = computed(() => {
    const g = grouping.value
    if (!g) return []
    const parts: string[] = []
    const n = g.albumCount ?? albums.value.length
    if (n) parts.push(`${n} ${n === 1 ? 'album' : 'albums'}`)
    if (g.songCount) parts.push(`${g.songCount} ${g.songCount === 1 ? 'song' : 'songs'}`)
    if (g.year) parts.push(String(g.year))
    return parts
})

const gathering = ref(false)

// getGrouping returns albums without their songs; gather them in the grouping's
// order (oldest album first) so Play runs the series front to back.
async function gatherSongs(): Promise<Song[]> {
    const results = await Promise.all(albums.value.map((a) => subsonicClient.getAlbum(a.id)))
    return results.flatMap((al) => al?.song ?? [])
}

const onPlay = async (): Promise<void> => {
    if (gathering.value) return
    gathering.value = true
    try {
        const songs = await gatherSongs()
        if (songs.length) player.playAlbum(songs)
    } finally {
        gathering.value = false
    }
}

const onQueue = async (): Promise<void> => {
    if (gathering.value) return
    gathering.value = true
    try {
        const songs = await gatherSongs()
        if (songs.length) player.addMultipleToQueue(songs)
    } finally {
        gathering.value = false
    }
}
</script>

<template>
    <div class="grouping-view">
        <div v-if="isLoading" class="loading">
            <i class="icon-spin ms-progress-activity" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="error || !grouping" class="error">
            <i class="ms-warning" style="font-size: 2rem"></i>
            <p>{{ error ? apiErrorMessage(error) : 'Collection not found' }}</p>
        </div>

        <ContentScaffold v-else title="" show-back @back="router.back()">
            <div class="grouping-scroll">
                <HeroHeader
                    class="detail-hero"
                    eyebrow="Collection"
                    :editing="false"
                    :cover-editable="false"
                    cover-placeholder-icon="ms-collections-bookmark"
                    :cover-url="coverUrl"
                >
                    <template #read>
                        <h2 class="hero-name">{{ grouping.name }}</h2>
                        <div v-if="heroMeta.length" class="meta-row">
                            <span v-for="(part, i) in heroMeta" :key="part" :class="{ dot: i > 0 }">{{
                                part
                            }}</span>
                        </div>
                    </template>
                    <template #actions>
                        <HeroActions
                            :play-disabled="albums.length === 0"
                            can-queue
                            :busy="gathering"
                            @play="onPlay"
                            @queue="onQueue"
                        />
                    </template>
                </HeroHeader>

                <div class="grouping-below">
                    <div class="grouping-body content-col">
                        <div class="album-grid">
                            <AlbumCard v-for="album in albums" :key="album.id" :album="album" />
                        </div>
                    </div>
                </div>
            </div>
        </ContentScaffold>
    </div>
</template>

<style scoped>
.grouping-view {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
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

/* Same recipe as ArtistView: the hero bleeds edge to edge and the albums below
   centre on the shared content column. */
.grouping-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    box-sizing: border-box;
}

.grouping-body {
    padding-top: 1rem;
    padding-bottom: 1rem;
}

.album-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 2rem;
}
</style>
