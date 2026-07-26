<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRouter, onBeforeRouteLeave } from 'vue-router'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import HeroHeader from '@/components/layout/HeroHeader.vue'
import HeroActions from '@/components/layout/HeroActions.vue'
import EditActionBar from '@/components/layout/EditActionBar.vue'
import AlbumCard from '@/components/library/AlbumCard.vue'
import { useArtist, useToggleStar, useUpdateArtistCover } from '@/composables/useSubsonicQueries'
import { useArtistImageSource } from '@/composables/useArtistImageSource'
import { usePlayer } from '@/composables/usePlayer'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Song } from '@/types/subsonic'

const MAX_COVER_BYTES = 5 * 1024 * 1024

const props = defineProps<{ id: string }>()
const router = useRouter()
const toggleStar = useToggleStar()
const updateCover = useUpdateArtistCover()

const { data: artist, isLoading, error } = useArtist(props.id)

const editing = ref(false)
const cacheBust = ref(0)
const selectedFile = ref<File | null>(null)
const previewUrl = ref<string | null>(null)
const coverClear = ref(false)
const coverSizeError = ref<string | null>(null)

const dirty = computed(() => selectedFile.value !== null || coverClear.value)

// The image may be a file read from the music folder rather than one of aether's
// own — say so while editing, so a "Remove" that cannot delete the user's file
// is not mistaken for one that can. Only fetched once the user opens the editor.
const { data: imageSource, refetch: refetchImageSource } = useArtistImageSource(
    props.id,
    () => editing.value
)
// The flip-back face stays mounted (CSS hides it), so gate on `editing` here
// rather than leaving a stale note in the DOM outside edit mode.
const folderImagePath = computed(() =>
    editing.value && !dirty.value && imageSource.value?.source === 'folder'
        ? imageSource.value.path
        : null
)

const handleStar = () => {
    if (!artist.value) return
    toggleStar.mutate({ id: artist.value.id, starred: !!artist.value.starred })
}

function resetCoverStaging(): void {
    if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
    previewUrl.value = null
    selectedFile.value = null
    coverClear.value = false
    coverSizeError.value = null
}

// Artist edit = cover only. Changes are staged locally and applied on Save.
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
    if (!dirty.value) {
        editing.value = false
        return
    }
    updateCover.mutate(
        {
            artistId: props.id,
            coverFile: selectedFile.value ?? undefined,
            coverClear: coverClear.value || undefined
        },
        {
            onSuccess: () => {
                resetCoverStaging()
                cacheBust.value++
                editing.value = false
                // An upload moves the image into aether's store, a clear can
                // uncover the folder image again — either way the note is stale.
                void refetchImageSource()
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
    if (!artist.value?.coverArt || !subsonicClient.isConfigured()) return null
    const base = subsonicClient.getCoverArtUrl(artist.value.coverArt, 250)
    return cacheBust.value > 0 ? `${base}&_cb=${cacheBust.value}` : base
})

// Unsaved-changes guards (mirror Playlist/Radio detail views).
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

const heroMeta = computed(() => {
    if (!artist.value) return []
    const parts: string[] = []
    const albums = artist.value.albumCount ?? artist.value.album?.length ?? 0
    const songs =
        artist.value.songCount ??
        artist.value.album?.reduce((n, a) => n + (a.songCount ?? 0), 0) ??
        0
    if (albums) parts.push(`${albums} ${albums === 1 ? 'album' : 'albums'}`)
    if (songs) parts.push(`${songs} ${songs === 1 ? 'song' : 'songs'}`)
    return parts
})

const sortedAlbums = computed(() => {
    if (!artist.value?.album) return []
    // getArtist doesn't always populate each album's `artist` field, which would
    // leave the card subtitle blank (one-line cards). Fall back to this artist's
    // name so the cards match the library grid (album name + artist, two lines).
    const artistName = artist.value.name
    return [...artist.value.album]
        .map((a) => ({ ...a, artist: a.artist || artistName }))
        .sort((a, b) => (b.year || 0) - (a.year || 0))
})

const player = usePlayer()
const gathering = ref(false)

// getArtist returns albums without their songs, so gather each album's songs on
// demand before playing/queuing the whole discography.
async function gatherSongs(): Promise<Song[]> {
    const results = await Promise.all(sortedAlbums.value.map((a) => subsonicClient.getAlbum(a.id)))
    return results.flatMap((al) => al?.song ?? [])
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
</script>

<template>
    <div class="artist-view">
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="error" class="error">
            <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
            <p>{{ error.message }}</p>
        </div>

        <ContentScaffold v-else-if="artist" title="" show-back @back="router.back()">
            <template #actions>
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

            <div class="artist-scroll">
                <div class="artist-body content-col">
                    <HeroHeader
                        eyebrow="Artist"
                        cover-placeholder-icon="pi pi-user"
                        cover-back-label="Artist image"
                        :cover-url="coverUrl"
                        :cover-size-error="coverSizeError"
                        v-model:editing="editing"
                        @cover-select="onCoverSelect"
                        @cover-remove="onRemoveCover"
                    >
                        <template #cover-note>
                            <span
                                v-if="folderImagePath"
                                class="image-source-note"
                                :title="folderImagePath"
                            >
                                <i class="pi pi-image"></i>
                                <span>Image loaded from music folder</span>
                            </span>
                        </template>

                        <template #read>
                            <h2 class="hero-name">{{ artist.name }}</h2>
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
                                can-queue
                                can-star
                                :starred="!!artist.starred"
                                :busy="gathering"
                                @play="onPlay"
                                @queue="onQueue"
                                @star="handleStar"
                            />
                        </template>
                    </HeroHeader>

                    <section v-if="sortedAlbums.length > 0" class="discography">
                        <h2>Albums</h2>
                        <div class="album-grid">
                            <AlbumCard
                                v-for="album in sortedAlbums"
                                :key="album.id"
                                :album="album"
                            />
                        </div>
                    </section>
                </div>
            </div>
        </ContentScaffold>
    </div>
</template>

<style scoped>
.artist-view {
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

.artist-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
    /* Recipe B: uniform rail clearance so the column matches the list views. */
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    box-sizing: border-box;
}

.artist-body {
    padding-top: 1rem;
    padding-bottom: 1rem;
}

.discography h2 {
    font-size: 1.5rem;
    font-weight: 600;
    margin-bottom: 1.5rem;
}

/* Sits on the cover's flip-back face, so it has to stay inside 250px. */
.image-source-note {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.35rem;
    font-size: 0.72rem;
    line-height: 1.2;
    color: var(--app-text-secondary);
    cursor: help;
}
/* The sentence is longer than the 250px panel is wide at some font sizes, so it
   wraps rather than truncating — the hover tooltip carries the path anyway. */
.image-source-note span {
    text-align: center;
}
.image-source-note i {
    flex-shrink: 0;
}

.album-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 2rem;
}
</style>
