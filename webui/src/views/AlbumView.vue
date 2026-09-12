<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { useRouter, onBeforeRouteLeave } from 'vue-router'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import HeroHeader from '@/components/layout/HeroHeader.vue'
import HeroActions from '@/components/layout/HeroActions.vue'
import HeroSelectionBar from '@/components/layout/HeroSelectionBar.vue'
import EditActionBar from '@/components/layout/EditActionBar.vue'
import AlbumTrackRow from '@/components/library/AlbumTrackRow.vue'
import TrackActionSheet from '@/components/library/TrackActionSheet.vue'
import { useAlbum, useToggleStar, useUpdateAlbumCover } from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { useAlbumDrag } from '@/composables/useAlbumDrag'
import { useSongsDrag } from '@/composables/useSongsDrag'
import { useRowSelection } from '@/composables/useRowSelection'
import { useAuth } from '@/composables/useAuth'
import { bumpCoverVersion, versionedCoverUrl } from '@/composables/useCoverVersion'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Song } from '@/types/subsonic'

const props = defineProps<{ id: string }>()
const router = useRouter()
const player = usePlayer()
const toggleStar = useToggleStar()
const albumDrag = useAlbumDrag()
const songsDrag = useSongsDrag()
const { isSelected, onRowClick, selectionForDrag, clearSelection, selectedCount, selectedIndices } =
    useRowSelection()

const actionSong = ref<Song | null>(null)
const actionIndex = ref(0)
const actionSheetOpen = ref(false)

// Touch tap-to-play: queue the album as shown and start at the tapped track, the
// same primitive the hero Play uses. NOT `playNow`, which would wipe the queue
// down to the single tapped song and discard the rest of the album
// (see docs/architecture/unified-play-experience.md, "Touch contract").
const playTrack = (index: number): void => {
    const songs = orderedSongs.value
    if (songs[index]) player.playAlbum(songs, index)
}

const openTrackMenu = (index: number): void => {
    const song = orderedSongs.value[index]
    if (!song) return
    actionSong.value = song
    actionIndex.value = index
    actionSheetOpen.value = true
}

// The hero cover is the drag source: grabbing it drags the album into the queue.
const onCoverDragStart = (event: DragEvent): void => {
    if (album.value) albumDrag.start(event, album.value, coverUrl.value)
}

const handleStar = () => {
    if (!album.value) return
    toggleStar.mutate({ id: album.value.id, starred: !!album.value.starred })
}

const { data: album, isLoading, error } = useAlbum(props.id)

const MAX_COVER_BYTES = 5 * 1024 * 1024

const updateCover = useUpdateAlbumCover()
const { isAdmin } = useAuth()

// --- Cover editing (mirrors GenreDetailView: staged locally, applied on Save) ---
const editing = ref(false)
const selectedFile = ref<File | null>(null)
const previewUrl = ref<string | null>(null)
const coverClear = ref(false)
const coverSizeError = ref<string | null>(null)

const dirty = computed(() => selectedFile.value !== null || coverClear.value)

function resetCoverStaging(): void {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
    selectedFile.value = null
    coverClear.value = false
    coverSizeError.value = null
}

const onCoverSelect = (file: File): void => {
    if (file.size > MAX_COVER_BYTES) {
        coverSizeError.value = `File is ${(file.size / 1024 / 1024).toFixed(1)} MB — max is 5 MB`
        return
    }
    coverSizeError.value = null
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    selectedFile.value = file
    previewUrl.value = URL.createObjectURL(file)
    coverClear.value = false
}

const onRemoveCover = (): void => {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
    selectedFile.value = null
    coverClear.value = true
    coverSizeError.value = null
}

const saveEdit = (): void => {
    if (!dirty.value || !album.value?.id) {
        editing.value = false
        return
    }
    updateCover.mutate(
        {
            albumId: album.value.id,
            coverFile: selectedFile.value ?? undefined,
            coverClear: coverClear.value || undefined
        },
        {
            onSuccess: () => {
                resetCoverStaging()
                // Shared, module-level version: a local ref would die with this
                // component, so navigating away and back would re-show the old
                // image from the browser's in-memory cache.
                if (album.value?.coverArt) bumpCoverVersion(album.value.coverArt)
                editing.value = false
            }
        }
    )
}

const cancelEdit = (): void => {
    resetCoverStaging()
    editing.value = false
}

// Unsaved-changes guards (mirror Genre/Artist/Playlist detail views).
onBeforeRouteLeave(() => {
    if (dirty.value) {
        return window.confirm('You have unsaved changes. Leave without saving?')
    }
})
const onBeforeUnload = (e: BeforeUnloadEvent): void => {
    if (!dirty.value) return
    e.preventDefault()
    e.returnValue = ''
}
onMounted(() => window.addEventListener('beforeunload', onBeforeUnload))
onUnmounted(() => {
    window.removeEventListener('beforeunload', onBeforeUnload)
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
})

const currentTrackId = computed(() => player.currentTrack.value?.id)

