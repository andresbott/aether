<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { useRouter, onBeforeRouteLeave } from 'vue-router'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import ConfirmDialog from 'primevue/confirmdialog'
import { useToast } from 'primevue/usetoast'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import HeroHeader from '@/components/layout/HeroHeader.vue'
import HeroActions from '@/components/layout/HeroActions.vue'
import EditActionBar from '@/components/layout/EditActionBar.vue'
import TrackEditList from '@/components/layout/TrackEditList.vue'
import GenreTrackRow from '@/components/library/GenreTrackRow.vue'
import {
    usePlaylist,
    useUpdatePlaylist,
    useUpdatePlaylistCover,
    useDeletePlaylist,
    useReplacePlaylistTracks
} from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { useSongsDrag } from '@/composables/useSongsDrag'
import { useRowSelection } from '@/composables/useRowSelection'
import { reorderQueue } from '@/utils/queueReorder'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Song } from '@/types/subsonic'

const MAX_COVER_BYTES = 5 * 1024 * 1024

const props = defineProps<{ id: string }>()
const router = useRouter()
const player = usePlayer()
const toast = useToast()
const songsDrag = useSongsDrag()
const { isSelected, onRowClick, selectionForDrag, clearSelection } = useRowSelection()

const { data: playlist, isLoading, error } = usePlaylist(props.id)
const updatePlaylist = useUpdatePlaylist()
const updateCover = useUpdatePlaylistCover()
const deletePlaylist = useDeletePlaylist()
const replaceTracks = useReplacePlaylistTracks()

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

const seed = (): void => {
    const p = playlist.value
    if (!p) return
    working.value = [...(p.entry ?? [])]
    savedIds.value = (p.entry ?? []).map((s) => s.id)
    editName.value = baseName.value = p.name
    editComment.value = baseComment.value = p.comment ?? ''
}

const tracksDirty = computed(() => {
    const ids = working.value.map((s) => s.id)
    if (ids.length !== savedIds.value.length) return true
    return ids.some((id, i) => id !== savedIds.value[i])
})

const metaDirty = computed(
    () => editName.value !== baseName.value || editComment.value !== baseComment.value
)

const valid = computed(() => editName.value.trim().length > 0)

// --- Staged cover editing (persisted behind Save, alongside identity/track edits) ---
const selectedCoverFile = ref<File | null>(null)
const coverClear = ref(false)
const coverPreviewUrl = ref<string | null>(null)
const coverSizeError = ref<string | null>(null)
// Bumped after a cover save so the (otherwise unchanged) cover URL busts the
// browser cache and the hero shows the new image without a manual reload.
const coverCacheBust = ref(0)

const coverDirty = computed(() => selectedCoverFile.value !== null || coverClear.value)

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
}

const displayedCoverUrl = computed(() => {
    if (coverPreviewUrl.value) return coverPreviewUrl.value
    if (coverClear.value) return null
    if (playlist.value?.coverArt) {
        const base = subsonicClient.getCoverArtUrl(playlist.value.coverArt, 250)
        return coverCacheBust.value ? `${base}&cb=${coverCacheBust.value}` : base
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
    if (working.value.length) player.playAlbum(working.value)
}

const queueAll = (): void => {
    if (working.value.length) player.addMultipleToQueue(working.value)
}

// --- View-mode song list (album-style rows with a cover column) ---
const playFrom = (index: number): void => {
    player.playAlbum(working.value, index)
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

// --- Track edits (local until Save) ---
const onReorder = (indices: number[], target: number): void => {
    working.value = reorderQueue(working.value, indices, target)
}
const onDelete = (indices: number[]): void => {
    const drop = new Set(indices)
    working.value = working.value.filter((_, i) => !drop.has(i))
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
                    comment: editComment.value
                })
                .then(() => {
                    baseName.value = editName.value
                    baseComment.value = editComment.value
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
                    coverClear: coverClear.value || undefined
                })
                .then(() => {
                    resetCoverStaging()
                    coverCacheBust.value = Date.now()
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
            <template #actions>
                <EditActionBar
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

            <div class="playlist-scroll">
                <div class="playlist-body content-col">
                    <HeroHeader
                        eyebrow="Playlist"
                        :cover-url="displayedCoverUrl"
                        :cover-size-error="coverSizeError"
                        v-model:editing="editing"
                        @cover-select="onCoverSelect"
                        @cover-remove="onRemoveCover"
                    >
                        <template #read>
                            <h2 class="hero-name">{{ playlist.name }}</h2>
                            <p v-if="playlist.comment" class="hero-desc">{{ playlist.comment }}</p>
                            <div class="meta-row">
                                <span v-if="summary">{{ summary }}</span>
                                <span v-if="playlist.owner" :class="{ dot: !!summary }">
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
                            <small v-if="coverClear" class="cleared-note">
                                Cover will be reset on save.
                            </small>
                        </template>
                        <template #actions>
                            <HeroActions
                                :play-disabled="working.length === 0"
                                can-queue
                                @play="playAll"
                                @queue="queueAll"
                            />
                        </template>
                    </HeroHeader>

                    <!-- Edit mode: the reorderable/deletable editor (same component
                         as the queue's edit mode). View mode: the album-style table
                         extended with a cover column (shared with GenreDetailView). -->
                    <TrackEditList
                        v-if="editing && working.length > 0"
                        :songs="working"
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
                            @select="(p) => onRowClick(index, p)"
                            @play="playFrom(index)"
                            @dragstart="(e) => onRowDragStart(e, index)"
                            @dragend="songsDrag.end"
                        />
                    </div>
                    <div v-else class="empty-tracks">
                        <p>This playlist is empty</p>
                    </div>
                </div>
            </div>
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
.playlist-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    /* Recipe B: uniform rail clearance so the column matches the list views. */
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    box-sizing: border-box;
}
.playlist-body {
    padding-top: 1rem;
    padding-bottom: 1rem;
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
.field-label {
    font-size: 0.72rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--app-text-secondary);
}
.cleared-note {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
}
.empty-tracks {
    padding: 3rem;
    text-align: center;
    color: var(--app-text-secondary);
}

.track-list {
    /* Shared grid template so the header and every row (GenreTrackRow) align.
       Custom properties inherit through the DOM regardless of scoped styles. */
    --genre-track-cols: 48px minmax(0, 2fr) minmax(0, 1.2fr) minmax(0, 1.4fr) 62px;
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
</style>
