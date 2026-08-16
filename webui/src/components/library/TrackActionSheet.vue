<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Drawer from 'primevue/drawer'
import { useToast } from 'primevue/usetoast'
import { usePlayer } from '@/composables/usePlayer'
import { useSongFavorite } from '@/composables/useSongFavorite'
import { usePlaylists, useUpdatePlaylist } from '@/composables/useSubsonicQueries'
import { apiErrorMessage } from '@/lib/apiError'
import type { Song } from '@/types/subsonic'

// The touch counterpart of the desktop hover affordances: one bottom sheet per
// view, fed the row whose ⋮ was tapped (see unified-play-experience.md).
const props = defineProps<{ song: Song | null; visible: boolean }>()
const emit = defineEmits<{
    (e: 'update:visible', value: boolean): void
    // Play this row via the host's tap-to-play primitive. Rows are tap
    // targets, not tab stops, so the sheet — reached through the tabbable ⋮ —
    // is the keyboard path to starting playback at a specific track.
    (e: 'play'): void
}>()

const router = useRouter()
const player = usePlayer()
const toast = useToast()
const { isStarred, toggleFavorite } = useSongFavorite(() => props.song)

// Every host view mounts this sheet, but only touch can open it: ungated, the
// playlist picker's getPlaylists cost one request per desktop view visit for a face
// nobody there can reach. Latched, not tied to `visible`, so the list is fetched
// once and then stays warm across opens instead of refetching on each one.
const hasOpened = ref(false)
const { data: playlistsData, isLoading: playlistsLoading } = usePlaylists({
    enabled: computed(() => hasOpened.value)
})
const updatePlaylist = useUpdatePlaylist()

const playlists = computed(() => playlistsData.value ?? [])

// Two faces: actions, and the playlist picker. Reset on every open.
const face = ref<'actions' | 'playlists'>('actions')
watch(
    () => props.visible,
    (open) => {
        if (!open) return
        face.value = 'actions'
        // The fetch starts here rather than on reaching the picker, so the list is
        // usually there by the time it renders; the empty face covers the rest.
        hasOpened.value = true
    }
)

const close = (): void => emit('update:visible', false)

const onPlay = (): void => {
    if (!props.song) return
    emit('play')
    close()
}

const onQueue = (): void => {
    if (props.song) player.addToQueue(props.song)
    close()
}

const onFavorite = (): void => {
    if (!props.song) return
    toggleFavorite()
    close()
}

const onAlbum = (): void => {
    const id = props.song?.albumId
    if (!id) return
    close()
    void router.push({ name: 'album', params: { id } })
}

const onArtist = (): void => {
    const id = props.song?.artistId
    if (!id) return
    close()
    void router.push({ name: 'artist', params: { id } })
}

// Adding leaves no trace on screen — the sheet closes and the playlist is
// elsewhere — so both outcomes are toasted, or a failed write looks like a
// successful one.
const onPickPlaylist = (playlistId: string, playlistName: string): void => {
    const song = props.song
    if (!song) return
    updatePlaylist.mutate(
        { playlistId, songIdsToAdd: [song.id] },
        {
            onSuccess: () => {
                toast.add({
                    severity: 'success',
                    summary: `Added to ${playlistName}`,
                    detail: song.title,
                    life: 3000
                })
            },
            onError: (err: unknown) => {
                toast.add({
                    severity: 'error',
                    summary: 'Failed to add to playlist',
                    detail: apiErrorMessage(err),
                    life: 5000
                })
            }
        }
    )
    close()
}

const favoriteLabel = computed(() =>
    isStarred.value ? 'Remove from favorites' : 'Add to favorites'
)
</script>

<template>
    <Drawer
        :visible="props.visible"
        position="bottom"
        class="track-action-sheet app-bottom-sheet"
        :header="song?.title ?? ''"
        @update:visible="emit('update:visible', $event)"
    >
        <nav v-if="face === 'actions'" class="sheet-actions" aria-label="Track actions">
            <button type="button" class="sheet-action" @click="onPlay">
                <i class="pi pi-play"></i>
                <span>Play</span>
            </button>
            <button type="button" class="sheet-action" @click="onQueue">
                <i class="pi pi-plus"></i>
                <span>Add to queue</span>
            </button>
            <button type="button" class="sheet-action" @click="onFavorite">
                <i :class="isStarred ? 'pi pi-heart-fill' : 'pi pi-heart'"></i>
                <span>{{ favoriteLabel }}</span>
            </button>
            <button type="button" class="sheet-action" @click="face = 'playlists'">
                <i class="pi pi-list"></i>
                <span>Add to playlist</span>
            </button>
            <button v-if="song?.albumId" type="button" class="sheet-action" @click="onAlbum">
                <i class="pi pi-clone"></i>
                <span>Go to album</span>
            </button>
            <button v-if="song?.artistId" type="button" class="sheet-action" @click="onArtist">
                <i class="pi pi-user"></i>
                <span>Go to artist</span>
            </button>
        </nav>

        <nav v-else class="sheet-actions" aria-label="Pick a playlist">
            <button type="button" class="sheet-action sheet-back" @click="face = 'actions'">
                <i class="pi pi-chevron-left"></i>
                <span>Back</span>
            </button>
            <button
                v-for="playlist in playlists"
                :key="playlist.id"
                type="button"
                class="sheet-action"
                @click="onPickPlaylist(playlist.id, playlist.name)"
            >
                <i class="pi pi-list"></i>
                <span>{{ playlist.name }}</span>
            </button>
            <!-- The fetch is gated on the first open (see script), so the very first
                 visit to this face can land before the list does. Saying so beats
                 claiming there are no playlists. -->
            <p v-if="playlists.length === 0" class="sheet-empty">
                {{ playlistsLoading ? 'Loading playlists…' : 'No playlists' }}
            </p>
        </nav>
    </Drawer>
</template>

<style scoped>
.sheet-actions {
    display: flex;
    flex-direction: column;
}

/* Full-width tap rows: icon, label, nothing else — the touch counterpart of a
   hover affordance has to be thumb-sized, so the row height comes from padding
   rather than a font size. */
.sheet-action {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.85rem 0.5rem;
    border: none;
    background: none;
    cursor: pointer;
    color: var(--app-text-primary);
    font-size: 0.95rem;
    text-align: left;
    width: 100%;
    border-radius: var(--app-radius);
}

.sheet-action:hover {
    background-color: var(--app-hover);
}

.sheet-back {
    color: var(--app-text-secondary);
}

.sheet-empty {
    margin: 0.5rem;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
}
</style>
