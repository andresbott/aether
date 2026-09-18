<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { useRouter, onBeforeRouteLeave } from 'vue-router'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import ConfirmDialog from 'primevue/confirmdialog'
import { useToast } from 'primevue/usetoast'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import HeroHeader from '@/components/layout/HeroHeader.vue'
import HeroActions from '@/components/layout/HeroActions.vue'
import HeroSelectionBar from '@/components/layout/HeroSelectionBar.vue'
import HeroEditSelectionBar from '@/components/layout/HeroEditSelectionBar.vue'
import HeroVisibilityBar from '@/components/layout/HeroVisibilityBar.vue'
import EditActionBar from '@/components/layout/EditActionBar.vue'
import TrackEditList from '@/components/layout/TrackEditList.vue'
import GenreTrackRow from '@/components/library/GenreTrackRow.vue'
import TrackActionSheet from '@/components/library/TrackActionSheet.vue'
import GenerateCoverDialog from '@/components/cover/GenerateCoverDialog.vue'
import {
    usePlaylist,
    useUpdatePlaylist,
    useUpdatePlaylistCover,
    useDeletePlaylist,
    useReplacePlaylistTracks,
    useTogglePlaylistStar
} from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { useIsPlaylistOwner } from '@/composables/usePlaylistOwnership'
import { useSongsDrag } from '@/composables/useSongsDrag'
import { useRowSelection } from '@/composables/useRowSelection'
import { reorderQueue } from '@/utils/queueReorder'
import { subsonicClient } from '@/lib/api/subsonic'
import { bumpCoverVersion, versionedCoverUrl } from '@/composables/useCoverVersion'
import type { Song } from '@/types/subsonic'

const MAX_COVER_BYTES = 5 * 1024 * 1024

const props = defineProps<{ id: string }>()
const router = useRouter()
const player = usePlayer()
const toast = useToast()
const songsDrag = useSongsDrag()
// One selection instance, shared with the edit list (via TrackEditList's
// `selection` prop) so the same clicks drive both the view-mode rows and the
// in-hero edit bubble.
const rowSelection = useRowSelection()
const { isSelected, onRowClick, selectionForDrag, clearSelection, selectedCount, selectedIndices } =
    rowSelection

const actionSong = ref<Song | null>(null)
const actionIndex = ref(0)
const actionSheetOpen = ref(false)

// Touch tap-to-play: queue the playlist as shown (the live `working` list, so an
// unsaved reorder plays in the order on screen) and start at the tapped track.
// NOT `playNow`, which would wipe the queue down to that one song
// (see docs/architecture/unified-play-experience.md, "Touch contract").
const playTrack = (index: number): void => {
    const songs = working.value
    if (songs[index]) player.playAlbum(songs, index)
}

const openTrackMenu = (index: number): void => {
    const song = working.value[index]
    if (!song) return
    actionSong.value = song
    actionIndex.value = index
    actionSheetOpen.value = true
}

const { data: playlist, isLoading, error } = usePlaylist(props.id)

// Only the owner may edit a playlist; the backend rejects a foreign write with
// Subsonic error 50, so offering the edit UI just fails at save.
const isOwner = useIsPlaylistOwner(() => playlist.value?.owner)
const updatePlaylist = useUpdatePlaylist()
const updateCover = useUpdatePlaylistCover()
const deletePlaylist = useDeletePlaylist()
const replaceTracks = useReplacePlaylistTracks()
const toggleStar = useTogglePlaylistStar()

// Edit mode covers the hero (name + description + cover) and the track list,
// which swaps from playable rows to the reorderable/deletable editor.
const editing = ref(false)

// `working` is the live track list edit mode mutates; `savedIds` is the
// last-persisted order. Combined with the staged name/description and cover
// state below, `dirty` gates the Save button and the unsaved-changes guards.
const working = ref<Song[]>([])
const savedIds = ref<string[]>([])

// Staged identity edits, baselined against the last-persisted values (mirrors the
// tracks working/savedIds pattern so the initial empty state reads as clean).
const editName = ref('')
const editComment = ref('')
const baseName = ref('')
const baseComment = ref('')
// Staged visibility (the `public` shared-read flag), baselined like name/comment.
const editPublic = ref(false)
const basePublic = ref(false)

