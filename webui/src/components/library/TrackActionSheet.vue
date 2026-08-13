<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Drawer from 'primevue/drawer'
import { usePlayer } from '@/composables/usePlayer'
import { useSongFavorite } from '@/composables/useSongFavorite'
import { usePlaylists, useUpdatePlaylist } from '@/composables/useSubsonicQueries'
import type { Song } from '@/types/subsonic'

// The touch counterpart of the desktop hover affordances: one bottom sheet per
// view, fed the row whose ⋮ was tapped (see unified-play-experience.md).
const props = defineProps<{ song: Song | null; visible: boolean }>()
const emit = defineEmits<{ (e: 'update:visible', value: boolean): void }>()

const router = useRouter()
const player = usePlayer()
const { isStarred, toggleFavorite } = useSongFavorite(() => props.song)
const { data: playlistsData } = usePlaylists()
const updatePlaylist = useUpdatePlaylist()

const playlists = computed(() => playlistsData.value ?? [])

// Two faces: actions, and the playlist picker. Reset on every open.
const face = ref<'actions' | 'playlists'>('actions')
watch(
    () => props.visible,
    (open) => {
        if (open) face.value = 'actions'
    }
)

const close = (): void => emit('update:visible', false)

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

const onPickPlaylist = (playlistId: string): void => {
    const song = props.song
    if (!song) return
    updatePlaylist.mutate({ playlistId, songIdsToAdd: [song.id] })
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
        class="track-action-sheet"
        :header="song?.title ?? ''"
        @update:visible="emit('update:visible', $event)"
    >
        <nav v-if="face === 'actions'" class="sheet-actions" aria-label="Track actions">
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
                @click="onPickPlaylist(playlist.id)"
            >
                <i class="pi pi-list"></i>
                <span>{{ playlist.name }}</span>
            </button>
            <p v-if="playlists.length === 0" class="sheet-empty">No playlists</p>
        </nav>
    </Drawer>
</template>

<style scoped>
.sheet-actions {
    display: flex;
    flex-direction: column;
}

/* Same row anatomy as MobileMoreDrawer's .drawer-item, kept separate because
   this sheet lives in the library domain, not the app chrome. */
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
