<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import AlbumTrackRow from '@/components/library/AlbumTrackRow.vue'
import { useAlbum, useToggleStar } from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { useAlbumDrag } from '@/composables/useAlbumDrag'
import { useSongsDrag } from '@/composables/useSongsDrag'
import { useRowSelection } from '@/composables/useRowSelection'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Song } from '@/types/subsonic'

const props = defineProps<{ id: string }>()
const router = useRouter()
const player = usePlayer()
const toggleStar = useToggleStar()
const albumDrag = useAlbumDrag()
const songsDrag = useSongsDrag()
const { isSelected, onRowClick, selectionForDrag, clearSelection } = useRowSelection()

const onAlbumDragStart = (event: DragEvent): void => {
    if (album.value) albumDrag.start(event, album.value, coverUrl.value)
}

const handleStar = () => {
    if (!album.value) return
    toggleStar.mutate({ id: album.value.id, starred: !!album.value.starred })
}

const { data: album, isLoading, error } = useAlbum(props.id)

const coverUrl = computed(() => {
    if (!album.value?.coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(album.value.coverArt, 250)
})

const totalDuration = computed(() => {
    if (!album.value?.duration) return ''
    const mins = Math.floor(album.value.duration / 60)
    return `${mins} min`
})

const summary = computed(() => {
    if (!album.value) return ''
    const parts: string[] = []
    const n = album.value.songCount ?? album.value.song?.length ?? 0
    if (n > 0) parts.push(`${n} ${n === 1 ? 'song' : 'songs'}`)
    if (totalDuration.value) parts.push(totalDuration.value)
    return parts.join(' • ')
})

const playAlbum = () => {
    if (album.value?.song) {
        player.playAlbum(album.value.song)
    }
}

const addToQueue = () => {
    if (album.value?.song) {
        player.addMultipleToQueue(album.value.song)
    }
}

const playFromIndex = (index: number): void => {
    if (index >= 0 && index < orderedSongs.value.length) {
        player.playAlbum(orderedSongs.value, index)
    }
}

const discs = computed(() => {
    const songs = album.value?.song ?? []
    const groups = new Map<number, Song[]>()
    for (const s of songs) {
        const d = s.discNumber ?? 1
        if (!groups.has(d)) groups.set(d, [])
        groups.get(d)!.push(s)
    }
    return [...groups.entries()]
        .sort(([a], [b]) => a - b)
        .map(([discNumber, discSongs]) => ({ discNumber, songs: discSongs }))
})

const hasMultipleDiscs = computed(() => discs.value.length > 1)

// Flat track list ordered by disc; selection indices refer to positions in it.
const orderedSongs = computed(() => discs.value.flatMap((disc) => disc.songs))

// Disc groups carrying each row's flat index, so the template can render disc
// headers while every row keeps its position in `orderedSongs`.
const discGroups = computed(() => {
    let i = 0
    return discs.value.map((disc) => ({
        discNumber: disc.discNumber,
        rows: disc.songs.map((song) => ({ song, index: i++ }))
    }))
})

// A drag from a selected row carries the whole selection; from an unselected row
// it carries just that row. The songs are resolved from the flat list.
const onRowDragStart = (event: DragEvent, index: number): void => {
    const songs = selectionForDrag(index)
        .map((i) => orderedSongs.value[i])
        .filter((s): s is Song => s !== undefined)
    songsDrag.start(event, songs, event.currentTarget as HTMLElement)
}

// Discard the selection when navigating to a different album.
watch(
    () => props.id,
    () => clearSelection()
)
</script>

<template>
    <div class="album-view">
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

        <ContentScaffold v-else-if="album" :title="album.name" :summary="summary">
            <template #actions>
                <Button label="Play" icon="pi pi-play" @click="playAlbum" />
                <Button
                    label="Add to Queue"
                    icon="pi pi-plus"
                    severity="secondary"
                    text
                    @click="addToQueue"
                />
                <Button
                    :icon="album?.starred ? 'pi pi-star-fill' : 'pi pi-star'"
                    text
                    rounded
                    @click="handleStar"
                />
                <span
                    class="album-drag-handle"
                    draggable="true"
                    v-tooltip.bottom="'Drag album to queue'"
                    @dragstart="onAlbumDragStart"
                    @dragend="albumDrag.end"
                >
                    <i class="pi pi-bars"></i>
                </span>
            </template>

            <div class="album-scroll">
                <div class="album-body">
                    <div class="album-hero">
                        <div class="album-cover">
                            <img v-if="coverUrl" :src="coverUrl" :alt="album.name" />
                            <div v-else class="cover-placeholder">
                                <i class="pi pi-music" style="font-size: 3rem"></i>
                            </div>
                        </div>
                        <div class="album-info">
                            <router-link
                                v-if="album.artistId"
                                :to="{ name: 'artist', params: { id: album.artistId } }"
                                class="artist-link"
                            >
                                {{ album.artist }}
                            </router-link>
                            <p v-else class="artist-name">{{ album.artist }}</p>
                            <p v-if="album.year" class="album-meta">{{ album.year }}</p>
                        </div>
                    </div>

                    <div v-if="orderedSongs.length > 0" class="track-list">
                        <div class="track-list-header">
                            <span class="col-index">#</span>
                            <span class="col-title">Title</span>
                            <span class="col-artist">Artist</span>
                            <span class="col-duration">Duration</span>
                        </div>
                        <template v-for="group in discGroups" :key="group.discNumber">
                            <div v-if="hasMultipleDiscs" class="disc-header">
                                Disc {{ group.discNumber }}
                            </div>
                            <AlbumTrackRow
                                v-for="row in group.rows"
                                :key="row.song.id"
                                :song="row.song"
                                :index="row.index"
                                :selected="isSelected(row.index)"
                                @select="(p) => onRowClick(row.index, p)"
                                @play="playFromIndex(row.index)"
                                @dragstart="(e) => onRowDragStart(e, row.index)"
                                @dragend="songsDrag.end"
                            />
                        </template>
                    </div>
                </div>
            </div>
        </ContentScaffold>
    </div>
</template>

<style scoped>
.album-view {
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

.album-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
}

.album-body {
    max-width: var(--app-content-max-width);
    margin: 0 auto;
    padding: 1rem;
}

.album-hero {
    display: flex;
    gap: 2rem;
    margin-bottom: 2rem;
}

.album-cover {
    width: 250px;
    height: 250px;
    flex-shrink: 0;
    border-radius: 8px;
    overflow: hidden;
}

.album-cover img {
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

.album-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.artist-link {
    font-size: 1.25rem;
    color: var(--app-accent);
}

.artist-name {
    font-size: 1.25rem;
    color: var(--app-text-secondary);
    margin: 0;
}

.album-meta {
    color: var(--app-text-secondary);
    font-size: 0.95rem;
    margin: 0;
}

.track-list {
    /* Shared grid template so the header and every row (a child component) align.
       Custom properties inherit through the DOM regardless of scoped styles. */
    --album-track-cols: 2.5rem minmax(0, 2fr) minmax(0, 1fr) 4.5rem;
    display: flex;
    flex-direction: column;
}

.track-list-header {
    display: grid;
    grid-template-columns: var(--album-track-cols);
    column-gap: 0.75rem;
    padding: 0 0.5rem 0.4rem;
    border-bottom: 1px solid var(--app-border);
    margin-bottom: 0.25rem;
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--app-text-secondary);
}

.track-list-header .col-index,
.track-list-header .col-duration {
    text-align: right;
}

.disc-header {
    background: var(--app-accent);
    color: #fff;
    text-align: center;
    padding: 0.7rem 1rem;
    margin: 0.5rem 0 0.25rem;
    font-size: 0.8rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
}

.album-drag-handle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 2.25rem;
    height: 2.25rem;
    color: var(--app-text-secondary);
    cursor: grab;
}

.album-drag-handle:active {
    cursor: grabbing;
}
</style>
