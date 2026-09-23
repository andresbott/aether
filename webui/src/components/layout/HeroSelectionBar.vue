<script setup lang="ts">
import { computed, ref } from 'vue'
import Popover from 'primevue/popover'
import InputText from 'primevue/inputtext'
import Button from 'primevue/button'
import { useToast } from 'primevue/usetoast'
import {
    usePlaylists,
    useUpdatePlaylist,
    useCreatePlaylist,
    useStarSongs
} from '@/composables/useSubsonicQueries'
import { subsonicClient } from '@/lib/api/subsonic'
import { apiErrorMessage } from '@/lib/apiError'
import type { Playlist, Song } from '@/types/subsonic'

// The in-hero action row, retargeted to the current multi-selection. It replaces
// the item's own Play/Queue/Favorite while any track rows are selected, using the
// SAME button chrome (no distinct pill) so switching in and out never shifts the
// hero identity. Vocabulary mirrors TrackActionSheet: Play · Add to queue · Add
// to playlist (→ pick which) · Add to favorites · Clear.
const props = defineProps<{
    count: number
    songs: Song[]
}>()

const emit = defineEmits<{
    (e: 'play'): void
    (e: 'queue'): void
    (e: 'clear'): void
}>()

const toast = useToast()
const songIds = computed(() => props.songs.map((s) => s.id))
const songWord = (n: number): string => (n === 1 ? 'song' : 'songs')
const countLabel = computed(() => `${props.count} ${songWord(props.count)}`)

