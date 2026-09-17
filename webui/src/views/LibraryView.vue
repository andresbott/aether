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
import { useMusicFolders } from '@/composables/useSubsonicQueries'
import { useAlbumIndex } from '@/composables/useAlbumIndex'
import { useArtistTable } from '@/composables/useArtistTable'
import { useStarredAlbums, useStarredArtists } from '@/composables/useStarred'
import { useDiscoveryFeed } from '@/composables/useDiscovery'
import { useUiStore } from '@/store/uiStore'
import { PRIMARY_RELEASE_TYPES } from '@/lib/releaseTypes'

type ViewMode = 'discover' | 'releases' | 'artists'
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

// On the root route, Discover is the default; a folder keeps its configured
// default_view, which the server only ever reports as albums/artists — mapped
// to this view's own releases/artists mode vocabulary.
const serverDefault = computed<ViewMode>(() => {
    if (discoverTabVisible.value) return 'discover'
    return folder.value?.defaultView === 'artists' ? 'artists' : 'releases'
})

// The browse mode is a path segment (/library/releases, /library/5/artists). The
// router constrains it to the library-scoped modes, so an absent or unknown value
// yields null and falls back to the server default below.
const modeParam = computed<ViewMode | null>(() => {
    const raw = route.params.mode
    const m = Array.isArray(raw) ? raw[0] : raw
    return m === 'releases' || m === 'artists' ? m : null
})

// Switched from the sidebar, so this is read-only: the mode lives in the path and
// changes by navigation, not by an in-view control.
const viewMode = computed<ViewMode>(() => {
    const wanted = modeParam.value ?? serverDefault.value
    // A mode the current route cannot offer (artists on a library with
    // showArtists=false) falls back to releases rather than rendering a tab with
    // no toggle to leave it. Discover is unreachable within a folder by
    // construction — the mode segment excludes it — so it needs no guard here.
    if (wanted === 'artists' && !artistsTabVisible.value) return 'releases'
    return wanted
})

// Per-type layout: each view mode has its own default and can be overridden
// independently. Session-scoped via uiStore (survives navigation but not reload).
const layout = computed<Layout>({
    get: () => uiStore.getLibraryViewMode(viewMode.value),
    set: (v) => uiStore.setLibraryViewMode(viewMode.value, v)
})

// Favorites filter, in the URL so it survives a reload and is linkable. It applies
// to the Releases and Artists tabs only: Discover is a ranked feed in which favorites
// are already a scoring term, not a filterable list.
const favoritesOnly = computed<boolean>({
    get: () => route.query.favorites === '1' && viewMode.value !== 'discover',
    set: (v) => {
        const query = { ...route.query }
        if (v) query.favorites = '1'
        else delete query.favorites
        router.replace({ query })
    }
})

const RELEASE_TYPE_LABELS: Record<string, string> = {
    Album: 'Albums',
    Single: 'Singles',
    EP: 'EPs',
    Broadcast: 'Broadcast',
    Other: 'Other'
}
const releaseTypeOptions = [
    { label: 'All', value: '' },
    ...PRIMARY_RELEASE_TYPES.map((t) => ({ label: RELEASE_TYPE_LABELS[t], value: t }))
]

// In the URL so it is linkable and reload-safe. '' means All. Only meaningful in
// Releases mode; forced to '' elsewhere and while favorites is on.
const releaseType = computed<string>({
    get: () => {
        if (viewMode.value !== 'releases' || favoritesOnly.value) return ''
        const q = route.query.releaseType
        return typeof q === 'string' ? q : ''
    },
    set: (v) => {
        const query = { ...route.query }
        if (v) query.releaseType = v
        else delete query.releaseType
        router.replace({ query })
    }
})
// '' → undefined so the "no filter" cache key matches unfiltered fetches.
const releaseTypeParam = computed<string | undefined>(() => releaseType.value || undefined)

