<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import InputText from 'primevue/inputtext'
import Checkbox from 'primevue/checkbox'
import SelectButton from 'primevue/selectbutton'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import ArtistCard from '@/components/library/ArtistCard.vue'
import AlbumCard from '@/components/library/AlbumCard.vue'
import ArtistRow from '@/components/library/ArtistRow.vue'
import AlbumRow from '@/components/library/AlbumRow.vue'
import GenreTrackRow from '@/components/library/GenreTrackRow.vue'
import { useSearch } from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { useSongsDrag } from '@/composables/useSongsDrag'
import { useRowSelection } from '@/composables/useRowSelection'
import type { Album, Artist, Song } from '@/types/subsonic'

type Layout = 'grid' | 'list'

const route = useRoute()
const router = useRouter()

const player = usePlayer()
const songsDrag = useSongsDrag()
const { isSelected, onRowClick, selectionForDrag, clearSelection } = useRowSelection()

const query = ref('')

const searchArtists = ref(true)
const searchAlbums = ref(true)
const searchSongs = ref(true)

const anyTypeSelected = computed(
    () => searchArtists.value || searchAlbums.value || searchSongs.value
)

const layoutOptions = [
    { label: 'List', value: 'list', icon: 'pi pi-list' },
    { label: 'Grid', value: 'grid', icon: 'pi pi-th-large' }
]

const layout = computed<Layout>({
    get: () => (route.query.view === 'list' ? 'list' : 'grid'),
    set: (v) => {
        const query = { ...route.query }
        if (v === 'list') query.view = 'list'
        else delete query.view
        router.replace({ query })
    }
})

const searchParams = computed(() => ({
    // An empty query keeps the search disabled when no type is selected.
    query: anyTypeSelected.value ? query.value : '',
    artistCount: searchArtists.value ? 24 : 0,
    albumCount: searchAlbums.value ? 24 : 0,
    songCount: searchSongs.value ? 50 : 0
}))

const { data: results, isLoading, error } = useSearch(searchParams)

const artists = computed<Artist[]>(() =>
    searchArtists.value ? results.value?.artist || [] : []
)
const albums = computed<Album[]>(() =>
    searchAlbums.value ? results.value?.album || [] : []
)
const songs = computed<Song[]>(() => (searchSongs.value ? results.value?.song || [] : []))

const hasQuery = computed(() => query.value.trim().length > 0 && anyTypeSelected.value)
const hasResults = computed(
    () => artists.value.length > 0 || albums.value.length > 0 || songs.value.length > 0
)

const summary = computed(() => {
    if (!hasQuery.value || isLoading.value || error.value) return ''
    const parts: string[] = []
    if (artists.value.length > 0) {
        parts.push(`${artists.value.length} ${artists.value.length === 1 ? 'artist' : 'artists'}`)
    }
    if (albums.value.length > 0) {
        parts.push(`${albums.value.length} ${albums.value.length === 1 ? 'album' : 'albums'}`)
    }
    if (songs.value.length > 0) {
        parts.push(`${songs.value.length} ${songs.value.length === 1 ? 'song' : 'songs'}`)
    }
    return parts.join(' • ')
})

// --- Song results (album-style rows with a cover column, as in the playlist view) ---
const playFrom = (index: number): void => {
    player.playAlbum(songs.value, index)
}

// A drag from a selected row carries the whole selection; from an unselected
// row it carries just that row.
const onRowDragStart = (event: DragEvent, index: number): void => {
    const dragSongs = selectionForDrag(index)
        .map((i) => songs.value[i])
        .filter((s): s is Song => s !== undefined)
    songsDrag.start(event, dragSongs, event.currentTarget as HTMLElement)
}

// Selection indices point into the result list — drop them when it changes.
watch(songs, () => clearSelection())
</script>