// --- Add to favorites (always stars; the label names one direction) ---
const starSongs = useStarSongs()
const favorite = (): void => {
    if (!songIds.value.length) return
    const n = props.count
    starSongs.mutate(songIds.value, {
        onSuccess: () => {
            toast.add({
                severity: 'success',
                summary: 'Added to favorites',
                detail: `${n} ${songWord(n)}`,
                life: 3000
            })
        },
        onError: (err: unknown) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to add to favorites',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}

// --- Add to playlist picker (anchored under its button) ---
const picker = ref<InstanceType<typeof Popover> | null>(null)
const pickerOpen = ref(false)
// Face inside the popover: pick an existing playlist, or name a new one.
const face = ref<'list' | 'new'>('list')
const query = ref('')
const newName = ref('')

// Latched like TrackActionSheet's: getPlaylists is only fetched once the picker
// has been opened, then stays warm across opens instead of refetching each time.
const hasOpened = ref(false)
const { data: playlistsData, isLoading: playlistsLoading } = usePlaylists({
    enabled: computed(() => hasOpened.value)
})
const playlists = computed(() => playlistsData.value ?? [])

const filtered = computed(() => {
    const q = query.value.trim().toLowerCase()
    if (!q) return playlists.value
    return playlists.value.filter((p) => p.name.toLowerCase().includes(q))
})

const togglePicker = (event: Event): void => {
    hasOpened.value = true
    face.value = 'list'
    query.value = ''
    newName.value = ''
    picker.value?.toggle(event)
}

const playlistCover = (pl: Playlist): string | null => {
    if (!pl.coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(pl.coverArt, 80)
}

const updatePlaylist = useUpdatePlaylist()
const createPlaylist = useCreatePlaylist()

// Adding leaves no trace on screen — the picker closes and the playlist is
// elsewhere — so both outcomes are toasted, or a failed write looks like a
// successful one. On success the selection is cleared (the "added" state), which
// also swaps the strip back to the item's own actions.
const addToExisting = (pl: Playlist): void => {
    if (!songIds.value.length) return
    const n = props.count
    updatePlaylist.mutate(
        { playlistId: pl.id, songIdsToAdd: songIds.value },
        {
            onSuccess: () => {
                toast.add({
                    severity: 'success',
                    summary: `Added to ${pl.name}`,
                    detail: `${n} ${songWord(n)}`,
                    life: 3000
                })
                picker.value?.hide()
                emit('clear')
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
}

const createBusy = computed(() => createPlaylist.isPending.value)
const createNew = (): void => {
    const name = newName.value.trim()
    if (!name || !songIds.value.length) return
    const n = props.count
    createPlaylist.mutate(
        { name, songIds: songIds.value },
        {
            onSuccess: () => {
                toast.add({
                    severity: 'success',
                    summary: `Created ${name}`,
                    detail: `${n} ${songWord(n)} added`,
                    life: 3000
                })
                picker.value?.hide()
                emit('clear')
            },
            onError: (err: unknown) => {
                toast.add({
                    severity: 'error',
                    summary: 'Failed to create playlist',
                    detail: apiErrorMessage(err),
                    life: 5000
                })
            }
        }
    )
}
</script>

<template>
    <!-- Same button chrome as HeroActions (no distinct pill) so entering/leaving
         selection never changes the row height or shifts the hero identity. -->
    <div class="hero-select" role="toolbar" aria-label="Selection actions">
        <Button
            class="sel-play"
            label="Play"
            icon="ms-play-arrow"
            @click="emit('play')"
        />
        <Button
            class="sel-queue"
            label="Add to queue"
            icon="ms-add"
            severity="secondary"
            text
            @click="emit('queue')"
        />
        <Button
            class="sel-add-playlist"
            :class="{ 'is-open': pickerOpen }"
            label="Add to playlist"
            icon="ms-queue-music"
            severity="secondary"
            text
            aria-haspopup="dialog"
            :aria-expanded="pickerOpen"
            @click="togglePicker"
        />
        <Button
            class="sel-favorite"
            icon="ms-favorite"
            severity="secondary"
            text
            rounded
            aria-label="Add to favorites"
            v-tooltip.bottom="'Add to favorites'"
            :disabled="starSongs.isPending.value"
            @click="favorite"
        />
        <span class="hs-sep" aria-hidden="true"></span>
        <span class="hs-count"><b>{{ count }}</b> selected</span>
        <Button
            class="sel-clear"
            icon="ms-close"
            severity="secondary"
            text
            rounded
            aria-label="Clear selection"
            v-tooltip.bottom="'Clear selection'"
            @click="emit('clear')"
        />

        <Popover ref="picker" class="pl-pop" @show="pickerOpen = true" @hide="pickerOpen = false">
            <div class="pl-body" role="dialog" aria-label="Add to playlist">
                <template v-if="face === 'list'">
                    <div class="pl-head">Add {{ countLabel }} to…</div>
                    <div class="pl-search">
                        <InputText v-model="query" placeholder="Find a playlist" autofocus />
                    </div>
                    <div class="pl-list">
                        <button
                            v-for="pl in filtered"
                            :key="pl.id"
                            type="button"
                            class="pl-item sel-pl-item"
                            @click="addToExisting(pl)"
                        >
                            <span class="pl-cover">
                                <img
                                    v-if="playlistCover(pl)"
                                    :src="playlistCover(pl) as string"
                                    alt=""
                                />
                                <i v-else class="ms-queue-music"></i>
                            </span>
                            <span class="pl-txt">
                                <span class="pl-name">{{ pl.name }}</span>
                                <span class="pl-sub"
                                    >{{ pl.songCount }} {{ songWord(pl.songCount) }}</span
                                >
                            </span>
                        </button>
                        <p v-if="filtered.length === 0" class="pl-empty">
                            {{
                                playlistsLoading
                                    ? 'Loading playlists…'
                                    : query.trim()
                                      ? 'No matches'
                                      : 'No playlists yet'
                            }}
                        </p>
                    </div>
                    <div class="pl-foot">
                        <button type="button" class="pl-item pl-new sel-pl-new" @click="face = 'new'">
                            <span class="pl-cover"><i class="ms-add"></i></span>
                            <span class="pl-name">New playlist…</span>
                        </button>
                    </div>
                </template>

                <template v-else>
                    <div class="pl-head">New playlist</div>
                    <div class="pl-new-form">
                        <InputText
                            v-model="newName"
                            placeholder="Playlist name"
                            autofocus
                            @keyup.enter="createNew"
                        />
                        <div class="pl-new-actions">
                            <Button class="sel-pl-back" text label="Back" @click="face = 'list'" />
                            <Button
                                class="sel-pl-create"
                                label="Create"
                                :loading="createBusy"
                                :disabled="!newName.trim()"
                                @click="createNew"
                            />
                        </div>
                    </div>
                </template>
            </div>
        </Popover>
    </div>
</template>

<style scoped>
/* Just the button layout — the frosted pill lives on the hero's action-slot
   wrapper (shared with HeroActions), so this row and the item actions swap inside
   the same pill without changing its height. */
.hero-select {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.5rem;
}

.hs-count {
    font-size: 0.9rem;
    color: #d3dbe3;
    white-space: nowrap;
}
.hs-count b {
    color: #8fecff;
    font-weight: 700;
}
/* Divider between the actions and the "N selected · clear" group on the right. */
.hs-sep {
    width: 1px;
    height: 22px;
    background: rgba(255, 255, 255, 0.22);
    margin: 0 0.1rem;
}

/* A wrapped actions ROW is fine; a wrapped button label is not. */
.hero-select :deep(.p-button-label) {
    white-space: nowrap;
}

/* Match HeroActions' "current playlist" open cue on the picker's button. */
.sel-add-playlist.is-open {
    background: rgba(255, 255, 255, 0.16) !important;
    color: #ffffff !important;
}

/* --- Playlist picker popover (a normal surface panel, not on the band) --- */
.pl-pop :deep(.p-popover-content) {
    padding: 0;
}
.pl-body {
    width: 320px;
    max-width: min(320px, calc(100vw - 2rem));
}
.pl-head {
    padding: 0.7rem 0.9rem;
    border-bottom: 1px solid var(--app-border);
    font-weight: 700;
    font-size: 0.92rem;
}
.pl-search {
    padding: 0.6rem 0.7rem;
    border-bottom: 1px solid var(--app-border);
}
.pl-search :deep(.p-inputtext) {
    width: 100%;
}
.pl-list {
    max-height: 264px;
    overflow: auto;
    padding: 0.35rem;
}
.pl-item {
    display: flex;
    align-items: center;
    gap: 0.7rem;
    width: 100%;
    padding: 0.45rem 0.5rem;
    border: 0;
    background: transparent;
    border-radius: var(--app-radius);
    cursor: pointer;
    text-align: left;
    color: var(--app-text-primary);
    font-family: inherit;
}
.pl-item:hover {
    background: var(--app-hover);
}
.pl-cover {
    width: 40px;
    height: 40px;
    border-radius: 6px;
    flex-shrink: 0;
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--app-bg-subtle);
    color: var(--app-text-secondary);
}
.pl-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}
.pl-txt {
    display: flex;
    flex-direction: column;
    min-width: 0;
}
.pl-name {
    font-weight: 600;
    font-size: 0.9rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.pl-sub {
    font-size: 0.78rem;
    color: var(--app-text-secondary);
}
.pl-empty {
    margin: 0.75rem 0.5rem;
    color: var(--app-text-secondary);
    font-size: 0.88rem;
}
.pl-foot {
    border-top: 1px solid var(--app-border);
    padding: 0.35rem;
}
.pl-new .pl-cover {
    background: var(--app-accent-soft);
    color: var(--app-accent);
    border: 1px dashed var(--app-accent);
}
.pl-new .pl-name {
    color: var(--app-accent);
}
.pl-new-form {
    display: flex;
    flex-direction: column;
    gap: 0.7rem;
    padding: 0.8rem;
}
.pl-new-form :deep(.p-inputtext) {
    width: 100%;
}
.pl-new-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
}
</style>