const seed = (): void => {
    const p = playlist.value
    if (!p) return
    working.value = [...(p.entry ?? [])]
    savedIds.value = (p.entry ?? []).map((s) => s.id)
    editName.value = baseName.value = p.name
    editComment.value = baseComment.value = p.comment ?? ''
    editPublic.value = basePublic.value = p.public ?? false
}

const tracksDirty = computed(() => {
    const ids = working.value.map((s) => s.id)
    if (ids.length !== savedIds.value.length) return true
    return ids.some((id, i) => id !== savedIds.value[i])
})

const metaDirty = computed(
    () =>
        editName.value !== baseName.value ||
        editComment.value !== baseComment.value ||
        editPublic.value !== basePublic.value
)

const valid = computed(() => editName.value.trim().length > 0)

// --- Staged cover editing (persisted behind Save, alongside identity/track edits) ---
const selectedCoverFile = ref<File | null>(null)
const coverClear = ref(false)
const coverPreviewUrl = ref<string | null>(null)
const coverSizeError = ref<string | null>(null)
const showGenerate = ref(false)
const stagedPick = ref<{ style: string; variation: number } | null>(null)

const coverDirty = computed(
    () => selectedCoverFile.value !== null || coverClear.value || stagedPick.value !== null
)

const dirty = computed(() => tracksDirty.value || coverDirty.value || metaDirty.value)

const savePending = computed(
    () =>
        replaceTracks.isPending.value ||
        updateCover.isPending.value ||
        updatePlaylist.isPending.value
)

function resetCoverStaging(): void {
    if (coverPreviewUrl.value) URL.revokeObjectURL(coverPreviewUrl.value)
    coverPreviewUrl.value = null
    selectedCoverFile.value = null
    coverClear.value = false
    coverSizeError.value = null
    stagedPick.value = null
}

const displayedCoverUrl = computed(() => {
    if (coverPreviewUrl.value) return coverPreviewUrl.value
    if (coverClear.value) return null
    if (playlist.value?.coverArt) {
        const base = subsonicClient.getCoverArtUrl(playlist.value.coverArt, 250)
        return versionedCoverUrl(base, playlist.value.coverArt)
    }
    return null
})

// Seed from the server list on load. Skip while dirty so a background refetch
// never clobbers unsaved edits.
watch(
    () => playlist.value,
    () => {
        if (playlist.value && !dirty.value) seed()
    },
    { immediate: true }
)

const summary = computed(() => {
    const n = working.value.length
    if (n === 0) return ''
    const parts = [`${n} ${n === 1 ? 'song' : 'songs'}`]
    const totalSec = working.value.reduce((sum, s) => sum + (s.duration || 0), 0)
    if (totalSec) parts.push(`${Math.floor(totalSec / 60)} min`)
    return parts.join(' • ')
})

const playAll = (): void => {
    if (!working.value.length) return
    player.playAlbum(working.value)
    // Counted per playlist, so the play site records it — usePlayer only sees
    // a flat Song[].
    if (playlist.value) void subsonicClient.scrobble(playlist.value.id)
}

const queueAll = (): void => {
    if (working.value.length) player.addMultipleToQueue(working.value)
}

const onStar = (): void => {
    if (!playlist.value) return
    toggleStar.mutate({ id: playlist.value.id, starred: !!playlist.value.starred })
}

// The selected songs in list order, fed to the in-hero selection action row.
const selectedSongs = computed(() =>
    [...selectedIndices.value]
        .sort((a, b) => a - b)
        .map((i) => working.value[i])
        .filter((s): s is Song => s !== undefined)
)

const playSelection = (): void => {
    if (selectedSongs.value.length) player.playAlbum(selectedSongs.value)
}
const queueSelection = (): void => {
    if (selectedSongs.value.length) player.addMultipleToQueue(selectedSongs.value)
}

// --- View-mode song list (album-style rows with a cover column) ---
// Double-clicking a row appends that song to the end of the queue rather than
// replacing it with the playlist (see docs/architecture/unified-play-experience.md).
const enqueueTrack = (index: number): void => {
    const song = working.value[index]
    if (song) player.enqueueAndPlayIfIdle([song])
}

