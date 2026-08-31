<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { useRouter, onBeforeRouteLeave } from 'vue-router'
import VirtualScroller from 'primevue/virtualscroller'
import type { VirtualScrollerLazyEvent } from 'primevue/virtualscroller'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import HeroHeader from '@/components/layout/HeroHeader.vue'
import HeroActions from '@/components/layout/HeroActions.vue'
import EditActionBar from '@/components/layout/EditActionBar.vue'
import GenreTrackRow from '@/components/library/GenreTrackRow.vue'
import TrackActionSheet from '@/components/library/TrackActionSheet.vue'
import { useGenres, useUpdateGenreCover } from '@/composables/useSubsonicQueries'
import { useGenreSongsTable, GENRE_SONG_PAGE_SIZE } from '@/composables/useGenreSongsTable'
import { usePlayer } from '@/composables/usePlayer'
import { useSongsDrag } from '@/composables/useSongsDrag'
import { useRowSelection } from '@/composables/useRowSelection'
import { subsonicClient } from '@/lib/api/subsonic'
import { bumpCoverVersion, versionedCoverUrl } from '@/composables/useCoverVersion'
import { useAuth } from '@/composables/useAuth'
import type { Song } from '@/types/subsonic'

const MAX_COVER_BYTES = 5 * 1024 * 1024
const GATHER_PAGE_SIZE = 500

const props = defineProps<{ name: string }>()
const router = useRouter()
const player = usePlayer()
const updateCover = useUpdateGenreCover()
const songsDrag = useSongsDrag()
const { isSelected, onRowClick, selectionForDrag, clearSelection } = useRowSelection()

// Genre cover art is global catalog data, so editing it is admin-only — the
// backend gates updateGenre on error 50. Match ArtistView/AlbumView and hide the
// whole edit affordance from non-admins rather than let it 403 on save.
const { isAdmin } = useAuth()

const actionSong = ref<Song | null>(null)
const actionIndex = ref(0)
const actionSheetOpen = ref(false)

// Touch tap-to-play: queue the list as shown and start at the tapped track, NOT
// `playNow`, which would wipe the queue down to that one song
// (see docs/architecture/unified-play-experience.md, "Touch contract").
//
// `items` is the SPARSE lazily-paged table: slots belonging to pages that have not
// been scrolled into view yet are holes, and usePlayer's queue is a dense Song[].
// So queue the entries that ARE loaded and start at the tapped song's position in
// that dense list — the song under the finger is always the one that plays.
// Scrolling on loads more pages but does not retro-fill the queue; gathering the
// complete genre stays the hero Play's job (it pages through getSongsByGenre).
const playTrack = (index: number): void => {
    const tapped = items.value[index]
    if (!tapped) return
    const loaded = items.value.filter((s): s is Song => s !== undefined)
    const start = loaded.indexOf(tapped)
    player.playAlbum(loaded, start === -1 ? 0 : start)
}

const openTrackMenu = (index: number): void => {
    const song = items.value[index]
    if (!song) return
    actionSong.value = song
    actionIndex.value = index
    actionSheetOpen.value = true
}

const { data: genres, isLoading, error } = useGenres()
const genre = computed(() => genres.value?.find((g) => g.value === props.name))
const songTotal = computed(() => genre.value?.songCount ?? 0)

const { items, ensureRange } = useGenreSongsTable(
    computed(() => props.name),
    songTotal
)

const currentTrackId = computed(() => player.currentTrack.value?.id)

// --- Cover editing (mirrors ArtistView: staged locally, applied on Save) ---
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
    if (!dirty.value || !genre.value?.coverArt) {
        editing.value = false
        return
    }
    updateCover.mutate(
        {
            genreId: genre.value.coverArt,
            coverFile: selectedFile.value ?? undefined,
            coverClear: coverClear.value || undefined
        },
        {
            onSuccess: () => {
                resetCoverStaging()
                // Shared, module-level version: a local ref would die with this
                // component, so navigating away and back would re-show the old
                // image from the browser's in-memory cache.
                if (genre.value?.coverArt) bumpCoverVersion(genre.value.coverArt)
                editing.value = false
            }
        }
    )
}

