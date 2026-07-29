<script setup lang="ts">
import { computed } from 'vue'
import AlbumCard from '@/components/library/AlbumCard.vue'
import AlbumRow from '@/components/library/AlbumRow.vue'
import PlaylistCard from '@/components/library/PlaylistCard.vue'
import {
    useDiscoverySection,
    SHELF_ALBUM_COUNT,
    SHELF_PLAYLIST_COUNT
} from '@/composables/useDiscovery'

const props = defineProps<{ sectionKey: string; layout: 'grid' | 'list' }>()

// Cards and rows are used directly rather than VirtualCardGrid / AlbumListView:
// those own a full-height scroller and an alphabet rail, which cannot nest in a
// shelf. A capped shelf gains nothing from virtualisation.
// Getter, not props.sectionKey: the composable's key must stay reactive.
const {
    section,
    albums,
    playlists,
    albumsLoading,
    albumsError,
    playlistsLoading,
    playlistsError
} = useDiscoverySection(() => props.sectionKey, SHELF_ALBUM_COUNT)

const shownAlbums = computed(() => albums.value.slice(0, SHELF_ALBUM_COUNT))
const shownPlaylists = computed(() => playlists.value.slice(0, SHELF_PLAYLIST_COUNT))

const isLoading = computed(() => albumsLoading.value && playlistsLoading.value)
const isError = computed(() => albumsError.value && playlistsError.value)
// "Nothing here yet" may only win when NEITHER block has anything to say — so it
// is gated on the per-block flags, not on the combined AND-flags above. Gating on
// `isError` alone would let an album-only failure (with no playlists to show)
// render as an empty section and hide the album error branch entirely.
const isEmpty = computed(
    () =>
        !albumsLoading.value &&
        !albumsError.value &&
        !playlistsLoading.value &&
        !playlistsError.value &&
        shownAlbums.value.length === 0 &&
        shownPlaylists.value.length === 0
)
</script>

<template>
    <section class="discovery-section">
        <header class="section-header content-col">
            <h2 class="section-title">
                <i v-if="section" :class="section.icon"></i>
                {{ section?.title ?? sectionKey }}
            </h2>
            <router-link
                class="section-show-all"
                :to="{ name: 'discover-section', params: { section: sectionKey } }"
            >
                Show all
                <i class="pi pi-arrow-right"></i>
            </router-link>
        </header>

        <div v-if="isLoading" class="section-loading content-col">
            <i class="pi pi-spin pi-spinner" style="font-size: 1.5rem"></i>
        </div>

        <div v-else-if="isError" class="section-error section-error-all content-col">
            <i class="pi pi-exclamation-triangle"></i>
            <span>Could not load this section</span>
        </div>

        <div v-else-if="isEmpty" class="section-empty content-col">
            <span>Nothing here yet</span>
        </div>

        <template v-else>
            <div
                v-if="albumsLoading"
                class="section-loading section-albums-loading content-col"
            >
                <i class="pi pi-spin pi-spinner" style="font-size: 1.5rem"></i>
            </div>
            <div v-else-if="albumsError" class="section-albums-error content-col">
                <i class="pi pi-exclamation-triangle"></i>
                <span>Could not load albums</span>
            </div>
            <div
                v-else-if="shownAlbums.length > 0"
                class="section-albums content-col"
                :class="layout"
            >
                <template v-if="layout === 'grid'">
                    <AlbumCard v-for="al in shownAlbums" :key="al.id" :album="al" />
                </template>
                <template v-else>
                    <AlbumRow v-for="al in shownAlbums" :key="al.id" :album="al" />
                </template>
            </div>

            <div
                v-if="playlistsLoading"
                class="section-loading section-playlists-loading content-col"
            >
                <i class="pi pi-spin pi-spinner" style="font-size: 1.5rem"></i>
            </div>
            <div
                v-else-if="playlistsError"
                class="section-playlists-error content-col"
            >
                <i class="pi pi-exclamation-triangle"></i>
                <span>Could not load playlists</span>
            </div>
            <div
                v-else-if="shownPlaylists.length > 0"
                class="section-playlists content-col"
                :class="layout"
            >
                <PlaylistCard v-for="pl in shownPlaylists" :key="pl.id" :playlist="pl" />
            </div>
        </template>
    </section>
</template>

<style scoped>
.discovery-section {
    padding-block: 1rem;
}

.section-header {
    display: flex;
    align-items: baseline;
    gap: 1rem;
}

.section-title {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin: 0;
    font-size: 1.1rem;
    font-weight: 700;
}

.section-show-all {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--app-text-secondary);
    text-decoration: none;
}

.section-show-all:hover {
    color: var(--app-accent);
}

.section-albums.grid,
.section-playlists.grid {
    padding-top: 0.75rem;
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 1.5rem;
}

.section-albums.list,
.section-playlists.list {
    padding-top: 0.75rem;
    display: flex;
    flex-direction: column;
}

.section-loading,
.section-error,
.section-empty {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding-block: 1.5rem;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
}
</style>
