<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import InputText from 'primevue/inputtext'
import SelectButton from 'primevue/selectbutton'
import Button from 'primevue/button'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import ArtistCard from '@/components/library/ArtistCard.vue'
import AlbumCard from '@/components/library/AlbumCard.vue'
import GenreCard from '@/components/library/GenreCard.vue'
import ArtistRow from '@/components/library/ArtistRow.vue'
import AlbumRow from '@/components/library/AlbumRow.vue'
import GenreRow from '@/components/library/GenreRow.vue'
import GenreTrackRow from '@/components/library/GenreTrackRow.vue'
import {
    useSearch,
    searchTermIsLongEnough,
    MIN_SEARCH_LENGTH
} from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { useSongsDrag } from '@/composables/useSongsDrag'
import { useRowSelection } from '@/composables/useRowSelection'
import type { Album, Artist, Genre, Song } from '@/types/subsonic'

type Layout = 'grid' | 'list'

const route = useRoute()
const router = useRouter()

const player = usePlayer()
const songsDrag = useSongsDrag()
const { isSelected, onRowClick, selectionForDrag, clearSelection } = useRowSelection()

const query = ref('')

// The scope is single-select: exactly one is always active, so unlike the old
// checkboxes there is no "nothing selected" state to guard against.
type Scope = 'all' | 'artists' | 'albums' | 'genres' | 'songs'

const scopeOptions: { label: string; value: Scope }[] = [
    { label: 'All', value: 'all' },
    { label: 'Artists', value: 'artists' },
    { label: 'Albums', value: 'albums' },
    { label: 'Genres', value: 'genres' },
    { label: 'Songs', value: 'songs' }
]

const scope = ref<Scope>('all')

const shows = (type: Exclude<Scope, 'all'>): boolean =>
    scope.value === 'all' || scope.value === type

// Reset puts the page back to how it loads: empty term, scope back to All.
// Layout is deliberately left alone — it is a display preference the user set
// (and it lives in the URL), not part of the search itself.
const isPristine = computed(() => query.value === '' && scope.value === 'all')

const resetSearch = (): void => {
    query.value = ''
    scope.value = 'all'
    // The songs watcher only fires when the list actually changes, so clear the
    // row selection here too — resetting from an empty result set must not leave
    // stale indices behind.
    clearSelection()
}

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

// A narrowed scope asks for MORE of the one type it shows: with the whole page
// to itself, 24 albums would leave most of it empty. A zeroed count tells the
// server to skip that query entirely.
const searchParams = computed(() => ({
    query: query.value,
    artistCount: shows('artists') ? (scope.value === 'all' ? 24 : 48) : 0,
    albumCount: shows('albums') ? (scope.value === 'all' ? 24 : 48) : 0,
    songCount: shows('songs') ? (scope.value === 'all' ? 50 : 100) : 0,
    // "searchGenres" extension: 0 tells the server to omit genres entirely.
    genreCount: shows('genres') ? (scope.value === 'all' ? 24 : 48) : 0
}))

const { data: results, isLoading, error } = useSearch(searchParams)

// Gate on the scope as well as the payload: a scope change re-renders before the
// narrowed request resolves, so the previous scope's cached results would flash
// on screen for a frame.
const artists = computed<Artist[]>(() => (shows('artists') ? results.value?.artist || [] : []))
const albums = computed<Album[]>(() => (shows('albums') ? results.value?.album || [] : []))
const songs = computed<Song[]>(() => (shows('songs') ? results.value?.song || [] : []))
const genres = computed<Genre[]>(() => (shows('genres') ? results.value?.genre || [] : []))

// A term shorter than the threshold is treated as "not searching yet", so the
// view shows the prompt rather than a spurious "No results found".
const hasQuery = computed(() => searchTermIsLongEnough(query.value))

// Distinguishes "typed too little" from "typed nothing" so the prompt can say
// what is missing instead of leaving the user waiting on a search that will
// never fire.
const termTooShort = computed(
    () => query.value.trim().length > 0 && !searchTermIsLongEnough(query.value)
)
const hasResults = computed(
    () =>
        artists.value.length > 0 ||
        albums.value.length > 0 ||
        songs.value.length > 0 ||
        genres.value.length > 0
)

// With a narrowed scope there is only ever one section, and the active button
// already names it — a heading above it would just repeat the label.
const showSectionLabels = computed(() => scope.value === 'all')

// Naming the scope makes the empty state actionable ("no *artists*" invites
// switching to All, where a plain "no results" reads as an empty library).
const emptyMessage = computed(() =>
    scope.value === 'all'
        ? 'No results found'
        : `No ${scopeOptions.find((o) => o.value === scope.value)?.label.toLowerCase()} found`
)