<template>
    <ContentScaffold title="Search" :summary="summary">
        <template #actions>
            <SelectButton
                v-model="layout"
                :options="layoutOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
                dataKey="value"
                aria-label="Layout"
            >
                <template #option="slotProps">
                    <i :class="slotProps.option.icon"></i>
                </template>
            </SelectButton>
        </template>

        <div class="search-page">
            <div class="search-hero">
                <span class="search-input-wrapper">
                    <i class="pi pi-search search-icon"></i>
                    <InputText
                        v-model="query"
                        placeholder="Search artists, albums, songs..."
                        class="search-input"
                        autofocus
                    />
                </span>
                <div class="search-filters" role="group" aria-label="Search in">
                    <label class="filter-option">
                        <Checkbox v-model="searchArtists" binary inputId="search-artists" />
                        <span>Artists</span>
                    </label>
                    <label class="filter-option">
                        <Checkbox v-model="searchAlbums" binary inputId="search-albums" />
                        <span>Albums</span>
                    </label>
                    <label class="filter-option">
                        <Checkbox v-model="searchSongs" binary inputId="search-songs" />
                        <span>Song titles</span>
                    </label>
                </div>
            </div>

            <div class="search-scroll">
                <div v-if="!hasQuery" class="state-message">
                    <i class="pi pi-search" style="font-size: 3rem"></i>
                    <p v-if="!anyTypeSelected">Select at least one type to search</p>
                    <p v-else>Search your library by artist, album, or song</p>
                </div>

                <div v-else-if="isLoading" class="state-message">
                    <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
                </div>

                <div v-else-if="error" class="state-message">
                    <i class="pi pi-exclamation-triangle" style="font-size: 3rem"></i>
                    <p>Could not search</p>
                </div>

                <div v-else-if="!hasResults" class="state-message">
                    <i class="pi pi-search" style="font-size: 3rem"></i>
                    <p>No results found</p>
                </div>

                <div v-else class="search-content">
                    <section v-if="artists.length > 0" class="result-section">
                        <h2 class="section-label">Artists</h2>
                        <div v-if="layout === 'grid'" class="artist-grid">
                            <ArtistCard v-for="artist in artists" :key="artist.id" :artist="artist" />
                        </div>
                        <div v-else class="row-list">
                            <ArtistRow v-for="artist in artists" :key="artist.id" :artist="artist" />
                        </div>
                    </section>

                    <section v-if="albums.length > 0" class="result-section">
                        <h2 class="section-label">Albums</h2>
                        <div v-if="layout === 'grid'" class="album-grid">
                            <AlbumCard v-for="album in albums" :key="album.id" :album="album" />
                        </div>
                        <div v-else class="row-list">
                            <AlbumRow v-for="album in albums" :key="album.id" :album="album" />
                        </div>
                    </section>

                    <section v-if="songs.length > 0" class="result-section">
                        <h2 class="section-label">Songs</h2>
                        <div class="track-list">
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
                                v-for="(song, index) in songs"
                                :key="song.id"
                                :song="song"
                                :index="index"
                                :selected="isSelected(index)"
                                @select="(p) => onRowClick(index, p)"
                                @play="playFrom(index)"
                                @dragstart="(e) => onRowDragStart(e, index)"
                                @dragend="songsDrag.end"
                            />
                        </div>
                    </section>
                </div>
            </div>
        </div>
    </ContentScaffold>
</template>

<style scoped>
.search-page {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
}

.search-hero {
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.9rem;
    width: 100%;
    box-sizing: border-box;
    padding: 2rem 1rem 1.25rem;
}

.search-input-wrapper {
    position: relative;
    display: flex;
    align-items: center;
    width: 100%;
    max-width: 640px;
}

.search-icon {
    position: absolute;
    left: 1.25rem;
    font-size: 1.1rem;
    color: var(--app-text-secondary);
    pointer-events: none;
}

.search-input {
    width: 100%;
    font-size: 1.1rem;
    padding: 0.85rem 1.5rem 0.85rem 3.25rem;
    border-radius: 9999px;
}

.search-filters {
    display: flex;
    align-items: center;
    justify-content: center;
    flex-wrap: wrap;
    gap: 1.5rem;
}

.filter-option {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.9rem;
    color: var(--app-text-secondary);
    cursor: pointer;
    user-select: none;
}

.search-scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    scrollbar-gutter: stable;
}

.state-message {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 4rem;
    gap: 1rem;
    color: var(--app-text-secondary);
    text-align: center;
}

.search-content {
    max-width: var(--app-content-max-width);
    margin: 0 auto;
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 2rem;
}

.section-label {
    margin: 0 0 1rem;
    font-size: 0.85rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--app-text-secondary);
}

.artist-grid,
.album-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 2rem;
}

.row-list {
    display: flex;
    flex-direction: column;
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
