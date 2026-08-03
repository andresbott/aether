<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Button from 'primevue/button'
import Tag from 'primevue/tag'
import type { Song } from '@/types/subsonic'
import { subsonicClient } from '@/lib/api/subsonic'
import { useAlbum } from '@/composables/useSubsonicQueries'
import { useSongFavorite } from '@/composables/useSongFavorite'

const props = defineProps<{
    song: Song
    showBackButton?: boolean
    card?: boolean
}>()

const emit = defineEmits<{
    back: []
    play: []
}>()

const coverArtUrl = computed(() => {
    if (!props.song.coverArt || !subsonicClient.isConfigured()) return null
    return subsonicClient.getCoverArtUrl(props.song.coverArt, 512)
})

const titleRef = ref<HTMLHeadingElement | null>(null)
let resizeObserver: ResizeObserver | null = null
let fitting = false

const fitTitle = () => {
    const el = titleRef.value
    if (!el || fitting) return
    fitting = true
    el.style.fontSize = ''
    el.style.whiteSpace = 'nowrap'
    let size = parseFloat(getComputedStyle(el).fontSize)
    const minSize = 16
    while (el.scrollWidth > el.clientWidth + 1 && size > minSize) {
        size -= 1
        el.style.fontSize = `${size}px`
    }
    if (el.scrollWidth > el.clientWidth + 1) {
        el.style.whiteSpace = 'normal'
    }
    fitting = false
}

onMounted(() => {
    nextTick(fitTitle)
    const parent = titleRef.value?.parentElement
    if (parent) {
        resizeObserver = new ResizeObserver(() => {
            if (!fitting) requestAnimationFrame(fitTitle)
        })
        resizeObserver.observe(parent)
    }
})

onBeforeUnmount(() => {
    resizeObserver?.disconnect()
})

watch(
    () => props.song.title,
    () => nextTick(fitTitle)
)

const formatDuration = (seconds?: number): string => {
    if (!seconds) return ''
    const mins = Math.floor(seconds / 60)
    const secs = Math.floor(seconds % 60)
    return `${mins}:${secs.toString().padStart(2, '0')}`
}

const formatFileSize = (bytes?: number): string => {
    if (!bytes) return ''
    const mb = bytes / (1024 * 1024)
    return `${mb.toFixed(1)} MB`
}

// The disc subtitle lives on the album (OpenSubsonic discTitles), not on the
// song, so the card fetches its album to label multi-disc releases. Only the
// card shows it, so the plain detail view makes no extra request.
const albumQuery = useAlbum(() => (props.card ? props.song.albumId : undefined))

// An unset disc number is disc 0, which is how a single-disc release with a
// disc subtitle is tagged — the subtitle still applies to this song.
const discSubtitle = computed(() => {
    const disc = props.song.discNumber ?? 0
    return albumQuery.data.value?.discTitles?.find((d) => d.disc === disc)?.title ?? ''
})

// "Disc 2", "Disc 2 · Bonus Tracks" or just the subtitle — rendered after the
// album title. Disc 0 means "unset" and is not shown.
const discLabel = computed(() => {
    const parts: string[] = []
    if (props.song.discNumber) parts.push(`Disc ${props.song.discNumber}`)
    if (discSubtitle.value) parts.push(discSubtitle.value)
    return parts.join(' · ')
})

// Shared with the player bar, the `L` shortcut and every track row, so the card's
// heart and the row hearts cannot diverge — and so the now-playing card responds
// on the tick even though the queue is not query-backed.
const { isStarred, toggleFavorite: toggleLike } = useSongFavorite(() => props.song)
</script>

<template>
    <div class="song-detail" :class="{ 'song-detail--card': card }">
        <Button
            v-if="showBackButton"
            icon="pi pi-arrow-left"
            text
            rounded
            class="back-btn"
            @click="emit('back')"
        />

        <div class="detail-layout">
            <div class="cover-art">
                <img v-if="coverArtUrl" :src="coverArtUrl" :alt="song.title" />
                <div v-else class="cover-placeholder">
                    <i class="pi pi-music" style="font-size: 4rem"></i>
                </div>
            </div>

            <div class="song-info">
                <h1 ref="titleRef">{{ song.title }}</h1>
                <p class="artist">{{ song.artist }}</p>
                <p v-if="song.albumId" class="album-line">
                    <router-link
                        :to="{ name: 'album', params: { id: song.albumId } }"
                        class="album-link"
                    >
                        {{ song.album }}
                    </router-link>
                    <span v-if="discLabel" class="disc-label">{{ discLabel }}</span>
                </p>

                <div v-if="!card" class="meta">
                    <span v-if="song.year">{{ song.year }}</span>
                    <Tag v-if="song.genre" :value="song.genre" severity="secondary" />
                    <span v-if="song.duration">{{ formatDuration(song.duration) }}</span>
                    <span v-if="song.bitRate">{{ song.bitRate }} kbps</span>
                    <span v-if="song.size">{{ formatFileSize(song.size) }}</span>
                    <span v-if="song.contentType">{{ song.contentType }}</span>
                </div>

                <template v-if="card">
                    <div class="divider"></div>
                    <p v-if="song.genre" class="detail genre-line">
                        <span class="detail-label">Genres:</span>
                        <span>{{ song.genre }}</span>
                    </p>
                    <dl class="details-grid">
                        <div v-if="song.year" class="detail">
                            <dt>Year:</dt>
                            <dd>{{ song.year }}</dd>
                        </div>
                        <div v-if="song.duration" class="detail">
                            <dt>Duration:</dt>
                            <dd>{{ formatDuration(song.duration) }}</dd>
                        </div>
                        <div v-if="song.track" class="detail">
                            <dt>Track:</dt>
                            <dd>{{ song.track }}</dd>
                        </div>
                        <div v-if="song.bitRate" class="detail">
                            <dt>Bit rate:</dt>
                            <dd>{{ song.bitRate }} kbps</dd>
                        </div>
                        <div v-if="song.size" class="detail">
                            <dt>File size:</dt>
                            <dd>{{ formatFileSize(song.size) }}</dd>
                        </div>
                        <div v-if="song.contentType" class="detail">
                            <dt>Format:</dt>
                            <dd>{{ song.contentType }}</dd>
                        </div>
                    </dl>

                    <!-- `secondary` in both states: a favorite reads as favorite by
                         the FILLED icon alone, not by colour (it used to turn
                         `danger` red) — see unified-play-experience.md. -->
                    <div class="card-actions">
                        <Button
                            :icon="isStarred ? 'pi pi-heart-fill' : 'pi pi-heart'"
                            :label="isStarred ? 'Remove from favorites' : 'Add to favorites'"
                            severity="secondary"
                            outlined
                            @click="toggleLike"
                        />
                    </div>
                </template>

                <div v-if="!card" class="actions">
                    <Button label="Play" icon="pi pi-play" @click="emit('play')" />
                </div>
            </div>
        </div>
    </div>