const summary = computed(() => {
    if (!hasQuery.value || isLoading.value || error.value) return ''
    const counted = (n: number, singular: string, plural: string) =>
        n > 0 ? [`${n} ${n === 1 ? singular : plural}`] : []
    return [
        ...counted(artists.value.length, 'artist', 'artists'),
        ...counted(albums.value.length, 'album', 'albums'),
        ...counted(genres.value.length, 'genre', 'genres'),
        ...counted(songs.value.length, 'song', 'songs')
    ].join(' • ')
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
                        placeholder="Search artists, albums, genres, songs..."
                        class="search-input"
                        :class="{ 'has-reset': !isPristine }"
                        autofocus
                    />
                    <!-- Clears the term AND the type filters, so it stays useful
                         when only a filter was changed; hidden when there is
                         nothing to clear rather than sitting there disabled.
                         The positioning lives on this span, not on the Button:
                         PrimeVue's ripple directive writes `position: relative`
                         as an INLINE style, which no stylesheet rule can beat,
                         so an absolutely-positioned Button silently lays out in
                         the flex flow and juts past the input's right edge. -->
                    <span v-if="!isPristine" class="reset-search-slot">
                        <Button
                            class="reset-search"
                            icon="pi pi-times"
                            text
                            rounded
                            v-tooltip.bottom="'Reset search'"
                            aria-label="Reset search"
                            @click="resetSearch"
                        />
                    </span>
                </span>
                <SelectButton
                    v-model="scope"
                    :options="scopeOptions"
                    optionLabel="label"
                    optionValue="value"
                    :allowEmpty="false"
                    dataKey="value"
                    class="search-filters"
                    aria-label="Search in"
                />
            </div>

            <div class="search-scroll">
                <div v-if="!hasQuery" class="state-message">
                    <i class="pi pi-search" style="font-size: 3rem"></i>
                    <p v-if="termTooShort">
                        Keep typing — at least {{ MIN_SEARCH_LENGTH }} characters
                    </p>
                    <p v-else>Search your library by artist, album, genre, or song</p>
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
                    <p>{{ emptyMessage }}</p>
                </div>

                <div v-else class="search-content content-col">
                    <section v-if="artists.length > 0" class="result-section">
                        <h2 v-if="showSectionLabels" class="section-label">Artists</h2>
                        <div v-if="layout === 'grid'" class="artist-grid">
                            <ArtistCard v-for="artist in artists" :key="artist.id" :artist="artist" />
                        </div>
                        <div v-else class="row-list">
                            <ArtistRow v-for="artist in artists" :key="artist.id" :artist="artist" />
                        </div>
                    </section>

                    <section v-if="albums.length > 0" class="result-section">
                        <h2 v-if="showSectionLabels" class="section-label">Albums</h2>
                        <div v-if="layout === 'grid'" class="album-grid">
                            <AlbumCard v-for="album in albums" :key="album.id" :album="album" />
                        </div>
                        <div v-else class="row-list">
                            <AlbumRow v-for="album in albums" :key="album.id" :album="album" />
                        </div>
                    </section>

                    <section v-if="genres.length > 0" class="result-section">
                        <h2 v-if="showSectionLabels" class="section-label">Genres</h2>
                        <div v-if="layout === 'grid'" class="genre-grid">
                            <GenreCard v-for="genre in genres" :key="genre.value" :genre="genre" />
                        </div>
                        <div v-else class="row-list">
                            <GenreRow v-for="genre in genres" :key="genre.value" :genre="genre" />
                        </div>
                    </section>

                    <section v-if="songs.length > 0" class="result-section">
                        <h2 v-if="showSectionLabels" class="section-label">Songs</h2>
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
    padding: 2rem var(--app-content-gutter) 1.25rem;
    /* Recipe A: center the search box on the same axis as the content column. */
    padding-right: calc(var(--app-rail-clearance) + 2 * var(--sb-w, 0px) + var(--app-content-gutter));
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

/* Keep the term clear of the reset button, which only exists while it can act. */
.search-input.has-reset {
    padding-right: 3.25rem;
}

/* Overlay the button on the input's right edge. This must sit on the wrapper
   span rather than the Button — see the template comment. */
.reset-search-slot {
    position: absolute;
    right: 0.5rem;
    display: flex;
    align-items: center;
}

.reset-search {
    width: 2.25rem;
    height: 2.25rem;
    color: var(--app-text-secondary);
}

/* Wraps on narrow viewports rather than overflowing the hero. */
.search-filters {
    display: flex;
    flex-wrap: wrap;
    justify-content: center;
}

.search-scroll {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    scrollbar-gutter: stable;
    padding-right: calc(var(--app-rail-clearance) + var(--sb-w, 0px));
    box-sizing: border-box;
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
    padding-top: 1rem;
    padding-bottom: 1rem;
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
.album-grid,
.genre-grid {
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