// A drag from a selected row carries the whole selection; from an unselected
// row it carries just that row.
const onRowDragStart = (event: DragEvent, index: number): void => {
    const songs = selectionForDrag(index)
        .map((i) => working.value[i])
        .filter((s): s is Song => s !== undefined)
    songsDrag.start(event, songs, event.currentTarget as HTMLElement)
}

// --- Cover picker ---
const onCoverSelect = (file: File): void => {
    if (file.size > MAX_COVER_BYTES) {
        coverSizeError.value = `File is ${(file.size / 1024 / 1024).toFixed(1)} MB — max is 5 MB`
        return
    }
    coverSizeError.value = null
    if (coverPreviewUrl.value) URL.revokeObjectURL(coverPreviewUrl.value)
    selectedCoverFile.value = file
    coverPreviewUrl.value = URL.createObjectURL(file)
    coverClear.value = false
}

const onRemoveCover = (): void => {
    if (coverPreviewUrl.value) URL.revokeObjectURL(coverPreviewUrl.value)
    coverPreviewUrl.value = null
    selectedCoverFile.value = null
    coverClear.value = true
}

const onGeneratePick = (pick: { style: string; variation: number }): void => {
    stagedPick.value = pick
    selectedCoverFile.value = null
    if (coverPreviewUrl.value) URL.revokeObjectURL(coverPreviewUrl.value)
    coverPreviewUrl.value = subsonicClient.getGeneratedCoverPreviewUrl({
        id: props.id,
        style: pick.style,
        variation: pick.variation,
        size: 512
    })
    coverClear.value = false
}

// --- Track edits (local until Save) ---
const onReorder = (indices: number[], target: number): void => {
    working.value = reorderQueue(working.value, indices, target)
}
const onDelete = (indices: number[]): void => {
    const drop = new Set(indices)
    working.value = working.value.filter((_, i) => !drop.has(i))
}

// The in-hero edit bubble's actions, run against the current selection. Each
// clears the selection afterwards — the moved/removed indices no longer point at
// the same rows (mirrors the drag reorder + delete in TrackEditList).
const selectedEditIndices = computed(() => [...selectedIndices.value].sort((a, b) => a - b))
const moveSelectionToTop = (): void => {
    if (!selectedEditIndices.value.length) return
    onReorder(selectedEditIndices.value, 0)
    clearSelection()
}
const moveSelectionToBottom = (): void => {
    if (!selectedEditIndices.value.length) return
    onReorder(selectedEditIndices.value, working.value.length)
    clearSelection()
}
const deleteSelection = (): void => {
    if (!selectedEditIndices.value.length) return
    onDelete(selectedEditIndices.value)
    clearSelection()
}
const saveEdit = async (): Promise<void> => {
    if (!dirty.value) {
        editing.value = false
        return
    }
    const tasks: Promise<unknown>[] = []
    if (metaDirty.value) {
        tasks.push(
            updatePlaylist
                .mutateAsync({
                    playlistId: props.id,
                    name: editName.value.trim(),
                    comment: editComment.value,
                    public: editPublic.value
                })
                .then(() => {
                    baseName.value = editName.value
                    baseComment.value = editComment.value
                    basePublic.value = editPublic.value
                })
        )
    }
    if (tracksDirty.value) {
        tasks.push(
            replaceTracks
                .mutateAsync({ playlistId: props.id, songIds: working.value.map((s) => s.id) })
                .then(() => {
                    // Re-baseline immediately so the button disables without waiting
                    // for the invalidated query to refetch.
                    savedIds.value = working.value.map((s) => s.id)
                })
        )
    }
    if (coverDirty.value) {
        tasks.push(
            updateCover
                .mutateAsync({
                    playlistId: props.id,
                    coverFile: selectedCoverFile.value ?? undefined,
                    coverClear: coverClear.value || undefined,
                    generate: stagedPick.value ?? undefined
                })
                .then(() => {
                    resetCoverStaging()
                    // Shared, module-level version: a local ref would die with
                    // this component, so navigating away and back would re-show
                    // the old image from the browser's in-memory cache.
                    if (playlist.value?.coverArt) bumpCoverVersion(playlist.value.coverArt)
                })
        )
    }
    try {
        await Promise.all(tasks)
        editing.value = false
    } catch (err) {
        // Stay in edit mode so the user can retry; successful slices were already
        // re-baselined above, so a retry only re-fires the still-dirty ones.
        toast.add({
            severity: 'error',
            summary: 'Save failed',
            detail: (err as Error)?.message || 'Some changes could not be saved. Please try again.',
            life: 5000
        })
    }
}