</template>

<style scoped>
.song-detail {
    width: 100%;
    margin: 0 auto;
}

.song-detail--card {
    background: var(--app-surface);
    border: 1px solid var(--app-border);
    border-radius: 12px;
    overflow: hidden;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.05);
}

.song-detail--card .back-btn {
    margin: 1rem 0 0 1rem;
}

.song-detail--card .detail-layout {
    gap: 2rem;
    align-items: stretch;
}

.song-detail--card .cover-art {
    border-radius: 0;
    width: 512px;
    height: 512px;
    aspect-ratio: 1;
}

.song-detail--card .song-info {
    padding: 1.75rem 1.75rem 1.75rem 0;
    gap: 0.5rem;
}

.song-detail--card .song-info h1 {
    font-size: 2.15rem;
}

.song-detail--card .divider {
    margin: 0.9rem 0 0.25rem;
}

.song-detail--card .details-grid {
    gap: 0.6rem 2rem;
    margin-top: 0.6rem;
}

.song-detail--card .card-actions {
    margin-top: auto;
    padding-top: 1rem;
}

.back-btn {
    margin-bottom: 1rem;
}

.detail-layout {
    display: flex;
    gap: 2.5rem;
}

.cover-art {
    width: 512px;
    height: 512px;
    flex-shrink: 0;
    border-radius: 8px;
    overflow: hidden;
}

.cover-art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.cover-placeholder {
    width: 100%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: rgba(255, 255, 255, 0.8);
}

.song-info {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.song-info h1 {
    font-size: 2.5rem;
    font-weight: 700;
    margin: 0;
    line-height: 1.2;
    white-space: nowrap;
    overflow-wrap: anywhere;
    min-width: 0;
}

.artist {
    font-size: 1.25rem;
    color: var(--app-text-secondary);
    margin: 0;
}

.album-line {
    display: flex;
    align-items: baseline;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin: 0;
    min-width: 0;
}

.album-link {
    font-size: 1rem;
    color: var(--app-accent);
}

.disc-label {
    font-size: 0.9rem;
    color: var(--app-text-secondary);
}

.disc-label::before {
    content: '\00b7';
    margin-right: 0.5rem;
}

.meta {
    display: flex;
    flex-wrap: wrap;
    gap: 0.75rem;
    align-items: center;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
    margin-top: 0.5rem;
}

.meta span:not(:last-child)::after {
    content: '\00b7';
    margin-left: 0.75rem;
}

.actions {
    margin-top: auto;
    display: flex;
    gap: 0.75rem;
}

.divider {
    height: 1px;
    background: var(--app-border);
    margin: 1rem 0 0.25rem;
}

.genre-line {
    margin: 0.75rem 0 0;
}

.card-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.75rem;
}

.details-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.75rem 2rem;
    margin: 0.75rem 0 0;
    padding: 0;
}

.detail {
    margin: 0;
    display: flex;
    align-items: baseline;
    gap: 0.4rem;
    font-size: 0.95rem;
    color: var(--app-text-primary);
}

.detail dt,
.detail .detail-label {
    font-weight: 600;
    color: var(--app-text-secondary);
    margin: 0;
}

.detail dd {
    margin: 0;
}

@media (max-width: 768px) {
    .detail-layout {
        flex-direction: column;
        align-items: center;
    }

    .cover-art {
        width: 280px;
        height: 280px;
    }

    .song-info {
        align-items: center;
        text-align: center;
    }

    .song-info h1 {
        font-size: 1.75rem;
    }

    .song-detail--card .cover-art {
        width: 100%;
        height: auto;
        aspect-ratio: 1;
    }

    .song-detail--card .song-info {
        padding: 1.5rem;
    }
}
</style>
