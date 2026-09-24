<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import SelectButton from 'primevue/selectbutton'
import ToggleButton from 'primevue/togglebutton'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import AlbumListView from '@/components/library/AlbumListView.vue'
import AlbumGrid from '@/components/library/AlbumGrid.vue'
import ArtistListView from '@/components/library/ArtistListView.vue'
import ArtistGrid from '@/components/library/ArtistGrid.vue'
import DiscoveryFeed from '@/components/library/DiscoveryFeed.vue'
import { useCatalogView, useMusicFolders, useReleaseTypes } from '@/composables/useSubsonicQueries'
import { useAlbumIndex } from '@/composables/useAlbumIndex'
import { useArtistTable } from '@/composables/useArtistTable'
import { useStarredAlbums, useStarredArtists } from '@/composables/useStarred'
import { useDiscoveryFeed } from '@/composables/useDiscovery'
import { useViewport } from '@/composables/useViewport'
import { useUiStore } from '@/store/uiStore'
import { PRIMARY_RELEASE_TYPES } from '@/lib/releaseTypes'
import { ALL_LIBRARY_VIEWS, LIBRARY_VIEWS, openingView } from '@/lib/libraryViews'
import type { LibraryView as ViewMode } from '@/types/libraries'

type Layout = 'grid' | 'list'

const route = useRoute()
const router = useRouter()
const uiStore = useUiStore()
const { shell } = useViewport()

const layoutOptions = [
    { label: 'List', value: 'list', icon: 'ms-view-list' },
    { label: 'Grid', value: 'grid', icon: 'ms-grid-view' }
]

const folderId = computed<number | undefined>(() => {
    const raw = route.params.folderId
    const value = Array.isArray(raw) ? raw[0] : raw
    if (!value) return undefined
    const num = Number(value)
    return Number.isFinite(num) ? num : undefined
})

const { data: folders } = useMusicFolders()
const { data: catalog } = useCatalogView()
const folder = computed(() => folders.value?.find((f) => f.id === folderId.value))
const folderName = computed(() =>
    folderId.value === undefined ? 'Library' : (folder.value?.name ?? 'Library')
)

// The views this page offers: every view at the root, which is the whole
// catalog; the configured ones inside a library. Empty while the library is
// unknown — its folders still loading, or an id that names no library.
const offeredViews = computed<readonly ViewMode[]>(() =>
    folderId.value === undefined ? ALL_LIBRARY_VIEWS : (folder.value?.views ?? [])
)

// Where the bare path lands: Discover at the root, a library's configured
// default inside one. Releases when the library offers nothing known, so an id
// that names no library shows the empty release list it always has.
const landingView = computed<ViewMode>(() => {
    if (folderId.value === undefined) return 'discover'
    return openingView(offeredViews.value, folder.value?.defaultView) ?? 'releases'
})

// The view named by the path segment (/library/releases, /library/5/artists).
// The router constrains the segment, so anything else reads as absent.
const modeParam = computed<ViewMode | null>(() => {
    const raw = route.params.mode
    const m = Array.isArray(raw) ? raw[0] : raw
    return m === 'discover' || m === 'releases' || m === 'artists' ? m : null
})

// The view shown: the path's when this page offers it, else the landing view.
const viewMode = computed<ViewMode>(() => {
    const wanted = modeParam.value
    return wanted && offeredViews.value.includes(wanted) ? wanted : landingView.value
})

// A library's views are unknown until its folder arrives, and rendering a guess
// would fetch a list only to swap it out; nothing is fetched until then.
const viewKnown = computed(() => folderId.value === undefined || folders.value !== undefined)

// A view the library does not offer — a stale link, or a view since turned off —
// is rewritten to the library's bare path, so the URL names what is shown. Only
// once the library is known: before that, every view looks unoffered.
watch(
    [modeParam, folder],
    ([mode, known]) => {
        if (!mode || !known || offeredViews.value.includes(mode)) return
        void router.replace({
            name: 'library-folder',
            params: { folderId: String(known.id) },
            query: route.query
        })
    },
    { immediate: true }
)

// A view's canonical path — the shape the sidebar links use too: the landing
// view is the bare path, every other view a segment.
function viewPath(view: ViewMode): string {
    const base = folderId.value === undefined ? '/library' : `/library/${folderId.value}`
    return view === landingView.value ? base : `${base}/${view}`
}

// The view switcher, wherever the sidebar is not already switching views: on
// the phone shell, which has no sidebar, and on the desktop wherever the root or
// the library has a single sidebar entry. A split one (an entry per view)
// leaves it to the sidebar — the root splits unless its descriptor says not to.
const viewOptions = computed(() => {
    const sidebarSwitches =
        folderId.value === undefined
            ? (catalog.value?.splitViews ?? true)
            : !!folder.value?.splitViews
    const show =
        offeredViews.value.length > 1 && (shell.value === 'mobile' || !sidebarSwitches)
    if (!show) return []
    return LIBRARY_VIEWS.filter((v) => offeredViews.value.includes(v.value))
})

// Choosing a view navigates to its canonical path: the view lives in the URL.
const selectedView = computed<ViewMode>({
    get: () => viewMode.value,
    set: (v) => void router.push(viewPath(v))
})