const coverUrl = computed(() => {
    if (previewUrl.value) return previewUrl.value
    if (coverClear.value) return null
    if (!album.value?.coverArt || !subsonicClient.isConfigured()) return null
    const base = subsonicClient.getCoverArtUrl(album.value.coverArt, 250)
    return versionedCoverUrl(base, album.value.coverArt)
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

// The selected songs in list order, fed to the in-hero selection action row.
const selectedSongs = computed(() =>
    [...selectedIndices.value]
        .sort((a, b) => a - b)
        .map((i) => orderedSongs.value[i])
        .filter((s): s is Song => s !== undefined)
)

const playSelection = (): void => {
    if (selectedSongs.value.length) player.playAlbum(selectedSongs.value)
}
const queueSelection = (): void => {
    if (selectedSongs.value.length) player.addMultipleToQueue(selectedSongs.value)
}

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
    () => {
        clearSelection()
        resetCoverStaging()
        editing.value = false
    }
)

// Entering edit mode gives the hero's action row over to Save/Cancel, so a stray
// selection would have nowhere to act — drop it.
watch(editing, (isEditing) => {
    if (isEditing) clearSelection()
})
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
            <div class="album-scroll">
                <HeroHeader
                    class="detail-hero"
                    eyebrow="Album"
                    cover-placeholder-icon="pi pi-music"
                    cover-back-label="Album cover"
                    :cover-url="coverUrl"
                    :cover-size-error="coverSizeError"
                    cover-draggable
                    cover-drag-title="Drag to the play queue"
                    v-model:editing="editing"
                    @cover-select="onCoverSelect"
                    @cover-remove="onRemoveCover"
                    @cover-dragstart="onCoverDragStart"
                    @cover-dragend="albumDrag.end"
                >
                    <template v-if="isAdmin" #edit-actions>
                        <EditActionBar
                            v-model:editing="editing"
                            :can-delete="false"
                            :save-disabled="!dirty"
                            :saving="updateCover.isPending.value"
                            :dirty="dirty"
                            @save="saveEdit"
                            @cancel="cancelEdit"
                        />
                    </template>
                    <template #cover-note>
                        <div class="cover-help">
                            Remove clears aether's managed cover and reverts to the folder or
                            embedded artwork.
                        </div>
                    </template>
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
                            <span v-if="summary" :class="{ dot: !!album.year }">{{ summary }}</span>
                        </div>
                    </template>
                    <template #actions>
                        <HeroSelectionBar
                            v-if="selectedCount > 0"
                            :count="selectedCount"
                            :songs="selectedSongs"
                            @play="playSelection"
                            @queue="queueSelection"
                            @clear="clearSelection"
                        />
                        <HeroActions
                            v-else
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

                <div class="album-below">
                    <div class="album-body content-col">
                        <div v-if="orderedSongs.length > 0" class="track-list">
                            <div class="track-list-header">
                                <span class="col-index">#</span>
                                <span class="col-title">Title</span>
                                <span class="col-artist">Artist</span>
                                <!-- The select and favorite columns are hover-revealed
                                     per row, so their headers stay blank rather than
                                     labelling controls that are usually invisible. -->
                                <span class="col-select"></span>
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
                                    :selecting="selectedCount > 0"
                                    :playing="row.song.id === currentTrackId"
                                    @select="(p) => onRowClick(row.index, p)"
                                    @enqueue="enqueueTrack(row.index)"
                                    @play="playTrack(row.index)"
                                    @menu="openTrackMenu(row.index)"
                                    @dragstart="(e) => onRowDragStart(e, row.index)"
                                    @dragend="songsDrag.end"
                                />
                            </template>
                        </div>
                    </div>
                </div>
            </div>
            <TrackActionSheet
                v-model:visible="actionSheetOpen"
                :song="actionSong"
                @play="playTrack(actionIndex)"
            />
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

/* Recipe B, but with the hero pulled OUT of the centered column so the duotone
   band bleeds edge to edge: the scroll container no longer reserves the rail
   clearance itself (the band spans its full content box), and each part reserves
   it instead — the hero internally (HeroHeader's .hero-pad) and the body via
   .album-below — so both columns still line up. */
.album-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    box-sizing: border-box;
}

.album-below {
    box-sizing: border-box;
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
}

.album-body {
    padding-top: 1rem;
    padding-bottom: 1rem;
}

/* The identity links sit on the dark duotone band, so they take a light cyan
   rather than the app accent (too dark on the band in light mode). */
.artist-link {
    font-size: 1.25rem;
    color: #8fecff;
}

.artist-name {
    font-size: 1.25rem;
    color: #cdd7df;
    margin: 0;
}

.track-list {
    /* Shared grid template so the header and every row (a child component) align.
       Custom properties inherit through the DOM regardless of scoped styles. */
    --album-track-cols: 38px minmax(0, 2.4fr) minmax(0, 1.4fr) 2rem 2rem 62px;
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

/* Phone: AlbumTrackRow hides its artist cell, so the shared template and this
   view's header row must drop the same track — otherwise every row misaligns.
   767.98px = $bp-phone-max - 0.02px (guarded by breakpoints.spec.ts). */
@media (max-width: 767.98px) {
    .track-list {
        --album-track-cols: 38px minmax(0, 1fr) 2rem 2rem 62px;
    }

    .track-list-header .col-artist {
        display: none;
    }
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

.cover-help {
    display: flex;
    align-items: center;
    justify-content: center;
    align-self: stretch;
    box-sizing: border-box;
    width: 100%;
    padding: 0.5rem 0.75rem;
    border: 1px solid var(--app-border);
    border-radius: var(--app-radius);
    background: var(--app-bg-subtle);
    color: var(--app-text-secondary);
    font-size: 0.85rem;
    line-height: 1.4;
    text-align: center;
}
</style>
