<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SelectButton from 'primevue/selectbutton'
import ToggleButton from 'primevue/togglebutton'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import AlbumListView from '@/components/library/AlbumListView.vue'
import AlbumGrid from '@/components/library/AlbumGrid.vue'
import ArtistListView from '@/components/library/ArtistListView.vue'
import ArtistGrid from '@/components/library/ArtistGrid.vue'
import DiscoveryFeed from '@/components/library/DiscoveryFeed.vue'
import SongListView from '@/components/library/SongListView.vue'
import { useMusicFolders } from '@/composables/useSubsonicQueries'
import { useAlbumIndex } from '@/composables/useAlbumIndex'
import { useArtistTable } from '@/composables/useArtistTable'
import { useStarredAlbums, useStarredArtists } from '@/composables/useStarred'
import { useDiscoveryFeed } from '@/composables/useDiscovery'
import { useSongList } from '@/composables/useSongList'
import { useUiStore } from '@/store/uiStore'

type ViewMode = 'discover' | 'albums' | 'artists' | 'songs'
type Layout = 'grid' | 'list'

const route = useRoute()
const router = useRouter()
const uiStore = useUiStore()

const layoutOptions = [
    { label: 'List', value: 'list', icon: 'pi pi-list' },
    { label: 'Grid', value: 'grid', icon: 'pi pi-th-large' }
]

const folderId = computed<number | undefined>(() => {
    const raw = route.params.folderId
    const value = Array.isArray(raw) ? raw[0] : raw
    if (!value) return undefined
    const num = Number(value)
    return Number.isFinite(num) ? num : undefined
})

const { data: folders } = useMusicFolders()
const folder = computed(() => folders.value?.find((f) => f.id === folderId.value))
const folderName = computed(() =>
    folderId.value === undefined ? 'Library' : (folder.value?.name ?? 'Library')
)

// The Discovery ranking is cross-collection, so the tab exists only on the root
// route (no folderId). A per-library discovery feed is deliberately not a thing —
// see DiscoveryFeed.vue. This tab is also the ONLY way to reach the feed: there is
// no standalone /discover route.
const discoverTabVisible = computed(() => folderId.value === undefined)

const artistsTabVisible = computed(() => {
    if (folderId.value === undefined) return true
    return folder.value?.showArtists !== false
})

const viewOptions = computed(() => [
    ...(discoverTabVisible.value ? [{ label: 'Discover', value: 'discover' }] : []),
    { label: 'Albums', value: 'albums' },
    ...(artistsTabVisible.value ? [{ label: 'Artists', value: 'artists' }] : []),
    { label: 'Songs', value: 'songs' }
])

// On the root route, Discover is the default; a folder keeps its configured
// default_view, which the server only ever reports as albums/artists.
const serverDefault = computed<ViewMode>(() => {
    if (discoverTabVisible.value) return 'discover'
    return folder.value?.defaultView ?? 'albums'
})

const hashView = computed<ViewMode | null>(() => {
    const h = route.hash.replace('#', '')
    return h === 'discover' || h === 'albums' || h === 'artists' || h === 'songs' ? h : null
})

const viewMode = computed<ViewMode>({
    get: () => {
        const wanted = hashView.value ?? serverDefault.value
        // A hash for a tab this route does not offer (a folder deep-linked to
        // #discover, or #artists on a library with showArtists=false) falls back
        // to albums rather than rendering a tab with no toggle to leave it.
        if (wanted === 'discover' && !discoverTabVisible.value) return 'albums'
        if (wanted === 'artists' && !artistsTabVisible.value) return 'albums'
        return wanted
    },
    set: (v) => {
        router.replace({ hash: `#${v}`, query: route.query })
    }
})

// Per-type layout: each view mode has its own default and can be overridden
// independently. Session-scoped via uiStore (survives navigation but not reload).
const layout = computed<Layout>({
    get: () => uiStore.getLibraryViewMode(viewMode.value),
    set: (v) => uiStore.setLibraryViewMode(viewMode.value, v)
})

// Favorites filter, in the URL so it survives a reload and is linkable. It applies
// to the Albums and Artists tabs only: Discover is a ranked feed in which favorites
// are already a scoring term, not a filterable list. Songs tab does not support the
// favorites filter (search3 has no starred param).
const favoritesOnly = computed<boolean>({
    get: () => route.query.favorites === '1' && viewMode.value !== 'discover' && viewMode.value !== 'songs',
    set: (v) => {
        const query = { ...route.query }
        if (v) query.favorites = '1'
        else delete query.favorites
        router.replace({ hash: route.hash, query })
    }
})