// Per-type layout: each view mode has its own default and can be overridden
// independently. Session-scoped via uiStore (survives navigation but not reload).
const layout = computed<Layout>({
    get: () => uiStore.getLibraryViewMode(viewMode.value),
    set: (v) => uiStore.setLibraryViewMode(viewMode.value, v)
})

// Favorites filter, in the URL so it survives a reload and is linkable. It applies
// to the Releases and Artists views only: Discover is a ranked feed in which
// favorites are already a scoring term, not a filterable list.
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

// Each source below is fetched only while its view is the one shown.
const showing = (view: ViewMode) => computed(() => viewKnown.value && viewMode.value === view)
const showingReleases = showing('releases')
const showingArtists = showing('artists')
const showingDiscover = showing('discover')

// A type no album in this library carries would only ever list nothing, so its
// tab is not offered. Until the carried types arrive, or if they cannot be
// fetched, every type is offered as before. The active type is always kept, so a
// linked or stale ?releaseType= still shows which filter is on and can be left.
// Types compare case-insensitively, as the server's filter matches them.
const { data: carriedReleaseTypes } = useReleaseTypes(folderId, { enabled: showingReleases })
const releaseTypeOptions = computed(() => {
    const carried = carriedReleaseTypes.value
        ? new Set(carriedReleaseTypes.value.map((t) => t.name.toLowerCase()))
        : null
    const active = releaseType.value.toLowerCase()
    const offered = PRIMARY_RELEASE_TYPES.filter((t) => {
        const key = t.toLowerCase()
        return !carried || carried.has(key) || key === active
    })
    return [
        { label: 'All', value: '' },
        ...offered.map((t) => ({ label: RELEASE_TYPE_LABELS[t], value: t }))
    ]
})

// Header counts — only the shown view's ACTIVE SOURCE is fetched, so the count
// never costs a request the body isn't already making (the pairs share a query
// cache entry with the grid/list below).
const { total: albumTotal } = useAlbumIndex(
    folderId,
    { enabled: computed(() => showingReleases.value && !favoritesOnly.value) },
    releaseTypeParam
)
const { total: artistTotal } = useArtistTable(folderId, {
    enabled: computed(() => showingArtists.value && !favoritesOnly.value)
})
const { total: starredAlbumTotal } = useStarredAlbums(folderId, {
    enabled: computed(() => showingReleases.value && favoritesOnly.value)
})
const { total: starredArtistTotal } = useStarredArtists(folderId, {
    enabled: computed(() => showingArtists.value && favoritesOnly.value)
})
// Shares its query cache entry with the DiscoveryFeed in the body (same folder),
// so reading the count here costs no extra request.
const { items: discoveryItems } = useDiscoveryFeed(folderId, { enabled: showingDiscover })
const summary = computed(() => {
    if (viewMode.value === 'discover') {
        const n = discoveryItems.value.length
        return n > 0 ? `${n} item${n === 1 ? '' : 's'}` : ''
    }
    // Filtered, the count is of favorites, and a bare "6 releases" would read as the
    // whole library. "6 favorites" rather than "6 favorite releases" because the
    // view already names the type, and a phone header has no room for the longer
    // form.
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
            <!-- The view switcher comes first: it decides what the filters
                 after it narrow. Same button-group look as the filters. -->
            <SelectButton
                v-if="viewOptions.length"
                v-model="selectedView"
                :options="viewOptions"
                optionLabel="label"
                optionValue="value"
                :allowEmpty="false"
                dataKey="value"
                class="as-button-group library-view-switch"
                aria-label="Library views"
            >
                <template #option="slotProps">
                    <i :class="slotProps.option.icon" aria-hidden="true"></i>
                    <span class="view-label">{{ slotProps.option.label }}</span>
                </template>
            </SelectButton>
            <!-- These two narrow WHAT is listed, so they read left-to-right as
                 one filter group: release type first, then favorites. A lone
                 "All" (no typed release in this library) narrows nothing, so
                 the release-type row is dropped rather than shown with one
                 tab. -->
            <SelectButton
                v-if="viewKnown && viewMode === 'releases' && !favoritesOnly && releaseTypeOptions.length > 1"
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
                v-if="viewKnown && viewMode !== 'discover'"
                v-model="favoritesOnly"
                class="library-favorites-filter as-button-group"
                onIcon="ms-favorite"
                offIcon="mso-favorite"
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

        <template v-if="viewKnown">
            <DiscoveryFeed v-if="viewMode === 'discover'" :layout="layout" :folderId="folderId" />
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
        </template>
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

/* On a phone the view switcher plus the release-type row are wider than the
   screen: the switcher drops to icons (the labels stay for screen readers) and
   the controls may wrap onto a second line instead of running off the edge.
   767.98px = $bp-phone-max - 0.02px, the same literal ContentScaffold uses. */
@media (max-width: 767.98px) {
    :deep(.scaffold-actions) {
        flex-wrap: wrap;
        flex-shrink: 1;
        min-width: 0;
    }

    .library-view-switch .view-label {
        position: absolute;
        width: 1px;
        height: 1px;
        overflow: hidden;
        clip-path: inset(50%);
        white-space: nowrap;
    }
}
</style>
