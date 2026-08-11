<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRouter } from 'vue-router'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import HeroHeader from '@/components/layout/HeroHeader.vue'
import HeroActions from '@/components/layout/HeroActions.vue'
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

const currentTrackId = computed(() => player.currentTrack.value?.id)

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

// Double-clicking a track appends it to the end of the queue rather than
// replacing the queue with the album (see docs/architecture/unified-play-experience.md).
const enqueueTrack = (index: number): void => {
    const song = orderedSongs.value[index]
    if (song) player.enqueueAndPlayIfIdle([song])
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
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="error" class="error">
            <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
            <p>{{ error.message }}</p>
        </div>

        <ContentScaffold v-else-if="album" title="" show-back @back="router.back()">
            <template #actions>
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
                <div class="album-body content-col">
                    <HeroHeader
                        eyebrow="Album"
                        cover-placeholder-icon="pi pi-music"
                        :cover-url="coverUrl"
                        :cover-editable="false"
                        :editing="false"
                    >
                        <template #read>
                            <h2 class="hero-name">{{ album.name }}</h2>
                            <router-link
                                v-if="album.artistId"
                                :to="{ name: 'artist', params: { id: album.artistId } }"
                                class="artist-link"
                            >
                                {{ album.artist }}
                            </router-link>
                            <p v-else class="artist-name">{{ album.artist }}</p>
                            <div class="meta-row">
                                <span v-if="album.year">{{ album.year }}</span>
                                <span v-if="summary" :class="{ dot: !!album.year }">{{
                                    summary
                                }}</span>
                            </div>
                        </template>
                        <template #actions>
                            <HeroActions
                                :play-disabled="!album.song?.length"
                                can-queue
                                can-star
                                :starred="!!album.starred"
                                @play="playAlbum"
                                @queue="addToQueue"
                                @star="handleStar"
                            />
                        </template>
                    </HeroHeader>

                    <div v-if="orderedSongs.length > 0" class="track-list">
                        <div class="track-list-header">
                            <span class="col-index">#</span>
                            <span class="col-title">Title</span>
                            <span class="col-artist">Artist</span>
                            <!-- The favorite column is hover-revealed per row, so
                                 its header stays blank rather than labelling a
                                 control that is usually invisible. -->
                            <span class="col-star"></span>
                            <span class="col-duration" aria-label="Duration">
                                <i class="pi pi-clock"></i>
                            </span>
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
                                :playing="row.song.id === currentTrackId"
                                @select="(p) => onRowClick(row.index, p)"
                                @enqueue="enqueueTrack(row.index)"
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
    /* Recipe B: uniform rail clearance so the column matches the list views. */
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    box-sizing: border-box;
}

.album-body {
    padding-top: 1rem;
    padding-bottom: 1rem;
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

.track-list {
    /* Shared grid template so the header and every row (a child component) align.
       Custom properties inherit through the DOM regardless of scoped styles. */
    --album-track-cols: 38px minmax(0, 2.4fr) minmax(0, 1.4fr) 2rem 62px;
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
