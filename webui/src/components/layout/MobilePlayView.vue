<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import QueueBody from '@/components/layout/QueueBody.vue'
import QueueHeaderActions from '@/components/layout/QueueHeaderActions.vue'
import SavePlaylistDialog from '@/components/layout/SavePlaylistDialog.vue'
import { useCurrentTrackFavorite } from '@/composables/useCurrentTrackFavorite'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueActions } from '@/composables/useQueueActions'
import { useQueueEdit } from '@/composables/useQueueEdit'
import { useQueueSummary } from '@/composables/useQueueSummary'
import { subsonicClient } from '@/lib/api/subsonic'

// The phone's Now Playing, rendered by HomeView on the mobile shell. This is
// a first-class ROUTE view (hamburger in the scaffold header, entered and
// left through normal navigation, system back included) — not an overlay, so
// none of the history/focus-trap machinery the old PlayerSheet needed. Two
// faces, one screen: the player face and the queue list, toggled from the
// header. The mini player is hidden on this route (see MobileShell), so the
// face carries the full transport and reserves the bottom safe-area inset.
const player = usePlayer()
const router = useRouter()
const { isStarred, toggleFavorite } = useCurrentTrackFavorite()
const { showSaveDialog, playlistName, openSaveDialog, handleSave, isSaving, clearQueue } =
    useQueueActions()
const { editMode, toggleEditMode } = useQueueEdit()
const { trackCount, summary } = useQueueSummary()

const currentTrack = computed(() => player.currentTrack.value)