// Discard staged name/description/cover edits and leave edit mode.
const cancelEdit = (): void => {
    resetCoverStaging()
    seed()
    editing.value = false
}

const handleDelete = (): void => {
    deletePlaylist.mutate(props.id, {
        onSuccess: () => router.push({ name: 'playlists' })
    })
}

// Switching playlists discards any local edits/cover draft and reseeds.
const resetOnIdChange = (): void => {
    editing.value = false
    resetCoverStaging()
    clearSelection()
    seed()
}
watch(() => props.id, resetOnIdChange)

// Toggling edit mode in EITHER direction drops any selection. Entering swaps the
// track list for the reorder editor; leaving must reset it too (Esc/Cancel/Save),
// or the carried-over selection resurfaces as the view-mode selection bar and
// leaves rows highlighted once the hero's action row returns.
watch(editing, () => clearSelection())

// --- Unsaved-changes guards ---
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
    if (coverPreviewUrl.value) URL.revokeObjectURL(coverPreviewUrl.value)
})
</script>

<template>
    <div class="playlist-detail-view">
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="error" class="error">
            <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
            <p>{{ error.message }}</p>
        </div>

        <ContentScaffold v-else-if="playlist" title="" show-back @back="router.back()">
            <div class="playlist-scroll">
                <HeroHeader
                    class="detail-hero"
                    :eyebrow="editing ? '' : 'Playlist'"
                    :cover-url="displayedCoverUrl"
                    :cover-size-error="coverSizeError"
                    v-model:editing="editing"
                    @cover-select="onCoverSelect"
                    @cover-remove="onRemoveCover"
                >
                    <template #edit-actions>
                        <EditActionBar
                            v-if="isOwner"
                            v-model:editing="editing"
                            :save-disabled="savePending || !valid"
                            :saving="savePending"
                            :dirty="dirty"
                            delete-header="Delete playlist?"
                            :delete-message="`Delete playlist &quot;${playlist.name}&quot;? This cannot be undone.`"
                            @save="saveEdit"
                            @cancel="cancelEdit"
                            @delete="handleDelete"
                        />
                    </template>
                    <template v-if="isOwner" #cover-actions>
                        <Button label="Generate" icon="pi pi-sparkles" outlined @click="showGenerate = true" />
                    </template>
                    <template #read>
                        <h2 class="hero-name">{{ playlist.name }}</h2>
                        <p v-if="playlist.comment" class="hero-desc">{{ playlist.comment }}</p>
                        <div class="meta-row">
                            <span v-if="summary">{{ summary }}</span>
                            <span v-if="playlist.owner" :class="{ dot: !!summary }">
                                <i
                                    v-if="!isOwner"
                                    class="pi pi-lock not-mine-icon"
                                    aria-hidden="true"
                                    v-tooltip.bottom="`Shared by ${playlist.owner} — view only`"
                                ></i>
                                by {{ playlist.owner }}
                            </span>
                        </div>
                        <small v-if="coverClear" class="cleared-note">
                            Cover will be reset on save.
                        </small>
                    </template>
                    <template #edit>
                        <label class="form-field">
                            <span class="field-label">Name</span>
                            <InputText v-model="editName" maxlength="60" />
                        </label>
                        <label class="form-field">
                            <span class="field-label">Description</span>
                            <Textarea v-model="editComment" rows="3" autoResize />
                        </label>
                        <!-- Selecting rows in the reorder list below reveals this
                             bubble in the band, below the form (like the album
                             selection bar, but with the editor's reorder/delete
                             actions). -->
                        <HeroEditSelectionBar
                            v-if="selectedCount > 0"
                            :count="selectedCount"
                            @move-top="moveSelectionToTop"
                            @move-bottom="moveSelectionToBottom"
                            @delete="deleteSelection"
                            @clear="clearSelection"
                        />
                        <!-- With nothing selected, the same slot holds the
                             Public/Private toggle instead of an empty gap. -->
                        <HeroVisibilityBar v-else v-model="editPublic" />
                        <small v-if="coverClear" class="cleared-note">
                            Cover will be reset on save.
                        </small>
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
                            :play-disabled="working.length === 0"
                            can-queue
                            can-star
                            :starred="!!playlist?.starred"
                            @play="playAll"
                            @queue="queueAll"
                            @star="onStar"
                        />
                    </template>
                </HeroHeader>

                <div class="playlist-below">
                    <div class="playlist-body content-col">
                        <!-- Edit mode: the reorderable/deletable editor (same component
                             as the queue's edit mode). View mode: the album-style table
                             extended with a cover column (shared with GenreDetailView). -->
                        <TrackEditList
                            v-if="editing && working.length > 0"
                            :songs="working"
                            :selection="rowSelection"
                            delete-label="Remove from playlist"
                            group="playlist"
                            @reorder="onReorder"
                            @delete="onDelete"
                        />
                        <div v-else-if="working.length > 0" class="track-list">
                            <div class="track-list-header">
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
                            <GenreTrackRow
                                v-for="(song, index) in working"
                                :key="song.id + ':' + index"
                                :song="song"
                                :index="index"
                                :selected="isSelected(index)"
                                :selecting="selectedCount > 0"
                                @select="(p) => onRowClick(index, p)"
                                @enqueue="enqueueTrack(index)"
                                @play="playTrack(index)"
                                @menu="openTrackMenu(index)"
                                @dragstart="(e) => onRowDragStart(e, index)"
                                @dragend="songsDrag.end"
                            />
                        </div>
                        <div v-else class="empty-tracks">
                            <p>This playlist is empty</p>
                        </div>
                    </div>
                </div>
            </div>
            <TrackActionSheet
                v-model:visible="actionSheetOpen"
                :song="actionSong"
                @play="playTrack(actionIndex)"
            />
            <GenerateCoverDialog
                v-model:visible="showGenerate"
                :entity-id="props.id"
                :title="playlist?.name"
                @select="onGeneratePick"
            />
        </ContentScaffold>

        <ConfirmDialog />
    </div>
</template>

<style scoped>
.playlist-detail-view {
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
/* Recipe B with the hero pulled out of the column so the duotone band bleeds
   edge to edge: the scroll container no longer reserves the rail clearance
   itself; the hero (internally) and .playlist-below each reserve it, so the two
   columns still line up. */
.playlist-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    box-sizing: border-box;
}
.playlist-below {
    box-sizing: border-box;
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
}
.playlist-body {
    padding-top: 1rem;
    padding-bottom: 1rem;
}

/* Edit mode: pull the name/description form up (the shared hero reserves 3rem of
   top pad to clear the Save/Cancel trio; the playlist form only needs enough to
   sit below it). The reclaimed height lets the selection bubble drop into the
   space beside the cover instead of pushing the band taller when songs are
   selected. */
.detail-hero.editing :deep(.hero-info) {
    padding-top: 1.75rem;
}

/* Tighten the form stack a touch so the name/description + the selection bubble
   fit within the cover's height — the band stays cover-sized when songs are
   selected instead of growing. */
.detail-hero.editing :deep(.edit-only) {
    gap: 0.45rem;
}

.form-field {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
}
.form-field :deep(.p-inputtext),
.form-field :deep(textarea) {
    width: 100%;
}
/* The edit form sits on the dark duotone band, so its labels take a light ink. */
.field-label {
    font-size: 0.72rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: #cdd7df;
}
.cleared-note {
    color: #cdd7df;
    font-size: 0.85rem;
}
/* Read-only marker on the meta row for a playlist owned by someone else. */
.not-mine-icon {
    margin-right: 0.3rem;
    font-size: 0.85em;
    opacity: 0.85;
}
.empty-tracks {
    padding: 3rem;
    text-align: center;
    color: var(--app-text-secondary);
}

.track-list {
    /* Shared grid template so the header and every row (GenreTrackRow) align.
       Custom properties inherit through the DOM regardless of scoped styles. */
    --genre-track-cols: 48px minmax(0, 2fr) minmax(0, 1.2fr) minmax(0, 1.4fr) 2rem 2rem 62px;
    display: flex;
    flex-direction: column;
}

.track-list-header {
    display: grid;
    grid-template-columns: var(--genre-track-cols);
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
</style>