// Header counts — only the active tab's ACTIVE SOURCE is fetched, so the count
// never costs a request the body isn't already making (the pairs share a query
// cache entry with the grid/list below).
const { total: albumTotal } = useAlbumIndex(folderId, {
    enabled: computed(() => viewMode.value === 'albums' && !favoritesOnly.value)
})
const { total: artistTotal } = useArtistTable(folderId, {
    enabled: computed(() => viewMode.value === 'artists' && !favoritesOnly.value)
})
const { total: starredAlbumTotal } = useStarredAlbums(folderId, {
    enabled: computed(() => viewMode.value === 'albums' && favoritesOnly.value)
})
const { total: starredArtistTotal } = useStarredArtists(folderId, {
    enabled: computed(() => viewMode.value === 'artists' && favoritesOnly.value)
})
// Shares its query cache entry with the DiscoveryFeed in the body, so reading the
// count here costs no extra request.
const { items: discoveryItems } = useDiscoveryFeed()
// Shares its query cache entry with the SongListView in the body. Enabled only
// when the Songs tab is active. Unlike albums/artists, there is no "total" from
// the backend (search3 doesn't report it), so we count the flattened items from
// the loaded pages.
const { items: songItems } = useSongList(
    folderId,
    computed(() => false),
    computed(() => viewMode.value === 'songs')
)
const songCount = computed(() => songItems.value.length)

const summary = computed(() => {
    if (viewMode.value === 'discover') {
        const n = discoveryItems.value.length
        return n > 0 ? `${n} item${n === 1 ? '' : 's'}` : ''
    }
    // Filtered, the count is of favorites, and a bare "6 albums" would read as the
    // whole library. "6 favorites" rather than "6 favorite albums" because the
    // active tab already names the type, and the root route's header — three tabs
    // plus two toggles — has no room for the longer form.
    if (favoritesOnly.value) {
        const n = viewMode.value === 'albums' ? starredAlbumTotal.value : starredArtistTotal.value
        return n > 0 ? `${n} favorite${n === 1 ? '' : 's'}` : ''
    }
    if (viewMode.value === 'albums') {
        return albumTotal.value > 0
            ? `${albumTotal.value} ${albumTotal.value === 1 ? 'album' : 'albums'}`
            : ''
    }
    if (viewMode.value === 'songs') {
        return songCount.value > 0
            ? `${songCount.value} ${songCount.value === 1 ? 'song' : 'songs'}`
            : ''
    }
    return artistTotal.value > 0
        ? `${artistTotal.value} ${artistTotal.value === 1 ? 'artist' : 'artists'}`
        : ''
})
</script>

<template>
    <ContentScaffold :title="folderName" :summary="summary">
        <template #actions>
            <!-- Favorites filter, first in the bar because it changes WHAT is
                 listed while the two SelectButtons only change how. Hidden on
                 Discover (ranked feed) and Songs (search3 has no starred param).
                 Same heart pair and wording as every other favorite affordance. -->
            <ToggleButton
                v-if="viewMode !== 'discover' && viewMode !== 'songs'"
                v-model="favoritesOnly"
                class="library-favorites-filter"
                onIcon="pi pi-heart-fill"
                offIcon="pi pi-heart"
                onLabel=""
                offLabel=""
                :aria-label="favoritesOnly ? 'Show all' : 'Show favorites only'"
                :aria-pressed="favoritesOnly"
                v-tooltip.bottom="favoritesOnly ? 'Show all' : 'Show favorites only'"
            />
            <SelectButton
                v-if="viewOptions.length > 1"
                v-model="viewMode"
                :options="viewOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
            />
        </template>

        <template #secondary-actions>
            <!-- Layout toggle hidden on Songs tab (list-only per spec). -->
            <SelectButton
                v-if="viewMode !== 'songs'"
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

        <DiscoveryFeed v-if="viewMode === 'discover'" :layout="layout" />
        <AlbumListView
            v-else-if="viewMode === 'albums' && layout === 'list'"
            :folderId="folderId"
            :favoritesOnly="favoritesOnly"
        />
        <AlbumGrid
            v-else-if="viewMode === 'albums'"
            :folderId="folderId"
            :favoritesOnly="favoritesOnly"
        />
        <SongListView
            v-else-if="viewMode === 'songs'"
            :folderId="folderId"
            :favoritesOnly="favoritesOnly"
        />
        <ArtistListView
            v-else-if="layout === 'list'"
            :folderId="folderId"
            :favoritesOnly="favoritesOnly"
        />
        <ArtistGrid v-else :folderId="folderId" :favoritesOnly="favoritesOnly" />
    </ContentScaffold>
</template>

<style scoped>
/* The filter is a state toggle, not a destructive action: it reads as one of the
   header's controls, in the same grey as the unfilled hearts elsewhere, and only
   the FILL says it is on — the app-wide favorites rule (see
   docs/architecture/unified-play-experience.md). PrimeVue's checked ToggleButton
   would otherwise come up in the primary accent, which is reserved for what is
   playing and what is actionable. */
.library-favorites-filter :deep(.p-togglebutton-content) {
    color: var(--app-text-secondary);
}

.library-favorites-filter.p-togglebutton-checked :deep(.p-togglebutton-content) {
    color: var(--app-text-primary);
}

/* An empty on/offLabel still renders a &nbsp; span, which would pad the button
   wider than the icon-only SelectButtons beside it. Removing it (plus the label
   gap and the default min-width) is what keeps the four header controls on one
   line on the root route, which offers three tabs rather than two. */
.library-favorites-filter :deep(.p-togglebutton-label) {
    display: none;
}

.library-favorites-filter {
    min-width: 0;
}

.library-favorites-filter :deep(.p-togglebutton-content) {
    gap: 0;
}
</style>