const cancelEdit = (): void => {
    resetCoverStaging()
    editing.value = false
}

const coverUrl = computed(() => {
    if (previewUrl.value) return previewUrl.value
    if (coverClear.value) return null
    if (!genre.value?.coverArt || !subsonicClient.isConfigured()) return null
    const base = subsonicClient.getCoverArtUrl(genre.value.coverArt, 250)
    return versionedCoverUrl(base, genre.value.coverArt)
})

// Unsaved-changes guards (mirror Artist/Playlist/Radio detail views).
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

// --- Hero meta + play/queue ---
const heroMeta = computed(() => {
    if (!genre.value) return []
    const parts: string[] = []
    const { albumCount, songCount } = genre.value
    if (albumCount) parts.push(`${albumCount} ${albumCount === 1 ? 'album' : 'albums'}`)
    if (songCount) parts.push(`${songCount} ${songCount === 1 ? 'song' : 'songs'}`)
    return parts
})

const gathering = ref(false)

// The lazy table only holds the pages scrolled into view, so gather the full
// song list on demand before playing/queuing the whole genre.
async function gatherSongs(): Promise<Song[]> {
    const songs: Song[] = []
    for (let offset = 0; ; offset += GATHER_PAGE_SIZE) {
        const page = await subsonicClient.getSongsByGenre(props.name, GATHER_PAGE_SIZE, offset)
        songs.push(...page)
        if (page.length < GATHER_PAGE_SIZE) break
    }
    return songs
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

// --- Song list interactions ---
function onLazyLoad(event: VirtualScrollerLazyEvent): void {
    void ensureRange(event.first, event.last)
}

// Double-clicking a row appends that song to the end of the queue rather than
// replacing it with the genre (see docs/architecture/unified-play-experience.md).
// Only the double-clicked row is needed, so the unfetched pages don't matter.
const enqueueTrack = (index: number): void => {
    const song = items.value[index]
    if (song) player.enqueueAndPlayIfIdle([song])
}

// A drag from a selected row carries the whole selection; from an unselected
// row it carries just that row.
const onRowDragStart = (event: DragEvent, index: number): void => {
    const songs = selectionForDrag(index)
        .map((i) => items.value[i])
        .filter((s): s is Song => s !== undefined)
    songsDrag.start(event, songs, event.currentTarget as HTMLElement)
}

// Discard the selection when navigating to a different genre.
watch(
    () => props.name,
    () => clearSelection()
)
</script>

<template>
    <div class="genre-view">
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="error" class="error">
            <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
            <p>{{ error.message }}</p>
        </div>

        <div v-else-if="!genre" class="error">
            <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
            <p>Genre not found</p>
        </div>

        <ContentScaffold v-else title="" show-back @back="router.back()">
            <template #actions>
                <EditActionBar
                    v-if="isAdmin"
                    v-model:editing="editing"
                    :can-delete="false"
                    :save-disabled="!dirty"
                    :saving="updateCover.isPending.value"
                    :dirty="dirty"
                    @save="saveEdit"
                    @cancel="cancelEdit"
                />
            </template>

            <div class="genre-body">
                <div class="genre-hero">
                    <div class="genre-hero-inner content-col">
                        <HeroHeader
                            eyebrow="Genre"
                            cover-placeholder-icon="pi pi-tags"
                            cover-back-label="Genre image"
                            :cover-url="coverUrl"
                            :cover-size-error="coverSizeError"
                            v-model:editing="editing"
                            @cover-select="onCoverSelect"
                            @cover-remove="onRemoveCover"
                        >
                            <template #read>
                                <h2 class="hero-name">{{ genre.value }}</h2>
                                <div v-if="heroMeta.length" class="meta-row">
                                    <span
                                        v-for="(part, i) in heroMeta"
                                        :key="part"
                                        :class="{ dot: i > 0 }"
                                        >{{ part }}</span
                                    >
                                </div>
                            </template>
                            <template #actions>
                                <HeroActions
                                    :play-disabled="songTotal === 0"
                                    can-queue
                                    :busy="gathering"
                                    @play="onPlay"
                                    @queue="onQueue"
                                />
                            </template>
                        </HeroHeader>
                    </div>
                </div>

                <div v-if="songTotal > 0" class="track-list">
                    <div class="track-list-header">
                        <div class="track-list-header-row">
                            <span class="col-cover"></span>
                            <span class="col-title">Title</span>
                            <span class="col-artist">Artist</span>
                            <span class="col-album">Album</span>
                            <!-- The select and favorite columns are hover-revealed
                                 per row, so their headers stay blank rather than
                                 labelling controls that are usually invisible. -->
                            <span class="col-select"></span>
                            <span class="col-star"></span>
                            <span class="col-duration" aria-label="Duration">
                                <i class="pi pi-clock"></i>
                            </span>
                        </div>
                    </div>
                    <VirtualScroller
                        :items="items"
                        :itemSize="56"
                        lazy
                        :numToleratedItems="10"
                        class="track-scroller"
                        @lazy-load="onLazyLoad"
                    >
                        <template #item="{ item, options }">
                            <GenreTrackRow
                                :song="item"
                                :index="options.index"
                                :selected="isSelected(options.index)"
                                :playing="item?.id === currentTrackId"
                                @select="(p) => onRowClick(options.index, p)"
                                @enqueue="enqueueTrack(options.index)"
                                @play="playTrack(options.index)"
                                @menu="openTrackMenu(options.index)"
                                @dragstart="(e) => onRowDragStart(e, options.index)"
                                @dragend="songsDrag.end"
                            />
                        </template>
                    </VirtualScroller>
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
.genre-view {
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

/* Hero + header stay fixed; only the track list scrolls (virtualized). */
.genre-body {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
}

/* Recipe A: the hero is a fixed frame above the scrolling track list. */
.genre-hero {
    flex-shrink: 0;
    box-sizing: border-box;
    padding-right: calc(var(--app-rail-clearance) + 2 * var(--sb-w, 0px));
}

.genre-hero-inner {
    padding-top: 1rem;
}

.track-list {
    /* Shared grid template so the header and every row align. Custom properties
       inherit through the DOM regardless of scoped styles. */
    --genre-track-cols: 48px minmax(0, 2fr) minmax(0, 1.2fr) minmax(0, 1.4fr) 2rem 2rem 62px;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
}

.track-list-header {
    box-sizing: border-box;
    padding-left: var(--app-content-gutter);
    padding-right: calc(var(--app-rail-clearance) + 2 * var(--sb-w, 0px) + var(--app-content-gutter));
}

.track-list-header-row {
    display: grid;
    grid-template-columns: var(--genre-track-cols);
    column-gap: 0.75rem;
    padding: 0 0.5rem 0.4rem;
    border-bottom: 1px solid var(--app-border);
    margin: 0 auto 0.25rem;
    max-width: var(--app-content-max-width);
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--app-text-secondary);
}

.track-list-header .col-duration {
    text-align: right;
}

/* Phone: GenreTrackRow hides its artist and album cells, so the shared template
   and this view's header row must drop the same two tracks — otherwise every row
   misaligns. 767.98px = $bp-phone-max - 0.02px (guarded by breakpoints.spec.ts). */
@media (max-width: 767.98px) {
    .track-list {
        --genre-track-cols: 48px minmax(0, 1fr) 2rem 2rem 62px;
    }

    .track-list-header .col-artist,
    .track-list-header .col-album {
        display: none;
    }
}

.track-scroller {
    flex: 1;
    min-height: 0;
    width: 100%;
    scrollbar-gutter: stable;
}

.track-list :deep(.p-virtualscroller-content) {
    box-sizing: border-box;
    padding-left: var(--app-content-gutter);
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px) + var(--app-content-gutter));
}

/* Center the rows in the shared content column; the scroller stays full width
   so its scroll bar stays flush right. */
.track-list :deep(.genre-track-row) {
    max-width: var(--app-content-max-width);
    margin-left: auto;
    margin-right: auto;
}
</style>