// Header counts — only the active tab's ACTIVE SOURCE is fetched, so the count
// never costs a request the body isn't already making (the pairs share a query
// cache entry with the grid/list below).
const { total: albumTotal } = useAlbumIndex(
    folderId,
    { enabled: computed(() => viewMode.value === 'releases' && !favoritesOnly.value) },
    releaseTypeParam
)
const { total: artistTotal } = useArtistTable(folderId, {
    enabled: computed(() => viewMode.value === 'artists' && !favoritesOnly.value)
})
const { total: starredAlbumTotal } = useStarredAlbums(folderId, {
    enabled: computed(() => viewMode.value === 'releases' && favoritesOnly.value)
})
const { total: starredArtistTotal } = useStarredArtists(folderId, {
    enabled: computed(() => viewMode.value === 'artists' && favoritesOnly.value)
})
// Shares its query cache entry with the DiscoveryFeed in the body, so reading the
// count here costs no extra request.
const { items: discoveryItems } = useDiscoveryFeed()
const summary = computed(() => {
    if (viewMode.value === 'discover') {
        const n = discoveryItems.value.length
        return n > 0 ? `${n} item${n === 1 ? '' : 's'}` : ''
    }
    // Filtered, the count is of favorites, and a bare "6 releases" would read as the
    // whole library. "6 favorites" rather than "6 favorite releases" because the
    // active tab already names the type, and the root route's header — three tabs
    // plus two toggles — has no room for the longer form.
    if (favoritesOnly.value) {
        const n = viewMode.value === 'releases' ? starredAlbumTotal.value : starredArtistTotal.value
        return n > 0 ? `${n} favorite${n === 1 ? '' : 's'}` : ''
    }
    if (viewMode.value === 'releases') {
        return albumTotal.value > 0
            ? `${albumTotal.value} ${albumTotal.value === 1 ? 'release' : 'releases'}`
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
            <!-- Both controls here narrow WHAT is listed (the mode itself is
                 switched from the sidebar), so they read left-to-right as one
                 filter group: release type first, then favorites. -->
            <SelectButton
                v-if="viewMode === 'releases' && !favoritesOnly"
                v-model="releaseType"
                :options="releaseTypeOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
                class="as-button-group"
                aria-label="Release type"
            />
            <!-- Favorites filter. Hidden on Discover (ranked feed). Same heart
                 pair and wording as every other favorite affordance. -->
            <ToggleButton
                v-if="viewMode !== 'discover'"
                v-model="favoritesOnly"
                class="library-favorites-filter as-button-group"
                onIcon="pi pi-heart-fill"
                offIcon="pi pi-heart"
                onLabel=""
                offLabel=""
                :aria-label="favoritesOnly ? 'Show all' : 'Show favorites only'"
                :aria-pressed="favoritesOnly"
                v-tooltip.bottom="favoritesOnly ? 'Show all' : 'Show favorites only'"
            />
        </template>

        <template #secondary-actions>
            <!-- Layout toggle (grid/list), available in every mode. -->
            <SelectButton
                v-model="layout"
                :options="layoutOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
                dataKey="value"
                aria-label="Layout"
                class="as-button-group"
            >
                <template #option="slotProps">
                    <i :class="slotProps.option.icon"></i>
                </template>
            </SelectButton>
        </template>

        <DiscoveryFeed v-if="viewMode === 'discover'" :layout="layout" />
        <AlbumListView
            v-else-if="viewMode === 'releases' && layout === 'list'"
            :folderId="folderId"
            :favoritesOnly="favoritesOnly"
            :releaseType="releaseTypeParam"
        />
        <AlbumGrid
            v-else-if="viewMode === 'releases'"
            :folderId="folderId"
            :favoritesOnly="favoritesOnly"
            :releaseType="releaseTypeParam"
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
/* The favorites filter shares the header's button-group look (bordered,
   transparent, accent when active) via the global .as-button-group treatment in
   _main.scss. The app-wide "a favorite is grey, signalled by the FILL" rule (see
   docs/architecture/unified-play-experience.md) is deliberately relaxed HERE so
   the header controls read as one cluster; the per-track hearts keep grey.
   Locally the filter only needs to be an icon-only square that matches the add
   button: hide the empty on/offLabel's &nbsp; span (otherwise it pads the button
   wider), drop the content padding, and pin it to the control size. */
.library-favorites-filter {
    width: 2.125rem;
    height: 2.125rem;
    min-width: 0;
}

.library-favorites-filter :deep(.p-togglebutton-content) {
    padding: 0;
    gap: 0;
}

.library-favorites-filter :deep(.p-togglebutton-label) {
    display: none;
}
</style>