const coverUrl = computed(() => {
    const art = currentTrack.value?.coverArt
    if (!art || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(art, 512)
})

const showQueue = ref(false)

const formatTime = (seconds: number): string => {
    if (!seconds || !isFinite(seconds)) return '0:00'
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const onSeekInput = (event: Event): void => {
    player.seek(Number((event.target as HTMLInputElement).value))
}

const goAlbum = (): void => {
    const id = currentTrack.value?.albumId
    if (id) void router.push({ name: 'album', params: { id } })
}

const goArtist = (): void => {
    const id = currentTrack.value?.artistId
    if (id) void router.push({ name: 'artist', params: { id } })
}
</script>

<template>
    <ContentScaffold class="mobile-play-view" title="Now Playing" :summary="summary">
        <template #actions>
            <QueueHeaderActions
                v-if="showQueue"
                :edit-mode="editMode"
                :disabled="trackCount === 0"
                size="small"
                @toggle-edit="toggleEditMode"
                @save="openSaveDialog"
                @clear="clearQueue"
            />
            <Button
                class="play-queue-toggle"
                :class="{ active: showQueue }"
                icon="pi pi-list"
                text
                rounded
                :aria-label="showQueue ? 'Show now playing' : 'Show queue'"
                :aria-pressed="showQueue"
                @click="showQueue = !showQueue"
            />
        </template>

        <div v-if="showQueue" class="play-queue">
            <QueueBody variant="sidebar" :edit-mode="editMode" />
        </div>

        <div v-else class="play-face">
            <div class="play-art">
                <img v-if="coverUrl" :src="coverUrl" alt="" class="play-cover" />
                <div v-else class="play-cover play-cover--placeholder" aria-hidden="true">
                    <i class="pi pi-music"></i>
                </div>
            </div>

            <div class="play-meta">
                <button
                    type="button"
                    class="play-title"
                    :disabled="!currentTrack?.albumId"
                    @click="goAlbum"
                >
                    {{ currentTrack?.title }}
                </button>
                <button
                    type="button"
                    class="play-artist"
                    :disabled="!currentTrack?.artistId"
                    @click="goArtist"
                >
                    {{ currentTrack?.artist }}
                </button>
            </div>

            <div class="play-seek">
                <span class="play-time">{{ formatTime(player.currentTime.value) }}</span>
                <input
                    type="range"
                    class="play-range"
                    aria-label="Seek"
                    min="0"
                    :max="player.duration.value || 0"
                    step="1"
                    :value="player.currentTime.value"
                    @input="onSeekInput"
                />
                <span class="play-time">{{ formatTime(player.duration.value) }}</span>
            </div>

            <div class="play-transport">
                <button
                    type="button"
                    class="play-btn"
                    :class="{ active: player.shuffle.value }"
                    aria-label="Shuffle"
                    @click="player.toggleShuffle()"
                >
                    <i class="pi pi-arrow-right-arrow-left"></i>
                </button>
                <button
                    type="button"
                    class="play-btn"
                    aria-label="Previous track"
                    :disabled="!player.hasPrevious.value"
                    @click="player.playPrevious()"
                >
                    <i class="pi pi-step-backward"></i>
                </button>
                <button
                    type="button"
                    class="play-btn play-btn--play"
                    :aria-label="player.isPlaying.value ? 'Pause' : 'Play'"
                    @click="player.togglePlayPause()"
                >
                    <i :class="player.isPlaying.value ? 'pi pi-pause' : 'pi pi-play'"></i>
                </button>
                <button
                    type="button"
                    class="play-btn"
                    aria-label="Next track"
                    :disabled="!player.hasNext.value"
                    @click="player.playNext()"
                >
                    <i class="pi pi-step-forward"></i>
                </button>
                <button
                    type="button"
                    class="play-btn"
                    :class="{ active: player.repeat.value !== 'none' }"
                    aria-label="Repeat"
                    @click="player.toggleRepeat()"
                >
                    <i class="pi pi-refresh"></i>
                </button>
            </div>

            <button
                type="button"
                class="play-btn play-favorite"
                :aria-label="isStarred ? 'Remove from favorites' : 'Add to favorites'"
                @click="toggleFavorite()"
            >
                <i :class="isStarred ? 'pi pi-heart-fill' : 'pi pi-heart'"></i>
            </button>
        </div>

        <SavePlaylistDialog
            v-model:visible="showSaveDialog"
            v-model:name="playlistName"
            :saving="isSaving"
            @save="handleSave"
        />
    </ContentScaffold>
</template>

<style scoped>
.mobile-play-view {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
    width: 100%;
    /* Now Playing keeps the player-bar palette (the dark blue surface the old
       full-screen sheet used) in BOTH themes — the transport belongs to the
       player chrome, not the page. Everything inside (scaffold header, queue
       rows, transport) colours itself with the app tokens, so remap those for
       the subtree rather than forking the children; custom properties inherit
       through the DOM, so this reaches their scoped rules.
       --app-accent is deliberately NOT remapped: it is the "this is playing"
       signal and already clears 5.2:1 on the player background in both themes. */
    background-color: var(--app-player-bg);
    color: var(--app-player-text);
    --app-text-primary: var(--app-player-text);
    --app-text-secondary: var(--app-player-dim);
    --app-hover: color-mix(in srgb, var(--app-player-text) 12%, transparent);
    --app-border: color-mix(in srgb, var(--app-player-text) 20%, transparent);
    /* The light-theme soft accent is mixed for a white surface and all but
       vanishes here; strengthen it so the now-playing strip stays readable. */
    --app-accent-soft: color-mix(in srgb, var(--app-accent) 20%, transparent);
}

.mobile-play-view :deep(.content-scaffold-body) {
    display: flex;
    flex-direction: column;
    min-height: 0;
}

.play-queue {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
    /* No mini player on this route, so the list is the bottom-most surface
       and reserves the home-indicator inset itself. */
    padding-bottom: env(safe-area-inset-bottom);
}

.play-face {
    position: relative;
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 1.25rem;
    padding: 0 1.5rem calc(1.5rem + env(safe-area-inset-bottom));
}

.play-art {
    width: min(75vw, 45vh);
    aspect-ratio: 1;
}

.play-cover {
    width: 100%;
    height: 100%;
    border-radius: var(--app-radius);
    object-fit: cover;
}

.play-cover--placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: var(--app-hover);
    color: var(--app-text-secondary);
    font-size: 3rem;
}

.play-meta {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.25rem;
    max-width: 100%;
}

.play-title,
.play-artist {
    border: none;
    background: none;
    color: inherit;
    font: inherit;
    cursor: pointer;
    max-width: 80vw;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.play-title {
    font-size: 1.25rem;
    font-weight: 700;
}

.play-artist {
    font-size: 0.95rem;
    color: var(--app-text-secondary);
}

.play-title:disabled,
.play-artist:disabled {
    cursor: default;
}

.play-seek {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    width: 100%;
}

.play-range {
    flex: 1;
    accent-color: var(--app-accent);
}

.play-time {
    font-size: 0.75rem;
    font-family: var(--app-player-time-font);
    color: var(--app-text-secondary);
    min-width: 2.5rem;
    text-align: center;
}

.play-transport {
    display: flex;
    align-items: center;
    gap: 1.1rem;
}

.play-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 2.75rem;
    height: 2.75rem;
    border: none;
    background: none;
    color: var(--app-text-primary);
    cursor: pointer;
    font-size: 1.15rem;
}

.play-btn:disabled {
    color: var(--app-text-secondary);
    cursor: default;
}

.play-btn.active {
    color: var(--app-accent);
}

.play-btn--play {
    width: 3.5rem;
    height: 3.5rem;
    border-radius: 50%;
    background-color: var(--app-accent);
    /* Against the accent disc the icon matches the view's surface, not the
       page background (near-white in light theme). */
    color: var(--app-player-bg);
    font-size: 1.3rem;
}

.play-favorite {
    position: absolute;
    right: 1rem;
    bottom: calc(1rem + env(safe-area-inset-bottom));
}
</style>
